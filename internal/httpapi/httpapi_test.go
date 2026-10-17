package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

func TestHealth(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cases := []struct {
		name       string
		db         fakeDB
		wantStatus string
		wantDB     string
		wantReady  int
	}{
		{"db up", fakeDB{}, "ok", "ok", http.StatusOK},
		{"db down", fakeDB{err: errors.New("down")}, "degraded", "down", http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := New(tc.db, "test", log)

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("health code = %d", rec.Code)
			}
			var got healthResponse
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if got.Status != tc.wantStatus || got.DB != tc.wantDB || got.Version != "test" {
				t.Fatalf("health = %+v", got)
			}

			rec = httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if rec.Code != tc.wantReady {
				t.Fatalf("readyz code = %d, want %d", rec.Code, tc.wantReady)
			}
		})
	}
}

func TestLivezAndRouting(t *testing.T) {
	h := New(fakeDB{err: errors.New("down")}, "test", slog.New(slog.NewTextHandler(io.Discard, nil)))

	cases := []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/livez", http.StatusOK}, // alive even when the DB is down
		{http.MethodPost, "/api/v1/health", http.StatusMethodNotAllowed},
		{http.MethodGet, "/nope", http.StatusNotFound},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != tc.want {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.path, rec.Code, tc.want)
		}
	}
}

type panicDB struct{}

func (panicDB) Ping(context.Context) error { panic("boom") }

func TestRecoverer(t *testing.T) {
	h := New(panicDB{}, "test", slog.New(slog.NewTextHandler(io.Discard, nil)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d, want 500", rec.Code)
	}
}

func TestOpenAPI(t *testing.T) {
	h := New(fakeDB{}, "test", slog.New(slog.NewTextHandler(io.Discard, nil)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/openapi.yaml", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "/api/v1/parse/batch") {
		t.Fatalf("status %d, body %.80q", rec.Code, rec.Body.String())
	}
}
