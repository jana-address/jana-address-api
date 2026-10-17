package parser

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/jana-address/jana-address-api/internal/synth"
)

// benchInput returns the raw addresses of the generated 10k set (seed 1); BENCH_INPUT=file.json
// (jury format) benchmarks another set.
func benchInput(b *testing.B) []string {
	b.Helper()
	path := os.Getenv("BENCH_INPUT")
	if path == "" {
		gold := synth.Generate(10000, 1)
		raws := make([]string, len(gold))
		for i, g := range gold {
			raws[i] = g.Raw
		}
		return raws
	}
	data, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	var items []struct {
		Raw string `json:"raw_address"`
	}
	if err := json.Unmarshal(data, &items); err != nil {
		b.Fatal(err)
	}
	raws := make([]string, len(items))
	for i, it := range items {
		raws[i] = it.Raw
	}
	return raws
}

// BenchmarkParseSynth parses the whole set once per iteration on one core.
func BenchmarkParseSynth(b *testing.B) {
	raws := benchInput(b)
	b.ReportAllocs()
	for b.Loop() {
		for _, r := range raws {
			Parse(r)
		}
	}
}
