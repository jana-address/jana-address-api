package parser

import (
	"testing"

	"github.com/jana-address/jana-address-api/internal/kato"
	"github.com/jana-address/jana-address-api/internal/parser/dict"
)

// TestPopularTailAnywhere: a ЖК or microdistrict name no map knows, then a street the maps of all cities
// know by many votes in total, in a line without a city or in a city whose map lacks the street.
func TestPopularTailAnywhere(t *testing.T) {
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
		{"мкр. дарабоз Есенжанова дом 3, кв 5", map[Field]string{LivingArea: "дарабоз", StreetName: "Есенжанова", House: "3"}},
		{"Астана мкр дарабоз Есенжанова 4", map[Field]string{City: "Астана", LivingArea: "дарабоз", StreetName: "Есенжанова", House: "4"}},
		// two-word ЖК names stay whole
		{"Астана ЖК Северное сияние 7", map[Field]string{LivingArea: "", StreetName: "Северное сияние", House: "7"}},
		{"ЖК Алтын Булак 2", map[Field]string{StreetName: "Алтын Булак", House: "2"}},
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
