package eval_test

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/jana-address/jana-address-api/internal/eval"
	"github.com/jana-address/jana-address-api/internal/kato"
	"github.com/jana-address/jana-address-api/internal/parser"
	"github.com/jana-address/jana-address-api/internal/synth"
)

// TestMain scores the parser the way the server runs it: with the embedded KATO snapshot.
// EVAL_NO_KATO=1 measures the hand tables alone.
func TestMain(m *testing.M) {
	if os.Getenv("EVAL_NO_KATO") == "" {
		recs, _, err := kato.Embedded()
		if err != nil {
			panic(err)
		}
		parser.UseKATO(recs)
	}
	os.Exit(m.Run())
}

func predict(raw string) map[string]string {
	out := map[string]string{}
	for f, v := range parser.Parse(raw).Values {
		out[string(f)] = v
	}
	return out
}

// TestFastTypeSet scores the parser on fast-typed addresses and guards against regressions.
// fasttype_100 and holdout_100 are labeled by hand; synth_10000 is generated in place by cmd/synth's
// generator with seed 1, so it is not stored in the repository.
// EVAL_EXTRA=set1,set2 scores more sets from testdata (without .jsonl).
// EVAL_FAILURES=dir writes every wrong field to dir/<set>.txt for error analysis.
func TestFastTypeSet(t *testing.T) {
	const dir = "../../testdata/"
	sets := []string{"fasttype/fasttype_100", "fasttype/holdout_100", "synth/synth_10000"}
	if extra := os.Getenv("EVAL_EXTRA"); extra != "" { // e.g. synth/synth_100000 made by cmd/synth, not committed
		sets = append(sets, strings.Split(extra, ",")...)
	}
	for _, set := range sets {
		t.Run(set, func(t *testing.T) {
			var gold []eval.Gold
			var err error
			if set == "synth/synth_10000" {
				gold = synth.Generate(10000, 1)
			} else {
				gold, err = eval.Load(dir + set + ".jsonl")
			}
			if os.IsNotExist(err) {
				t.Skip("no gold file")
			}
			if err != nil {
				t.Fatal(err)
			}
			rep := eval.Score(gold, predict)
			t.Log("\n" + rep.String())

			if out := os.Getenv("EVAL_FAILURES"); out != "" {
				var b strings.Builder
				for _, f := range rep.Failures {
					fmt.Fprintf(&b, "%s\t%-11s\twant=%q\tgot=%q\t| %s\n", f.ID, f.Field, f.Want, f.Got, f.Raw)
				}
				if err := os.WriteFile(out+"/"+strings.ReplaceAll(set, "/", "_")+".txt", []byte(b.String()), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			if raw, err := os.ReadFile(dir + set + ".baseline"); err == nil {
				base, _ := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
				if rep.Score < base-0.005 {
					t.Fatalf("score %.3f dropped below baseline %.3f", rep.Score, base)
				}
			}
		})
	}
}

func TestNorm(t *testing.T) {
	for in, want := range map[string]string{"Аксай-3а": "аксай 3а", "  Ул .Курильская ": "ул курильская", "1/2": "1 2", "": ""} {
		if got := eval.Norm(in); got != want {
			t.Errorf("Norm(%q) = %q, want %q", in, got, want)
		}
	}
}
