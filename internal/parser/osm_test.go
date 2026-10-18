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
	for _, city := range []string{"Алматы", "Астана", "Шымкент", "Караганда", "Павлодар"} {
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
		// not in the map, between 237 and 247 of the same side
		{"Алматы, улица Розыбакиева, 241", map[Field]string{District: "Бостандыкский"}, LevelNeighbors},
		// 33 is Алмалинский, two of three buildings of 37 too (37а is across the border)
		{"алматы розыбакиева 35", map[Field]string{District: "Алмалинский"}, LevelNeighbors},
		{"Алматы, ул. Тимирязева 42", map[Field]string{District: "Бостандыкский"}, ""},
		{"Астана, пр. Мангилик Ел 55", map[Field]string{District: "Есиль"}, ""},
		{"город Алматы/Алматы қаласы, Мкр Аксай 3а, 59, кв 70", map[Field]string{District: "Ауэзовский"}, ""},
		{"Алматы, мкр Самал-2, 33", map[Field]string{District: "Медеуский"}, ""},
		// written in the line: kept as written
		{"Алматы, Медеуский район, ул. Тимирязева 42", map[Field]string{District: "Медеуский"}, ""},
		// 140/1 and 140/2 are in the map
		{"Алматы, ул. Жандосова 140", map[Field]string{District: "Ауэзовский", StreetType: "улица"}, LevelNeighbors},
		// a street across districts and neighbours in two districts (17 and 23): no guess
		{"Алматы, ул. Жандосова 19", map[Field]string{District: "-"}, ""},
		// no city, no district
		{"ул. Тимирязева 42", map[Field]string{District: "-"}, ""},
		// a microdistrict that runs into an untyped street the city knows
		{"astana мкр жагалау кабанбай батыра 37, 295 кв 4 кабат", map[Field]string{LivingArea: "жагалау", StreetName: "кабанбай батыра", House: "37", StreetType: "проспект"}, ""}, // the type from the directory
		{"г.Алматы Алматы мкрн Баганашыл Муканова д 43 кв. 115, 18эт", map[Field]string{LivingArea: "Баганашыл", StreetName: "Муканова", House: "43"}, ""},
		// map names are noisy: "Алтын" is an area and "Булак" a street, the ЖК stays whole
		{"алматы жк алтын булак 31 квартира 93", map[Field]string{StreetName: "алтын булак", LivingArea: "-"}, ""},
		{"алматы жк. асыл тау 92а 59 кв", map[Field]string{StreetName: "асыл тау", LivingArea: "-"}, ""},
		{"Шымкент каласы микрорайон Асар Республики д.52 этаж 8 кв 2", map[Field]string{LivingArea: "Асар", StreetName: "Республики", House: "52"}, ""},
		// a short street key the map knows well, a long one it knows at all
		{"Павлодар мкр Усолка Абая 15", map[Field]string{LivingArea: "Усолка", StreetName: "Абая", House: "15"}, ""},
		{"Тараз мкр. Карасу Абая 101", map[Field]string{LivingArea: "Карасу", StreetName: "Абая", House: "101"}, ""},
		{"Талдыкорган мкр Мушелтой Тауелсиздик 12", map[Field]string{LivingArea: "Мушелтой", StreetName: "Тауелсиздик", House: "12"}, ""},
		// known whole, or a number in the name: not split
		{"Алматы, мкр Самал-2, д. 33, кв. 7", map[Field]string{StreetName: "Самал-2", LivingArea: "-"}, ""},
		{"город Алматы/Алматы қаласы, Мкр Аксай 3а, 59, кв 70", map[Field]string{StreetName: "Аксай 3а", LivingArea: "-"}, ""},
		// a city taken whole: its streets and living areas split the name, but give no district
		{"Павлодар мкр Достык Академика Маргулана 47", map[Field]string{LivingArea: "Достык", StreetName: "Академика Маргулана", House: "47", District: "-"}, ""},
		{"г. Кокшетау, мкрн Центральный Абылай хана, 32", map[Field]string{LivingArea: "Центральный", StreetName: "Абылай хана", District: "-"}, ""},
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
