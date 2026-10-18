package parser

import (
	"testing"

	"github.com/jana-address/jana-address-api/internal/kato"
	"github.com/jana-address/jana-address-api/internal/parser/dict"
)

// TestPopularTail: a ЖК or microdistrict name the map does not know, then a street the city's map knows by
// many votes, as written, one letter off or as the masculine adjective, right before the house.
func TestPopularTail(t *testing.T) {
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
		{"Астана ЖК Нурлы Турн 12", map[Field]string{City: "Астана", LivingArea: "Нурлы", StreetName: "Турн", House: "12"}},
		{"Шымкент жк Дарын сайрамский 5", map[Field]string{City: "Шымкент", LivingArea: "Дарын", StreetName: "сайрамский", House: "5"}},
		{"Петропавловск мкр Жанар Поьеды д 7", map[Field]string{City: "Петропавловск", LivingArea: "Жанар", StreetName: "Поьеды", House: "7"}},
		// two-word ЖК names stay whole
		{"Астана ЖК Северное сияние 7", map[Field]string{LivingArea: "", StreetName: "Северное сияние", House: "7"}},
		{"Алматы ЖК Алтын Булак 2", map[Field]string{LivingArea: "", StreetName: "Алтын Булак", House: "2"}},
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
