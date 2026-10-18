package parser

import (
	"testing"

	"github.com/jana-address/jana-address-api/internal/kato"
	"github.com/jana-address/jana-address-api/internal/parser/dict"
)

// TestNameBeforeTypeWord: a microdistrict name one letter off a type word stays a name before "ықшам ауданы",
// and "кала" after a village name is part of a name once the city was already given.
func TestNameBeforeTypeWord(t *testing.T) {
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
		{"Талдықорған қаласы Қвратал ықшам ауданы 14 үй", map[Field]string{City: "Талдыкорган", StreetName: "Қвратал", StreetType: "микрорайон", House: "14"}},
		{"г. Туркестан Жанап кала 5, кв 2", map[Field]string{City: "Туркестан", StreetName: "Жанап кала", House: "5", Apartment: "2"}},
		// a real unit word stays a unit
		{"Талдыкорган мкр Каратал 12 кварира 3", map[Field]string{LivingArea: "", StreetName: "Каратал", House: "12", Apartment: "3"}},
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
