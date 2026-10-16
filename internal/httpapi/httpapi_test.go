package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
