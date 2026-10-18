package parser

import (
	"testing"

	"github.com/jana-address/jana-address-api/internal/kato"
	"github.com/jana-address/jana-address-api/internal/parser/dict"
)

// TestGluedLast: a mistyped city glued to an area of that city that reads as an adjective.
func TestGluedLast(t *testing.T) {
	recs, _, err := kato.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	old := dict.Current()
	defer dict.SetCurrent(old)
	UseKATO(recs)
	cases := []struct {
		raw  string
		want map[Field]string
	}{
		{"астанаботанический 12", map[Field]string{City: "Астана", StreetName: "ботанический", House: "12"}},
		{"Астагаботанический 147 кв.19", map[Field]string{City: "Астана", StreetName: "ботанический", House: "147", Apartment: "19"}},
		{"Алматинская 5", map[Field]string{StreetName: "Алматинская", House: "5"}},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			o := Parse(tc.raw)
			for f, want := range tc.want {
				if got := o.Values[f]; got != want {
					t.Errorf("%s = %q, want %q (all: %v)", f, got, want, o.Values)
				}
			}
		})
	}
}
