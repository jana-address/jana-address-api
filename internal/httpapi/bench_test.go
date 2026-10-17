package httpapi

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jana-address/jana-address-api/internal/synth"
)

// BenchmarkBatch10k runs the whole batch handler on the generated 10k set (seed 1), byte for byte the file
// `server synth` writes for the UI; BENCH_INPUT=file.json benchmarks another file.
func BenchmarkBatch10k(b *testing.B) {
	var body []byte
	if path := os.Getenv("BENCH_INPUT"); path != "" {
		var err error
		if body, err = os.ReadFile(path); err != nil {
			b.Fatal(err)
		}
	} else {
		var buf bytes.Buffer
		if err := synth.WriteJury(&buf, synth.Generate(10000, 1)); err != nil {
			b.Fatal(err)
		}
		body = buf.Bytes()
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
