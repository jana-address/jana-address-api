package parser

import "strings"

// houseAt reports whether a house marker or the bound house number starts at i: "д 10", "д72", "41үй".
func (st *state) houseAt(i int) bool {
	if i < 0 || i >= len(st.toks) {
		return false
	}
	t := st.toks[i]
	if t.kind == tWord && t.mk != nil && t.mk.class == mHouse {
		return true
	}
	h, ok := st.vals[House]
	return ok && t.kind == tNum && t.used && compactNum(t.text) == h
}

// livingPart takes a separate part of plain words between the city and the street as the living area:
// "Караганда, Пришахтинск, ул. Ленина 5", "Астана, Нурсая, Сыганак 10". Without a city such a part may be
// the town itself, so it is left alone.
func (st *state) livingPart() {
	if _, ok := st.vals[LivingArea]; ok || st.vals[City] == "" || st.vals[StreetName] == "" || st.streetE <= 0 {
		return
	}
	streetSeg := st.toks[st.streetE-1].seg
	cityEnd := -1
	for _, sp := range st.spans {
		if sp.Field == City {
			cityEnd = max(cityEnd, sp.End)
		}
	}
	best := -1
	for seg := 0; seg < streetSeg; seg++ {
		a, b := st.segRange(seg)
		if a < 0 || b-a > 4 || st.c.start[st.toks[a].s] < cityEnd {
			continue
		}
		words := 0
		for i := a; i <= b; i++ {
			t := st.toks[i]
			switch {
			case t.kind == tWord && !t.used && t.mk == nil:
				words++
			case t.kind == tNum && !t.used && i > a+1 && st.isPunct(i-1, "-") && glued(st.toks[i-1], t): // "Самал-2"
			case st.isPunct(i, "-\"«»"):
			default:
				words = -100
			}
		}
		if words > 0 && words <= 3 {
			best = seg
		}
	}
	if best < 0 {
		return
	}
	a, b := st.segRange(best)
	for st.isPunct(a, "\"«»") {
		a++
	}
	for st.isPunct(b, "\"«»") {
		b--
	}
	st.set(LivingArea, st.text(a, b), SourceString, a, b)
	st.markUsed(a, b)
}

func (st *state) house() {
	if _, ok := st.vals[House]; !ok {
		from := st.streetE
		if from < 0 {
			from = 0
		}
		for k := from; k < len(st.toks); k++ {
			if st.availNum(k) {
				st.vals[House], st.src[House] = compactNum(st.toks[k].text), SourceString
				st.addSpan(House, k, k)
				st.toks[k].used = true
				break
			}
		}
	}
	// "дом 5, 12" — a single leftover number after the house is the apartment
	if _, ok := st.vals[Apartment]; ok {
		return
	}
	if _, ok := st.vals[House]; !ok {
		return
	}
	left := -1
	for k := range st.toks {
		if st.availNum(k) {
			if left >= 0 {
				return
			}
			left = k
		}
	}
	if left >= 0 && strings.IndexFunc(st.toks[left].text, isLetter) < 0 && !strings.Contains(st.toks[left].text, "/") {
		st.vals[Apartment], st.src[Apartment] = st.toks[left].text, SourceString
		st.addSpan(Apartment, left, left)
		st.toks[left].used = true
	}
}

// restore fills missing reference fields from the built-in directory.
func (st *state) restore() {
	if city, ok := st.vals[City]; ok {
		if p, ok := st.d.cities[key(city)]; ok {
			st.setDir(Province, p.province)
			st.setDir(District, p.district)
		} else if p, ok := st.settlement(key(city)); ok {
			st.setDir(Province, p.province)
			st.setDir(District, p.district)
		}
	}
	if _, ok := st.vals[District]; ok || st.vals[City] != "Алматы" {
		return
	}
	for _, f := range []Field{LivingArea, StreetName} {
		v, ok := st.vals[f]
		if !ok || (f == StreetName && st.vals[StreetType] != "микрорайон") {
			continue
		}
		w := strings.Fields(strings.NewReplacer("-", " ").Replace(fold(v)))
		if len(w) == 0 {
			continue
		}
		if d, ok := almatyAreas[w[0]]; ok {
			st.setDir(District, d)
			return
		}
	}
}
