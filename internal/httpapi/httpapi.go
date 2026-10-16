// Package httpapi wires the HTTP routes.
package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Pinger is the part of the DB pool the handlers need.
type Pinger interface {
	Ping(ctx context.Context) error
}

type api struct {
	db      Pinger
	version string
	log     *slog.Logger
}

// New returns the root handler.
func New(db Pinger, version string, log *slog.Logger) http.Handler {
	a := &api{db: db, version: version, log: log}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", a.livez)
	mux.HandleFunc("GET /readyz", a.readyz)
	mux.HandleFunc("GET /api/v1/health", a.health)

	return a.recoverer(mux)
}

// livez: the process is alive.
func (a *api) livez(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

// readyz: the API can serve requests, i.e. the database answers.
func (a *api) readyz(w http.ResponseWriter, r *http.Request) {
	if !a.dbOK(r.Context()) {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	DB      string `json:"db"`
}

func (a *api) health(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{Status: "ok", Version: a.version, DB: "ok"}
	if !a.dbOK(r.Context()) {
		resp.Status, resp.DB = "degraded", "down"
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *api) dbOK(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := a.db.Ping(ctx); err != nil {
		a.log.Warn("db ping failed", "err", err)
		return false
	}
	return true
}

func (a *api) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				a.log.Error("panic", "path", r.URL.Path, "panic", v)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}
