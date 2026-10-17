package parser

import (
	"strings"

	"github.com/jana-address/jana-address-api/internal/parser/dict"
	"github.com/jana-address/jana-address-api/internal/parser/text"
)

// kazakhPostfix markers always follow the name, unlike Russian "район" that can stand on either side.
var kazakhPostfix = map[string]bool{text.Fold("ауданы"): true, text.Fold("аудан"): true, text.Fold("қаласы"): true, text.Fold("облысы"): true}

// postfixAdmin markers usually follow the name: "Алматы қаласы", "Тұран ауданы", "Турксибский район".
var postfixAdmin = map[string]bool{}

func init() {
	for _, w := range []string{"қаласы", "қалас", "ауданы", "аудан", "облысы", "район", "р-н", "рн", "область", "обл", "обл-ть", "ауылы", "ауыл", "ауд", "қ", "қала"} {
		postfixAdmin[text.Fold(w)] = true
	}
}

func (st *state) keyOf(a, b int) string {
	var parts []string
	for i := a; i <= b; i++ {
		if st.toks[i].kind == tWord {
			parts = append(parts, st.toks[i].fold)
		}
	}
	return strings.Join(parts, " ")
}

// wordRun returns consecutive unmarked words next to the marker m in direction dir (+1/-1).
func (st *state) wordRun(m, dir int) (int, int) {
	j := m + dir
	for dir > 0 && st.isPunct(j, ".") {
		j++
	}
	a, b := -1, -1
	for k := j; k >= 0 && k < len(st.toks); k += dir {
		t := st.toks[k]
		if t.kind != tWord || t.used || t.mk != nil || t.seg != st.toks[m].seg {
			break
		}
		if dir > 0 && a >= 0 && st.postfixStreetAt(k+1) {
			break
		}
		if a < 0 {
			a, b = k, k
		}
		if dir > 0 {
			b = k
		} else {
			a = k
		}
	}
	return a, b
}

// adminName picks the name of an admin marker: the longest directory match next to it,
// otherwise a single word when more text follows ("г. Арысь Абая 10" -> "Арысь").
func (st *state) adminName(m int) (int, int) {
	class := st.toks[m].mk.Class
	fa, fb := st.wordRun(m, +1)
	ba, bb := st.wordRun(m, -1)
	gluedAfter := m > 0 && st.toks[m-1].kind == tWord && glued(st.toks[m-1], st.toks[m])
	backward := ba >= 0 && (postfixAdmin[st.toks[m].fold] || fa < 0 || gluedAfter)
	fwdKnown := func() bool {
		for b := fb; fa >= 0 && b >= fa; b-- {
			if st.d.AdminKnown(class, st.keyOf(fa, b)) {
				return true
			}
		}
		return false
	}
	if backward {
		for a := ba; a <= bb; a++ {
			k := st.keyOf(a, bb)
			if _, city := st.d.Cities[k]; city && class == dict.District && !kazakhPostfix[st.toks[m].fold] && fwdKnown() {
				break
			}
			if st.d.AdminKnown(class, k) {
				return a, bb
			}
		}
	}
	if fa >= 0 { // a known name after the marker wins over an unknown one before it: "Астана р-н Нура"
		for b := fb; b >= fa; b-- {
			if st.d.AdminKnown(class, st.keyOf(fa, b)) {
				return fa, b
			}
		}
	}
	if backward {
		if ba == 0 || st.toks[ba-1].kind == tPunct {
			return ba, bb
		}
		return bb, bb
	}
	if fa < 0 {
		return -1, -1
	}
	if fb > fa && st.availNum(fb+1) {
		return fa, fa
	}
	return fa, fb
}

func (st *state) admin() {
	// 1. explicit markers: "г. Алматы", "Алматы қаласы", "Тұран ауданы", "Карагандинская обл"
	for i := range st.toks {
		t := st.toks[i]
		if t.kind != tWord || t.used || !t.mk.IsAdmin() {
			continue
		}
		a, b := st.adminName(i)
		if a < 0 {
			continue
		}
		k, name := st.keyOf(a, b), st.text(a, b)
		switch t.mk.Class {
		case dict.Region:
			if r, ok := st.d.Regions[k]; ok {
				st.set(Province, r, SourceString, a, b)
			} else {
				st.set(Province, name, SourceString, a, b)
			}
		case dict.District:
			st.setDistrict(k, name, a, b)
		default:
			st.setCity(k, name, a, b, t.mk.Class == dict.Settlement)
		}
		st.markUsed(min(i, a), max(i, b))
	}

	maxSeg := 0
	for _, t := range st.toks {
		maxSeg = max(maxSeg, t.seg)
	}
	for seg := 0; seg <= maxSeg; seg++ {
		a, b := st.segRange(seg)
		if a < 0 {
			continue
		}
		// skip leading postcodes: "050000 Алматы ул Абая 10"
		for a <= b && (st.toks[a].used || (st.toks[a].kind == tNum && st.toks[a].ignore) || st.isPunct(a, ".")) {
			a++
		}
		if a > b {
			continue
		}
		clean := st.toks[a].kind == tWord // only unmarked words, no numbers
		for i := a; i <= b; i++ {
			t := st.toks[i]
			if t.used || t.kind == tNum || (t.kind == tWord && t.mk != nil) {
				clean = false
			}
		}
		k := st.keyOf(a, b)
		switch {
		// 2. a segment that is just a known name: "Алматы", "Турксибский"
		case clean && (st.d.Cities[k] != dict.Place{} || (seg == 0 && st.d.AdminKnown(dict.City, k) && st.typoCityBefore(b))): // close spelling only in the first segment
			st.setCity(k, st.text(a, b), a, b, false)
			st.markUsed(a, b)
		// 2b. a town of the classifier as a part of its own before more words: "Жаркент, ул. Абая 5".
		// Only towns: thousands of villages are called like streets ("Абай", "Достык").
		case clean && st.vals[City] == "" && st.townBefore(k, b):
			st.setCity(k, st.text(a, b), a, b, false)
			st.markUsed(a, b)
		case clean && st.d.Regions[k] != "" && (len([]rune(k)) == 3 || strings.HasSuffix(k, "ская")):
			st.set(Province, st.d.Regions[k], SourceString, a, b)
			st.markUsed(a, b)
		case clean && st.vals[City] == "" && len(st.d.Settlements[k]) > 0 &&
			(st.writtenValue(Province) != "" || st.writtenValue(District) != ""):
			st.setCity(k, st.text(a, b), a, b, true)
			st.markUsed(a, b)
		case clean && strings.HasSuffix(k, "ий") && st.d.AdminKnown(dict.District, k):
			st.setDistrict(k, st.text(a, b), a, b)
			st.markUsed(a, b)
		default:
			// 3. a known city inside a street segment: "Алматы Сатпаева 5", "050000 Алматы ул Абая 10"
			for i := a; i <= b; i++ {
				t := st.toks[i]
				if t.kind != tWord || t.used || t.mk != nil || (!st.d.AdminKnown(dict.City, t.fold) && i != 0) {
					continue
				}
				areaNext := i+2 <= b && st.toks[i+1].kind == tNum && st.toks[i+2].mk != nil && dict.NumberedAreaTypes[st.toks[i+2].mk.Canon]
				if _, exact := st.d.Cities[t.fold]; !exact && !areaNext && !st.typoCityBefore(i) {
					continue
				}
				if !st.d.AdminKnown(dict.City, t.fold) && (i != 0 || !st.townBefore(t.fold, i)) {
					continue // a town of the classifier only opens the line: "Жаркент ул. Абая 5"
				}
				p := i - 1
				for st.isPunct(p, ".") {
					p--
				}
				if p >= 0 && st.toks[p].mk != nil && st.toks[p].mk.Class == dict.Street {
					continue // "ул. Астана" is a street, not the city
				}
				if cur, set := st.vals[City]; set {
					// the city written twice: "алматы алматы жандосова 58"
					if pl, _ := st.d.LookupCity(t.fold); pl.Name == cur && i < b && st.toks[i+1].kind == tWord {
						st.toks[i].used = true
					}
					break
				}
				st.setCity(t.fold, t.text, i, i, false)
				st.toks[i].used = true
			}
		}
	}
}

// townBefore reports whether k, ending at token i, is a town of the classifier followed by a word, not by
// a house number, a street type or a house marker: "Жаркент, ул. Абая 5" is the town, "Абай, 12",
// "Абай даңғылы 10" and "Абай д 44" stay streets.
// A town named like a city district ("Алатау") is left to the district and living-area rules.
func (st *state) townBefore(k string, i int) bool {
	if _, ok := st.d.Town(k); !ok || i+1 >= len(st.toks) || !st.typoCityBefore(i) || st.postfixStreetAt(i+1) {
		return false
	}
	if d, ok := st.d.Districts[k]; ok && d.City != "" {
		return false
	}
	j := i + 1
	for st.isPunct(j, ",;()/") {
		j++
	}
	return !st.toks[j].mk.IsUnit()
}

// typoCityBefore reports whether a close spelling of a city may end at token i: only before another word
// or at the end ("алматыы абая 10", "алматыы, абая 10", "шымкнет квартира 120" are the city), never before
// a number ("Сатпаева 22" and "Сатпаева, 22" are a street one letter from the town Сатпаев).
func (st *state) typoCityBefore(i int) bool {
	j := i + 1
	for st.isPunct(j, ",;()/") {
		j++ // a comma between two parts
	}
	return j == len(st.toks) || st.toks[j].kind == tWord
}

// setCity stores a city or village. The hand table of large places goes first unless the line names
// another region or rayon; then the classifier's settlements, with the written rayon or region choosing
// among namesakes; a typo match comes last and, after "с.", "п.", "ауылы", only for names the
// classifier does not know ("с. Алмалы" must not become Алматы).
func (st *state) setCity(k, name string, a, b int, village bool) {
	use := func(p dict.Place) {
		st.set(City, p.Name, SourceString, a, b)
		st.setDir(Province, p.Province)
		st.setDir(District, p.District)
	}
	consistent := func(p dict.Place) bool {
		prov, dist := st.writtenValue(Province), st.writtenValue(District)
		return (prov == "" || p.Province == prov) && (dist == "" || p.District == "" || p.District == dist)
	}
	if hand, ok := st.d.Cities[k]; ok && consistent(hand) {
		// a written rayon picks its own village over a town of another region: "Мамлютский район, Щучье"
		// is a village there, not Щучинск; in the same region the hand table keeps its spelling
		if p, own := st.settlement(k); own && hand.District == "" && st.writtenValue(District) != "" && p.Province != hand.Province {
			use(p)
		} else {
			use(hand)
		}
		return
	}
	ps := st.d.Settlements[k]
	if p, ok := st.settlement(k); ok {
		use(p)
		return
	}
	if p, ok := st.d.Town(k); ok && !village && consistent(p) { // "г. Шу": the town, not the village namesake
		use(p)
		return
	}
	if len(ps) == 0 || !village {
		if p, ok := st.d.LookupCity(k); ok {
			use(p)
			return
		}
	}
	if len(ps) > 0 && sameName(ps) { // namesakes in several regions: the name is certain, the region is not
		st.set(City, ps[0].Name, SourceString, a, b)
		return
	}
	st.set(City, text.RuSpelling(name), SourceString, a, b)
}

func sameName(ps []dict.Place) bool {
	for _, p := range ps[1:] {
		if p.Name != ps[0].Name {
			return false
		}
	}
	return true
}

func (st *state) setDistrict(k, name string, a, b int) {
	d, ok := st.d.Districts[k]
	if !ok {
		st.set(District, name, SourceString, a, b)
		return
	}
	st.set(District, d.Name, SourceString, a, b)
	// "Абайский", "Есильский" are city districts of Шымкент and Астана and also rural rayons in other
	// regions: the city comes from the district only when the line names no other region
	if prov := st.writtenValue(Province); prov == "" || prov == d.Province {
		st.setDir(City, d.City)
		st.setDir(Province, d.Province)
	}
}

// settlement resolves a town or village name; province and district written in the line pick one
// of several namesakes. It returns false when the name is unknown or still ambiguous.
func (st *state) settlement(k string) (dict.Place, bool) {
	ps := st.d.Settlements[k]
	if len(ps) == 0 {
		return dict.Place{}, false
	}
	prov, dist := st.writtenValue(Province), st.writtenValue(District)
	var match []dict.Place
	for _, p := range ps {
		if (prov == "" || p.Province == prov) && (dist == "" || p.District == dist) {
			match = append(match, p)
		}
	}
	if len(match) == 1 {
		return match[0], true
	}
	return dict.Place{}, false
}

// writtenValue returns a field value only when it came from the line itself.
func (st *state) writtenValue(f Field) string {
	if st.src[f] != SourceString {
		return ""
	}
	return st.vals[f]
}
