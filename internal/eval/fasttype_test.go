package eval

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/jana-address/jana-address-api/internal/parser"
)

func predict(raw string) map[string]string {
	out := map[string]string{}
	for f, v := range parser.Parse(raw).Values {
		out[string(f)] = v
	}
	return out
}

// TestFastTypeSet scores the parser on fast-typed addresses and guards against regressions.
// EVAL_FAILURES=path writes every wrong field to a file for error analysis.
func TestFastTypeSet(t *testing.T) {
	const dir = "../../testdata/fasttype/"
	gold, err := Load(dir + "fasttype_100.jsonl")
	if os.IsNotExist(err) {
		t.Skip("no gold file")
	}
	if err != nil {
		t.Fatal(err)
	}
	rep := Score(gold, predict)
	t.Log("\n" + rep.String())

	if path := os.Getenv("EVAL_FAILURES"); path != "" {
		var b strings.Builder
		for _, f := range rep.Failures {
			fmt.Fprintf(&b, "%s\t%-11s\twant=%q\tgot=%q\t| %s\n", f.ID, f.Field, f.Want, f.Got, f.Raw)
		}
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if raw, err := os.ReadFile(dir + "baseline.txt"); err == nil {
		base, _ := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
		if rep.Score < base-0.005 {
			t.Fatalf("score %.3f dropped below baseline %.3f", rep.Score, base)
		}
	}
}

func TestNorm(t *testing.T) {
	for in, want := range map[string]string{"Аксай-3а": "аксай 3а", "  Ул .Курильская ": "ул курильская", "1/2": "1 2", "": ""} {
		if got := Norm(in); got != want {
			t.Errorf("Norm(%q) = %q, want %q", in, got, want)
		}
	}
}
