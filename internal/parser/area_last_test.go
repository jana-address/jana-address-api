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
		{"астана мкр. меруерт ботанический 17 дом", map[Field]string{LivingArea: "меруерт", StreetName: "ботанический", House: "17"}},
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

// TestPopularTailAnywhereFuzzy: after a microdistrict name, the street is one letter off a name the maps
// know by many votes; a ЖК keeps such a word.
func TestPopularTailAnywhereFuzzy(t *testing.T) {
	recs, _, err := kato.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	old := dict.Current()
	defer dict.SetCurrent(old)
	UseKATO(recs)
	for _, tc := range []struct{ raw, area, street string }{
		{"мкр.дарабоз есенжвнова д5", "дарабоз", "есенжвнова"},
		{"мкр дарабоз аузэова д13, 1 пәтер", "дарабоз", "аузэова"},
		{"мкр. кунаева Есенжанова дом 8, 22кв под 4", "кунаева", "Есенжанова"},
		{"мкр.ильинка аузэова д13, 1 пәтер", "ильинка", "аузэова"},
		{"Костанай ЖК Бауырластар Тлендиевва 14", "", "Бауырластар Тлендиевва"},
	} {
		o := Parse(tc.raw)
		if o.Values[LivingArea] != tc.area || o.Values[StreetName] != tc.street {
			t.Errorf("%s: %q | %q, want %q | %q", tc.raw, o.Values[LivingArea], o.Values[StreetName], tc.area, tc.street)
		}
	}
}
