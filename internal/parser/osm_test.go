package parser

import (
	"strings"
	"testing"

	"github.com/jana-address/jana-address-api/internal/parser/dict"
)

// TestOSMDistricts: the district of a city address comes from the OSM street directory when the line
// does not name it, and never replaces a written one.
func TestOSMDistricts(t *testing.T) {
	ref := dict.OSM()
	for _, city := range []string{"Алматы", "Астана", "Шымкент"} {
		if len(ref.Streets[city]) < 500 {
			t.Fatalf("%s: %d streets in the directory", city, len(ref.Streets[city]))
		}
	}
	cases := []struct {
		raw   string
		want  map[Field]string // "-" means must be absent
		level string           // how the district was matched
	}{
		{"Алматы, ул. Розыбакиева 247, БЦ Нурлы Тау", map[Field]string{District: "Бостандыкский"}, LevelHouse},
		{"Алматы, ул. Тимирязева 42", map[Field]string{District: "Бостандыкский"}, ""},
		{"Астана, пр. Мангилик Ел 55", map[Field]string{District: "Есиль"}, ""},
		{"город Алматы/Алматы қаласы, Мкр Аксай 3а, 59, кв 70", map[Field]string{District: "Ауэзовский"}, ""},
		{"Алматы, мкр Самал-2, 33", map[Field]string{District: "Медеуский"}, ""},
		// written in the line: kept as written
		{"Алматы, Медеуский район, ул. Тимирязева 42", map[Field]string{District: "Медеуский"}, ""},
		// a street across districts and a house the map does not know: no guess
		{"Алматы, ул. Жандосова 140", map[Field]string{District: "-", StreetType: "улица"}, ""},
		// no city, no district
		{"ул. Тимирязева 42", map[Field]string{District: "-"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			o := Parse(tc.raw)
			for f, want := range tc.want {
				got, ok := o.Values[f]
				if want == "-" {
					if ok {
						t.Errorf("%s = %q, want empty (all: %v)", f, got, o.Values)
					}
					continue
				}
				if got != want {
					t.Errorf("%s = %q, want %q (all: %v)", f, got, want, o.Values)
				}
			}
			if tc.level != "" && o.Levels[District] != tc.level {
				t.Errorf("district level = %q, want %q", o.Levels[District], tc.level)
			}
		})
	}

	// every street the directory places in one district gets it back through the parser, so the key
	// cmd/osmref builds is the one the parser looks up
	for _, city := range []string{"Алматы", "Астана", "Шымкент"} {
		total, right := 0, 0
		var miss []string
		for k, s := range ref.Streets[city] {
			key, typ, _ := strings.Cut(k, "\x00")
			if typ != dict.AnyType || s.Share < dict.MinStreetShare || len([]rune(key)) < 4 || strings.ContainsAny(key, "0123456789") {
				continue
			}
			total++
			o := Parse(city + ", " + key + " 5")
			if o.Values[District] == s.District {
				right++
			} else if len(miss) < 15 {
				miss = append(miss, key+" -> "+o.Values[District])
			}
		}
		if right*100 < total*90 {
			t.Errorf("%s: %d of %d streets get their district, e.g. %v", city, right, total, miss)
		}
		t.Logf("%s: %d of %d streets get their district", city, right, total)
	}
}
