package parser

import "strings"

// kazakhPostfix markers always follow the name, unlike Russian "район" that can stand on either side.
var kazakhPostfix = map[string]bool{fold("ауданы"): true, fold("аудан"): true, fold("қаласы"): true, fold("облысы"): true}

// postfixAdmin markers usually follow the name: "Алматы қаласы", "Тұран ауданы", "Турксибский район".
var postfixAdmin = map[string]bool{}

func init() {
	for _, w := range []string{"қаласы", "қалас", "ауданы", "аудан", "облысы", "район", "р-н", "рн", "область", "обл", "обл-ть", "ауылы", "ауыл", "ауд", "қ", "қала"} {
		postfixAdmin[fold(w)] = true
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

func (d *directory) adminKnown(class mclass, k string) bool {
	switch class {
	case mRegion:
		_, ok := d.regions[k]
		return ok
	case mDistrict:
		_, ok := d.districts[k]
		return ok
	}
	_, ok := d.lookupCity(k)
	return ok
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
	class := st.toks[m].mk.class
	fa, fb := st.wordRun(m, +1)
	ba, bb := st.wordRun(m, -1)
	gluedAfter := m > 0 && st.toks[m-1].kind == tWord && glued(st.toks[m-1], st.toks[m])
	backward := ba >= 0 && (postfixAdmin[st.toks[m].fold] || fa < 0 || gluedAfter)
	fwdKnown := func() bool {
		for b := fb; fa >= 0 && b >= fa; b-- {
			if st.d.adminKnown(class, st.keyOf(fa, b)) {
				return true
			}
		}
		return false
	}
	if backward {
		for a := ba; a <= bb; a++ {
			k := st.keyOf(a, bb)
			if _, city := st.d.cities[k]; city && class == mDistrict && !kazakhPostfix[st.toks[m].fold] && fwdKnown() {
				break
			}
			if st.d.adminKnown(class, k) {
				return a, bb
			}
		}
	}
	if fa >= 0 { // a known name after the marker wins over an unknown one before it: "Астана р-н Нура"
		for b := fb; b >= fa; b-- {
			if st.d.adminKnown(class, st.keyOf(fa, b)) {
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
		if t.kind != tWord || t.used || !t.mk.isAdmin() {
			continue
		}
		a, b := st.adminName(i)
		if a < 0 {
			continue
		}
		k, name := st.keyOf(a, b), st.text(a, b)
		switch t.mk.class {
		case mRegion:
			if r, ok := st.d.regions[k]; ok {
				st.set(Province, r, SourceString, a, b)
			} else {
				st.set(Province, name, SourceString, a, b)
			}
		case mDistrict:
			st.setDistrict(k, name, a, b)
		default:
			st.setCity(k, name, a, b, t.mk.class == mSettlement)
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
		case clean && (st.d.cities[k] != place{} || (seg == 0 && st.d.adminKnown(mCity, k) && st.typoCityBefore(b))): // close spelling only in the first segment
			st.setCity(k, st.text(a, b), a, b, false)
			st.markUsed(a, b)
		case clean && st.d.regions[k] != "" && (len([]rune(k)) == 3 || strings.HasSuffix(k, "ская")):
			st.set(Province, st.d.regions[k], SourceString, a, b)
			st.markUsed(a, b)
		case clean && st.vals[City] == "" && len(st.d.settlements[k]) > 0 &&
			(st.writtenValue(Province) != "" || st.writtenValue(District) != ""):
			st.setCity(k, st.text(a, b), a, b, true)
			st.markUsed(a, b)
		case clean && strings.HasSuffix(k, "ий") && st.d.adminKnown(mDistrict, k):
			st.setDistrict(k, st.text(a, b), a, b)
			st.markUsed(a, b)
		default:
			// 3. a known city inside a street segment: "Алматы Сатпаева 5", "050000 Алматы ул Абая 10"
			for i := a; i <= b; i++ {
				t := st.toks[i]
				if t.kind != tWord || t.used || t.mk != nil || !st.d.adminKnown(mCity, t.fold) {
					continue
				}
				areaNext := i+2 <= b && st.toks[i+1].kind == tNum && st.toks[i+2].mk != nil && numberedAreaTypes[st.toks[i+2].mk.canon]
				if _, exact := st.d.cities[t.fold]; !exact && !areaNext && !st.typoCityBefore(i) {
					continue
				}
				p := i - 1
				for st.isPunct(p, ".") {
					p--
				}
				if p >= 0 && st.toks[p].mk != nil && st.toks[p].mk.class == mStreet {
					continue // "ул. Астана" is a street, not the city
				}
				if cur, set := st.vals[City]; set {
					// the city written twice: "алматы алматы жандосова 58"
					if pl, _ := st.d.lookupCity(t.fold); pl.name == cur && i < b && st.toks[i+1].kind == tWord {
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
	use := func(p place) {
		st.set(City, p.name, SourceString, a, b)
		st.setDir(Province, p.province)
		st.setDir(District, p.district)
	}
	consistent := func(p place) bool {
		prov, dist := st.writtenValue(Province), st.writtenValue(District)
		return (prov == "" || p.province == prov) && (dist == "" || p.district == "" || p.district == dist)
	}
	if hand, ok := st.d.cities[k]; ok && consistent(hand) {
		// a written rayon picks its own village over a town of another region: "Мамлютский район, Щучье"
		// is a village there, not Щучинск; in the same region the hand table keeps its spelling
		if p, own := st.settlement(k); own && hand.district == "" && st.writtenValue(District) != "" && p.province != hand.province {
			use(p)
		} else {
			use(hand)
		}
		return
	}
	ps := st.d.settlements[k]
	if p, ok := st.settlement(k); ok {
		use(p)
		return
	}
	if len(ps) == 0 || !village {
		if p, ok := st.d.lookupCity(k); ok {
			use(p)
			return
		}
	}
	if len(ps) > 0 && sameName(ps) { // namesakes in several regions: the name is certain, the region is not
		st.set(City, ps[0].name, SourceString, a, b)
		return
	}
	st.set(City, ruSpelling(name), SourceString, a, b)
}

func sameName(ps []place) bool {
	for _, p := range ps[1:] {
		if p.name != ps[0].name {
			return false
		}
	}
	return true
}

func (st *state) setDistrict(k, name string, a, b int) {
	d, ok := st.d.districts[k]
	if !ok {
		st.set(District, name, SourceString, a, b)
		return
	}
	st.set(District, d.name, SourceString, a, b)
	// "Абайский", "Есильский" are city districts of Шымкент and Астана and also rural rayons in other
	// regions: the city comes from the district only when the line names no other region
	if prov := st.writtenValue(Province); prov == "" || prov == d.province {
		st.setDir(City, d.city)
		st.setDir(Province, d.province)
	}
}
