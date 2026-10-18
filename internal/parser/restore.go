package parser

import (
	"sort"
	"strings"
	"sync"

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
	if _, ok := st.vals[District]; !ok {
		st.districtFromPrecincts(city)
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
	if st.vals[StreetType] == "шоссе" {
		// the map keeps "тасжолы" in the name: "Темірлан тасжолы" is "темирлан тасжолы", not the street "темирлан"
		if _, ok := ref.Street(city, key+" "+tasZholy, dict.AnyType); ok {
			key = key + " " + tasZholy
		}
	}
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

// streetAlias finds the map name of a street written shorter or longer than the map has it, or with one
// letter wrong, when only one street fits: "габдуллина" is Малика Габдуллина in Астана, "Малика Габдуллина"
// is Габдуллина in Алматы, "мангилик" is Мангилик Ел, "алалыкена" is Алалыкина.
func streetAlias(ref *dict.OSMRef, city, key string) (string, bool) {
	first, byFirst := byFirstWord(ref, city, key)
	if bySurname(city, key) {
		if last := surnames()[city][key][0]; !byFirst || last == first {
			return last, true
		}
		return "", false // "жибек" ends Алтын Жибек and starts Жибек Жолы: no guess
	}
	if byFirst {
		return first, true
	}
	if name, ok := closeStreetName(city, key); ok {
		return name, true // "алалыкена" is Алалыкина: only one street of the city is one letter off
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

// byFirstWord finds the street written by the first word of its map name: "мангилик" is Мангилик Ел.
// Several map spellings ("мангилик ел", "мангилик ели") must all lie in one district; then the one with
// the most segments and buildings is taken.
func byFirstWord(ref *dict.OSMRef, city, key string) (string, bool) {
	if len([]rune(key)) < 5 || strings.Contains(key, " ") {
		return "", false
	}
	keys := firstWords()[city][key]
	best, district := "", ""
	for i, k := range keys {
		s, _ := ref.Street(city, k, dict.AnyType)
		if i > 0 && (s.District == "" || s.District != district) {
			return "", false
		}
		district = s.District
		if b, _ := ref.Street(city, best, dict.AnyType); best == "" || s.Votes > b.Votes || s.Votes == b.Votes && k < best {
			best = k
		}
	}
	return best, best != ""
}

// firstWords maps a city and the first word of its streets of two words or more, known well, to their keys.
var firstWords = sync.OnceValue(func() map[string]map[string][]string {
	out := map[string]map[string][]string{}
	for city, streets := range dict.OSM().Streets {
		m := map[string][]string{}
		for key, s := range streets {
			name, typ, _ := strings.Cut(key, "\x00")
			i := strings.IndexByte(name, ' ')
			if typ != dict.AnyType || s.Votes < 3 || i < 0 {
				continue
			}
			m[name[:i]] = append(m[name[:i]], name)
		}
		for _, ks := range m {
			sort.Strings(ks)
		}
		out[city] = m
	}
	return out
})

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

// districtFromPrecincts restores the district of a city whose district borders the map does not draw
// (Караганда, Актобе) from the akim's list of polling precincts, which names the streets of each district: the
// street first, then the microdistrict or living area.
func (st *state) districtFromPrecincts(city string) {
	p := dict.Precincts()[city]
	if p == nil {
		return
	}
	if name, ok := st.vals[StreetName]; ok {
		k, _ := dict.StreetKey(name)
		typ := ""
		if st.src[StreetType] == SourceString {
			typ = st.vals[StreetType]
		}
		if d, ok := p.District(k, typ); ok {
			st.setDir(District, d, LevelStreet)
			return
		}
		if alias, ok := streetAlias(dict.OSM(), city, k); ok { // a typo or a shorter name of a street the map knows
			if d, ok := p.District(alias, typ); ok {
				st.setDir(District, d, LevelStreet)
				return
			}
		}
	}
	if v, ok := st.vals[LivingArea]; ok {
		if k, _ := dict.StreetKey(v); k != "" {
			if d, ok := p.District(k, ""); ok {
				st.setDir(District, d, LevelArea)
			}
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
