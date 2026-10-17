// Command synth writes a synthetic labeled address set.
//
//	synth -n 10000 -seed 1 -format gold -o testdata/synth/synth_10000.jsonl       (the set the tests generate in place)
//	synth -n 10000 -seed 1 -format jury -o testdata/synth/synth_10000_input.json  (the same set to upload in the UI)
//
// The server image has the same generator: `server synth -n 10000 -seed 1 > synth_10000_input.json`.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

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
	recs := synth.Generate(n, seed)
	if format == "gold" {
		return synth.WriteGold(w, recs)
	}
	return synth.WriteJury(w, recs)
}
