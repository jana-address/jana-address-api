package parser

import (
	"testing"

	"github.com/jana-address/jana-address-api/internal/kato"
	"github.com/jana-address/jana-address-api/internal/parser/dict"
)

// TestGluedLast: a mistyped or short city glued to an area or a street.
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
		// a city written short, the letters after it starting a street the map knows
		{"астанаажол 8", map[Field]string{City: "Астана", StreetName: "ажол", House: "8"}},
		{"лисаковстроителей 5", map[Field]string{City: "Лисаковск", StreetName: "строителей", House: "5"}},
		{"лисаковстроителй үй73кв65", map[Field]string{City: "Лисаковск", StreetName: "строителй", House: "73", Apartment: "65"}},
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
