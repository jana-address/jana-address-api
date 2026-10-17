// Package katosync keeps the KATO classifier in PostgreSQL and in the parser.
//
// Start-up: the parser gets the snapshot embedded in the binary at once, so parsing never waits
// for the database or the network. In the background the syncer waits for PostgreSQL, applies
// the migrations, seeds an empty database with the snapshot and switches the parser to the active
// version stored there. With OFFLINE=false it also checks stat.gov.kz every interval and stores
// a new version when the published file changes.
package katosync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jana-address/jana-address-api/internal/db"
	"github.com/jana-address/jana-address-api/internal/kato"
	"github.com/jana-address/jana-address-api/internal/parser"
)

// DefaultPage is the classifier page on stat.gov.kz with the link to the current xlsx.
const DefaultPage = kato.DefaultPageURL

// Config controls the syncer.
type Config struct {
	Offline  bool          // never touch the network
	Interval time.Duration // how often to check stat.gov.kz when online
	PageURL  string
	Client   *http.Client
}

// Status is what the health endpoint reports about the classifier in use.
type Status struct {
	Origin     string    `json:"origin"` // embedded or stat.gov.kz
	SourceDate string    `json:"source_date"`
	SHA256     string    `json:"sha256"`
	Rows       int       `json:"rows"`
	Offline    bool      `json:"offline"`
	LastCheck  time.Time `json:"last_check,omitzero"`
	LastError  string    `json:"last_error,omitempty"`
}

// Syncer owns the classifier lifecycle.
type Syncer struct {
	pool *pgxpool.Pool
	log  *slog.Logger
	cfg  Config

	mu     sync.Mutex
	status Status
}

// New applies the embedded snapshot to the parser and returns a syncer for the background work.
func New(pool *pgxpool.Pool, log *slog.Logger, cfg Config) (*Syncer, error) {
	if cfg.PageURL == "" {
		cfg.PageURL = DefaultPage
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 7 * 24 * time.Hour
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 2 * time.Minute}
	}
	recs, m, err := kato.Embedded()
	if err != nil {
		return nil, fmt.Errorf("embedded KATO snapshot: %w", err)
	}
	stats := parser.UseKATO(recs)
	s := &Syncer{pool: pool, log: log, cfg: cfg}
	s.status = Status{Origin: "embedded", SourceDate: m.SourceDate, SHA256: m.SHA256, Rows: len(recs), Offline: cfg.Offline}
	log.Info("kato: embedded snapshot in use", "source_date", m.SourceDate, "rows", len(recs),
		"settlement_names", stats.Settlements, "ambiguous", stats.Ambiguous, "rayons_added", stats.Districts)
	return s, nil
}

// Status returns the classifier version the parser uses now.
func (s *Syncer) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Run waits for the database, migrates, seeds, loads the active version and, when online,
// refreshes from stat.gov.kz every interval. It returns when ctx is done.
func (s *Syncer) Run(ctx context.Context) {
	if err := s.prepare(ctx); err != nil {
		if ctx.Err() == nil {
			s.log.Error("kato: database setup failed, keeping the embedded snapshot", "err", err)
		}
		return
	}
	if s.cfg.Offline {
		s.log.Info("kato: OFFLINE mode, stat.gov.kz is not checked")
		return
	}
	t := time.NewTimer(time.Minute) // first check soon after start, then every interval
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.refresh(ctx)
			t.Reset(s.cfg.Interval)
		}
	}
}

// Migrate runs the migrations and seeds the snapshot; used by `server migrate`.
func Migrate(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) error {
	applied, err := db.Migrate(ctx, pool)
	if err != nil {
		return err
	}
	if len(applied) > 0 {
		log.Info("migrations applied", "versions", applied)
	}
	return seed(ctx, pool, log)
}

func (s *Syncer) prepare(ctx context.Context) error {
	for wait := time.Second; ; wait = min(wait*2, 30*time.Second) {
		err := s.pool.Ping(ctx)
		if err == nil {
			break
		}
		s.log.Info("kato: waiting for the database", "err", err, "retry_in", wait.String())
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	if err := Migrate(ctx, s.pool, s.log); err != nil {
		return err
	}
	return s.loadActive(ctx)
}

// seed stores the embedded snapshot as the first, active version of an empty database.
func seed(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) error {
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM kato_version`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	recs, m, err := kato.Embedded()
	if err != nil {
		return err
	}
	if err := store(ctx, pool, "embedded", m.SourceURL, m.SourceDate, m.SHA256, recs); err != nil {
		return err
	}
	logSync(ctx, pool, "seeded", m.SourceURL, m.SHA256, fmt.Sprintf("%d rows", len(recs)))
	log.Info("kato: database seeded from the embedded snapshot", "rows", len(recs), "source_date", m.SourceDate)
	return nil
}

// loadActive switches the parser to the active database version when it differs from the one in use.
func (s *Syncer) loadActive(ctx context.Context) error {
	var (
		id                     int64
		origin, src, sha, date string
	)
	err := s.pool.QueryRow(ctx, `SELECT id, origin, source_url, sha256, coalesce(source_date::text, '')
		FROM kato_version WHERE active`).Scan(&id, &origin, &src, &sha, &date)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if sha == s.Status().SHA256 {
		return nil
	}
	recs, err := load(ctx, s.pool, id)
	if err != nil {
		return err
	}
	parser.UseKATO(recs)
	s.setStatus(func(st *Status) { st.Origin, st.SourceDate, st.SHA256, st.Rows = origin, date, sha, len(recs) })
	s.log.Info("kato: active database version in use", "origin", origin, "source_date", date, "rows", len(recs))
	return nil
}

// refresh downloads the published file and stores it as the new active version if it changed.
func (s *Syncer) refresh(ctx context.Context) {
	data, src, err := kato.Fetch(ctx, s.cfg.Client, s.cfg.PageURL)
	if err == nil {
		err = s.update(ctx, data, src)
	}
	s.setStatus(func(st *Status) {
		st.LastCheck = time.Now().UTC()
		st.LastError = ""
		if err != nil {
			st.LastError = err.Error()
		}
	})
	if err != nil {
		s.log.Error("kato: refresh failed, keeping the current version", "err", err)
		logSync(ctx, s.pool, "failed", src, "", err.Error())
	}
}

func (s *Syncer) update(ctx context.Context, data []byte, src string) error {
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM kato_version WHERE sha256 = $1)`, sha).Scan(&exists); err != nil {
		return err
	}
	if exists {
		logSync(ctx, s.pool, "unchanged", src, sha, "")
		s.log.Info("kato: stat.gov.kz file unchanged", "sha256", sha)
		return nil
	}
	recs, err := kato.ParseXLSX(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("parse %s: %w", src, err)
	}
	if err := kato.Validate(recs, s.Status().Rows); err != nil {
		return fmt.Errorf("validate %s: %w", src, err)
	}
	date := kato.SourceDate(src)
	if err := store(ctx, s.pool, "stat.gov.kz", src, date, sha, recs); err != nil {
		return err
	}
	parser.UseKATO(recs)
	s.setStatus(func(st *Status) { st.Origin, st.SourceDate, st.SHA256, st.Rows = "stat.gov.kz", date, sha, len(recs) })
	logSync(ctx, s.pool, "updated", src, sha, fmt.Sprintf("%d rows", len(recs)))
	s.log.Info("kato: new version from stat.gov.kz in use", "source_date", date, "rows", len(recs))
	return nil
}

func (s *Syncer) setStatus(f func(*Status)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(&s.status)
}

// store inserts a version with its rows and makes it the only active one, in one transaction.
func store(ctx context.Context, pool *pgxpool.Pool, origin, src, date, sha string, recs []kato.Record) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE kato_version SET active = false WHERE active`); err != nil {
			return err
		}
		var id int64
		err := tx.QueryRow(ctx, `INSERT INTO kato_version (origin, source_url, source_date, sha256, rows, active)
			VALUES ($1, $2, nullif($3, '')::date, $4, $5, true) RETURNING id`,
			origin, src, date, sha, len(recs)).Scan(&id)
		if err != nil {
			return err
		}
		_, err = tx.CopyFrom(ctx, pgx.Identifier{"kato_object"},
			[]string{"version_id", "code", "parent", "kind", "name_ru", "name_kk", "name_ru_clean", "name_kk_clean"},
			pgx.CopyFromSlice(len(recs), func(i int) ([]any, error) {
				r := recs[i]
				var parent any
				if r.Parent != "" {
					parent = r.Parent
				}
				return []any{id, r.Code, parent, int16(r.Kind), r.NameRU, r.NameKK, r.NameRUClean, r.NameKKClean}, nil
			}))
		return err
	})
}

func load(ctx context.Context, pool *pgxpool.Pool, version int64) ([]kato.Record, error) {
	rows, err := pool.Query(ctx, `SELECT code, coalesce(parent, ''), kind, name_ru, name_kk, name_ru_clean, name_kk_clean
		FROM kato_object WHERE version_id = $1 ORDER BY code`, version)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (kato.Record, error) {
		var r kato.Record
		var kind int16
		err := row.Scan(&r.Code, &r.Parent, &kind, &r.NameRU, &r.NameKK, &r.NameRUClean, &r.NameKKClean)
		r.Kind = kato.Kind(kind)
		return r, err
	})
}

func logSync(ctx context.Context, pool *pgxpool.Pool, status, src, sha, msg string) {
	_, _ = pool.Exec(context.WithoutCancel(ctx), `INSERT INTO kato_sync_log (status, source_url, sha256, message)
		VALUES ($1, nullif($2, ''), nullif($3, ''), nullif($4, ''))`, status, src, sha, msg)
}
