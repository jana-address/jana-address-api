// Command synth writes a synthetic labeled address set.
//
//	synth -n 10000 -seed 1 -format gold -o testdata/synth/synth_10000.jsonl       (the set the tests generate in place)
//	synth -n 10000 -seed 1 -format jury -o testdata/synth/synth_10000_input.json  (the same set to upload in the UI)
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jana-address/jana-address-api/internal/eval"
	"github.com/jana-address/jana-address-api/internal/synth"
)

func main() {
	n := flag.Int("n", 10000, "number of records")
	seed := flag.Uint64("seed", 1, "random seed")
	out := flag.String("o", "", "output file (default stdout)")
	format := flag.String("format", "gold", "gold: JSONL with labels; jury: JSON array of id and raw_address")
	flag.Parse()
	if err := run(*n, *seed, *out, *format); err != nil {
		fmt.Fprintln(os.Stderr, "synth:", err)
		os.Exit(1)
	}
}

func run(n int, seed uint64, out, format string) error {
	if format != "gold" && format != "jury" {
		return fmt.Errorf("unknown format %q (want gold or jury)", format)
	}
	var w io.Writer = os.Stdout
	if out != "" {
		f, err := os.Create(out)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	bw := bufio.NewWriter(w)
	recs := synth.Generate(n, seed)
	if format == "gold" {
		writeGold(bw, recs)
	} else {
		writeJury(bw, recs)
	}
	return bw.Flush()
}

// enc marshals a value without HTML escaping.
func enc(v any) []byte {
	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	e.SetEscapeHTML(false)
	_ = e.Encode(v) // strings, slices and *string cannot fail
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}

// writeGold writes one record per line, expected keys in field order.
func writeGold(w *bufio.Writer, recs []eval.Gold) {
	for _, g := range recs {
		w.WriteString(`{"id":`)
		w.Write(enc(g.ID))
		w.WriteString(`,"raw_address":`)
		w.Write(enc(g.Raw))
		w.WriteString(`,"expected":{`)
		for i, f := range synth.Fields {
			if i > 0 {
				w.WriteByte(',')
			}
			w.Write(enc(f))
			w.WriteByte(':')
			w.Write(enc(g.Expected[f]))
		}
		w.WriteString(`},"in_string":`)
		w.Write(enc(g.InString))
		w.WriteString(`,"tags":`)
		w.Write(enc(g.Tags))
		w.WriteString("}\n")
	}
}

// writeJury writes the hackathon input format: [{"id", "raw_address"}].
func writeJury(w *bufio.Writer, recs []eval.Gold) {
	w.WriteString("[\n")
	for i, g := range recs {
		w.WriteString(`  {"id": `)
		w.Write(enc(g.ID))
		w.WriteString(`, "raw_address": `)
		w.Write(enc(g.Raw))
		w.WriteString("}")
		if i < len(recs)-1 {
			w.WriteByte(',')
		}
		w.WriteByte('\n')
	}
	w.WriteString("]\n")
}
