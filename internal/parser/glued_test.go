package parser

import (
	"testing"

	"github.com/jana-address/jana-address-api/internal/kato"
	"github.com/jana-address/jana-address-api/internal/parser/dict"
)

// TestGluedBorders: the borders of words typed without spaces between a city, its type word and the street.
func TestGluedBorders(t *testing.T) {
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
		// "қала" glued after a mistyped city is the city's type word, not a part of the street
		{"павлдарқалалермонтова 15", map[Field]string{City: "Павлодар", StreetName: "лермонтова", House: "15"}},
		{"кокшетуқала абылайхана 3", map[Field]string{City: "Кокшетау", StreetName: "абылайхана", House: "3"}},
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
