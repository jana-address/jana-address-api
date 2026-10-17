package synth

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"

	"github.com/jana-address/jana-address-api/internal/eval"
)

// enc marshals a value without HTML escaping.
func enc(v any) []byte {
	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	e.SetEscapeHTML(false)
	_ = e.Encode(v) // strings, slices and *string cannot fail
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}

// WriteGold writes one labeled record per line, expected keys in field order.
func WriteGold(w io.Writer, recs []eval.Gold) error {
	bw := bufio.NewWriter(w)
	for _, g := range recs {
		bw.WriteString(`{"id":`)
		bw.Write(enc(g.ID))
		bw.WriteString(`,"raw_address":`)
		bw.Write(enc(g.Raw))
		bw.WriteString(`,"expected":{`)
		for i, f := range Fields {
			if i > 0 {
				bw.WriteByte(',')
			}
			bw.Write(enc(f))
			bw.WriteByte(':')
			bw.Write(enc(g.Expected[f]))
		}
		bw.WriteString(`},"in_string":`)
		bw.Write(enc(g.InString))
		bw.WriteString(`,"tags":`)
		bw.Write(enc(g.Tags))
		bw.WriteString("}\n")
	}
	return bw.Flush()
}

// WriteJury writes the hackathon input format: [{"id", "raw_address"}], one record per line.
func WriteJury(w io.Writer, recs []eval.Gold) error {
	bw := bufio.NewWriter(w)
	bw.WriteString("[\n")
	for i, g := range recs {
		bw.WriteString(`  {"id": `)
		bw.Write(enc(g.ID))
		bw.WriteString(`, "raw_address": `)
		bw.Write(enc(g.Raw))
		bw.WriteString("}")
		if i < len(recs)-1 {
			bw.WriteByte(',')
		}
		bw.WriteByte('\n')
	}
	bw.WriteString("]\n")
	return bw.Flush()
}
