package parser

import (
	"strings"

	"github.com/jana-address/jana-address-api/internal/parser/dict"
	"github.com/jana-address/jana-address-api/internal/parser/text"
)

// restore fills missing reference fields from the built-in directory. A value the line gives is
// never replaced; each restored value records how it was matched (Level*).
func (st *state) restore() {
	if city, ok := st.vals[City]; ok {
		if p, ok := st.d.Cities[text.Key(city)]; ok {
			st.setDir(Province, p.Province, LevelSettlement)
			st.setDir(District, p.District, LevelSettlement)
		} else if p, ok := st.settlement(text.Key(city)); ok {
			st.setDir(Province, p.Province, LevelSettlement)
			st.setDir(District, p.District, LevelSettlement)
		}
	}
	city := st.vals[City]
	ref := dict.OSM()
	if _, ok := st.vals[District]; !ok {
		st.districtFromStreets(ref, city)
	}
	if _, ok := st.vals[District]; !ok && city == "Алматы" {
		st.districtFromAlmatyAreas()
	}
	if _, ok := st.vals[StreetType]; !ok {
		if name, ok := st.vals[StreetName]; ok {
			k, _ := dict.StreetKey(name)
			if _, ok := ref.Street(city, k, dict.AnyType); !ok {
				if alias, ok := streetAlias(ref, city, k); ok {
					k = alias
				}
			}
			if s, ok := ref.Street(city, k, dict.AnyType); ok && s.Type != "" {
				st.setDir(StreetType, s.Type, LevelStreet) // every street of that name has this type
			}
		}
	}
}

// osmStreet finds the written street in the directory. The written type picks among streets of one
// name ("пр. Абая" is not "ул. Абая"), unless that street is small next to the others of the name:
// "ул. Абая 150" in Алматы is most likely the avenue, so then all streets of the name are taken.
func (st *state) osmStreet(ref *dict.OSMRef, city string) (key, typ string, s dict.StreetRef, ok bool) {
	name, ok := st.vals[StreetName]
	if !ok {
		return "", "", s, false
	}
	key, _ = dict.StreetKey(name)
	all, okAll := ref.Street(city, key, dict.AnyType)
	if !okAll {
		if alias, ok := streetAlias(ref, city, key); ok {
			key = alias
			all, okAll = ref.Street(city, key, dict.AnyType)
		}
	}
	if st.src[StreetType] == SourceString {
		if t, okT := ref.Street(city, key, st.vals[StreetType]); okT && (!okAll || t.Votes*2 >= all.Votes) {
			return key, st.vals[StreetType], t, true
		}
	}
	return key, dict.AnyType, all, okAll
}

// streetAlias finds the map name of a street written shorter or longer than the map has it, when only one
// street fits: "габдуллина" is Малика Габдуллина in Астана, "Малика Габдуллина" is Габдуллина in Алматы.
func streetAlias(ref *dict.OSMRef, city, key string) (string, bool) {
	if bySurname(city, key) {
		return surnames()[city][key][0], true
	}
	i := strings.LastIndexByte(key, ' ')
	if i < 0 {
		return "", false
	}
	last := key[i+1:]
	if len([]rune(last)) < 5 || len(surnames()[city][last]) > 0 {
		return "", false // "Кабанбай батыра" is not a street "батыра"
	}
	if _, ok := ref.Street(city, last, dict.AnyType); ok {
		return last, true
	}
	return "", false
}

// districtFromStreets restores the district of a city address from the OSM directory: the building
// when the street crosses a district border, else its neighbours on the same side of the street, else
// the street when most of it lies in one district, else the microdistrict or living area.
func (st *state) districtFromStreets(ref *dict.OSMRef, city string) {
	if ref.Streets[city] == nil {
		return
	}
	if k, typ, s, ok := st.osmStreet(ref, city); ok {
		if h, ok := st.vals[House]; ok {
			if d, ok := ref.House(city, k, typ, h); ok {
				st.setDir(District, d, LevelHouse)
				return
			}
			if d, ok := ref.Neighbors(city, k, typ, h); ok { // "Розыбакиева 241" between 237 and 247
				st.setDir(District, d, LevelNeighbors)
				return
			}
		}
		if s.Share >= dict.MinStreetShare {
			st.setDir(District, s.District, LevelStreet)
			return
		}
	}
	for _, f := range []Field{LivingArea, StreetName} {
		v, ok := st.vals[f]
		if !ok || (f == StreetName && !areaType[st.vals[StreetType]]) {
			continue
		}
		k, _ := dict.StreetKey(v)
		if a, ok := ref.Areas[city][k]; ok && a.Share >= dict.MinStreetShare {
			st.setDir(District, a.District, LevelArea)
			return
		}
	}
}

// areaType: street types that name a living area rather than a street
var areaType = map[string]bool{"микрорайон": true, "квартал": true, "жилой комплекс": true}

// districtFromAlmatyAreas is the hand table of Almaty microdistricts, for the ones the map does not name.
func (st *state) districtFromAlmatyAreas() {
	for _, f := range []Field{LivingArea, StreetName} {
		v, ok := st.vals[f]
		if !ok || (f == StreetName && st.vals[StreetType] != "микрорайон") {
			continue
		}
		w := strings.Fields(strings.NewReplacer("-", " ").Replace(text.Fold(v)))
		if len(w) == 0 {
			continue
		}
		if d, ok := dict.AlmatyAreas[w[0]]; ok {
			st.setDir(District, d, LevelArea)
			return
		}
	}
}
