package httpapi

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// BenchmarkBatch10k runs the whole batch handler on a jury-format file; BENCH_INPUT overrides the default set.
func BenchmarkBatch10k(b *testing.B) {
	path := os.Getenv("BENCH_INPUT")
	if path == "" {
		path = "../../testdata/synth/synth_10000_input.json"
	}
	body, err := os.ReadFile(path)
	if err != nil {
		b.Skip(err)
	}
	h := New(nil, "bench", slog.New(slog.NewTextHandler(io.Discard, nil)))
	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/parse/batch", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
	}
}
