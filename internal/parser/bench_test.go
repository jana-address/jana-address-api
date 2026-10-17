package parser

import (
	"encoding/json"
	"os"
	"testing"
)

// benchInput reads the raw addresses of a jury-format file; BENCH_INPUT overrides the default set.
func benchInput(b *testing.B) []string {
	b.Helper()
	path := os.Getenv("BENCH_INPUT")
	if path == "" {
		path = "../../testdata/synth/synth_10000_input.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		b.Skip(err)
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
