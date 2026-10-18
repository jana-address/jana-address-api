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
		// a stray letter between a city and its type word stays with the city
		{"кокшетауфқала абая 3", map[Field]string{City: "Кокшетау", StreetName: "абая", House: "3"}},
		{"павлодарркаласыкутузова 12", map[Field]string{City: "Павлодар", StreetName: "кутузова", House: "12"}},
		// the longest type word goes, a short street stays; "қаласатпаева" is қала and Сатпаева
		{"павлодарулицаабй 5", map[Field]string{City: "Павлодар", StreetType: "улица", StreetName: "абй", House: "5"}},
		{"алматықаласатпаева 10", map[Field]string{City: "Алматы", StreetName: "сатпаева", House: "10"}},
		// a street of the city that starts with a type word keeps it, also mistyped
		{"астанаулыдалла 3", map[Field]string{City: "Астана", StreetName: "улыдалла", House: "3"}},
		{"астанақаласыкошкарбайева 9", map[Field]string{City: "Астана", StreetName: "кошкарбайева", House: "9"}},
		// a city in capitals keeps its last letter before a lowercase street
		{"ПАВЛОДАРкутузова 4", map[Field]string{City: "Павлодар", StreetName: "кутузова", House: "4"}},
		{"СЕМЕЙАбая 2", map[Field]string{City: "Семей", StreetName: "Абая", House: "2"}},
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
