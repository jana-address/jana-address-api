package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jana-address/jana-address-api/internal/synth"
)

// BenchmarkBatch10k runs the whole batch handler on the generated 10k set (seed 1) in the jury format;
// BENCH_INPUT=file.json benchmarks another file.
func BenchmarkBatch10k(b *testing.B) {
	var body []byte
	var err error
	if path := os.Getenv("BENCH_INPUT"); path != "" {
		body, err = os.ReadFile(path)
	} else {
		type item struct {
			ID  string `json:"id"`
			Raw string `json:"raw_address"`
		}
		gold := synth.Generate(10000, 1)
		items := make([]item, len(gold))
		for i, g := range gold {
			items[i] = item{g.ID, g.Raw}
		}
		body, err = json.Marshal(items)
	}
	if err != nil {
		b.Fatal(err)
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
