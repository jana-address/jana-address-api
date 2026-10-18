package parser

import (
	"testing"

	"github.com/jana-address/jana-address-api/internal/kato"
	"github.com/jana-address/jana-address-api/internal/parser/dict"
)

// TestAreaThenStreetUnseparated: a microdistrict and a street written without a separator, when the city's
// map lacks both but some city of the map knows the area.
func TestAreaThenStreetUnseparated(t *testing.T) {
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
		// an area some city knows, then a street no city knows, right before the house
		{"Костанай мкр Бауырластар Тлендиевва 14", map[Field]string{City: "Костанай", LivingArea: "Бауырластар", StreetName: "Тлендиевва", House: "14"}},
		{"г. Костанай, мкр. акшат жумабайулы 7 кв 3", map[Field]string{LivingArea: "акшат", StreetName: "жумабайулы", House: "7", Apartment: "3"}},
		// a ЖК keeps its words, and so does a name with no house right after it
		{"Костанай ЖК Бауырластар Тлендиевва 14", map[Field]string{City: "Костанай", LivingArea: "", StreetName: "Бауырластар Тлендиевва", House: "14"}},
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
