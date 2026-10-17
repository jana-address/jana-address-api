package kato

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"
)

// Snapshot file names inside the snapshot directory.
const (
	SnapshotFile = "kato.csv.gz"
	ManifestFile = "manifest.json"
)

var csvHeader = []string{"code", "parent", "kind", "name_ru", "name_kk", "name_ru_clean", "name_kk_clean"}

// Manifest describes where a snapshot came from.
type Manifest struct {
	SourceURL   string    `json:"source_url"`
	SourceDate  string    `json:"source_date"` // YYYY-MM-DD from the file name
	SHA256      string    `json:"sha256"`      // of the original xlsx
	Rows        int       `json:"rows"`
	GeneratedAt time.Time `json:"generated_at"`
}

// NewManifest describes a snapshot built from xlsx downloaded from src.
func NewManifest(src string, xlsx []byte, rows int, now time.Time) Manifest {
	sum := sha256.Sum256(xlsx)
	return Manifest{
		SourceURL:   DisplayURL(src),
		SourceDate:  SourceDate(src),
		SHA256:      hex.EncodeToString(sum[:]),
		Rows:        rows,
		GeneratedAt: now.UTC().Truncate(time.Second),
	}
}

// WriteSnapshot writes recs as gzip-compressed CSV sorted by code. The
// output is byte-for-byte reproducible for the same input.
func WriteSnapshot(w io.Writer, recs []Record) error {
	recs = slices.Clone(recs)
	slices.SortStableFunc(recs, byCode)
	zw, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return err
	}
	cw := csv.NewWriter(zw)
	if err := cw.Write(csvHeader); err != nil {
		return err
	}
	for _, r := range recs {
		kind, err := r.Kind.MarshalText()
		if err != nil {
			return fmt.Errorf("kato: %s: %w", r.Code, err)
		}
		row := []string{r.Code, r.Parent, string(kind), r.NameRU, r.NameKK, r.NameRUClean, r.NameKKClean}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}
	return zw.Close()
}

// ReadSnapshot reads the format written by WriteSnapshot.
func ReadSnapshot(r io.Reader) ([]Record, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("kato: snapshot: %w", err)
	}
	defer zr.Close()
	cr := csv.NewReader(zr)
	cr.FieldsPerRecord = len(csvHeader)
	cr.ReuseRecord = true
	head, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("kato: snapshot header: %w", err)
	}
	if !slices.Equal(head, csvHeader) {
		return nil, fmt.Errorf("kato: snapshot header %q, want %q", head, csvHeader)
	}
	var recs []Record
	for {
		row, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return recs, nil
		}
		if err != nil {
			return nil, fmt.Errorf("kato: snapshot: %w", err)
		}
		kind, err := ParseKind(row[2])
		if err != nil {
			return nil, fmt.Errorf("kato: snapshot code %s: %w", row[0], err)
		}
		recs = append(recs, Record{
			Code:        row[0],
			Parent:      row[1],
			Kind:        kind,
			NameRU:      row[3],
			NameKK:      row[4],
			NameRUClean: row[5],
			NameKKClean: row[6],
		})
	}
}
