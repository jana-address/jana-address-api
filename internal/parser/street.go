package parser

import "strings"

type cand struct {
	typ      string
	mk       int // marker token index
	a, b     int // name token range
	numbered bool
	isLiving bool
}

func (st *state) laterNumberExists(i int) bool {
	if _, ok := st.vals[House]; ok {
		return true
	}
	for j := i + 1; j < len(st.toks); j++ {
		if st.availNum(j) {
			return true
		}
	}
	return false
}

// collectForward reads a street name after a type marker: "ул. Курильская", "Мкр Аксай 3а".
func (st *state) collectForward(m int, typ string) (int, int) {
	j := m + 1 + st.toks[m].tail
	for st.isPunct(j, ".\"") {
		j++
	}
	a, b := -1, -1
	for k := j; k < len(st.toks); k++ {
		t := st.toks[k]
		if t.used || t.seg != st.toks[m].seg && t.seg >= 0 {
			break
		}
		switch t.kind {
		case tWord:
			if t.mk != nil && typ == "жилой комплекс" && a >= 0 && t.mk.class == mStreet &&
				!(k+1 < len(st.toks) && st.toks[k+1].kind == tWord && st.toks[k+1].mk == nil && !st.toks[k+1].used) {
				break
			}
			if t.mk != nil {
				return a, b
			}
			if typ == "жилой комплекс" && a >= 0 && isLatin(st.toks[b].text) != isLatin(t.text) {
				return a, b
			}
			if !realStreetTypes[typ] && a >= 0 && st.postfixStreetAt(k+1) {
				return a, b
			}
		case tNum:
			if t.ignore {
				return a, b
			}
			if numberedAreaTypes[typ] {
				hyphened := a >= 0 && st.isPunct(k-1, "-") && glued(st.toks[k-1], t) // "Самал-2"
				if !(hyphened || st.laterNumberExists(k)) {
					return a, b
				}
				if a < 0 {
					a = k
				}
				return a, k
			}
			// "ул. 8 Марта": a number right after the type followed by a word
			if a < 0 && k+1 < len(st.toks) && st.toks[k+1].kind == tWord && st.toks[k+1].mk == nil && !st.toks[k+1].used &&
				!st.d.adminKnown(mCity, st.toks[k+1].fold) {
				break
			}
			return a, b
		case tPunct:
			if (t.text == "-" || t.text == "\"") && a >= 0 && k+1 < len(st.toks) && st.toks[k+1].kind != tPunct {
				continue
			}
			if t.text == "." && a >= 0 && len([]rune(st.toks[k-1].text)) == 1 {
				continue
			}
			return a, b
		}
		if a < 0 {
			a = k
		}
		b = k
	}
	return a, b
}

// collectBackward reads a name before a postfix type: "Рыбинский переулок", "Нурсити 2 микрорайон", "66квартал".
func (st *state) collectBackward(m int, typ string) (int, int) {
	a, b := -1, -1
	for k := m - 1; k >= 0; k-- {
		t := st.toks[k]
		if t.used || t.seg != st.toks[m].seg {
			break
		}
		ok := false
		switch t.kind {
		case tWord:
			ok = t.mk == nil
		case tNum:
			ok = numberedAreaTypes[typ] && !t.ignore
		case tPunct:
			ok = t.text == "-" && a >= 0 || strings.Contains("\"«»", t.text)
		}
		if !ok {
			break
		}
		if t.kind == tPunct && t.text != "-" {
			continue // a quote around the name
		}
		if b < 0 {
			b = k
		}
		a = k
	}
	return a, b
}

// postfixStreetAt reports whether a real street type word starts at i: "көшесі", "көш.", "ул.".
func (st *state) postfixStreetAt(i int) bool {
	if !(i < len(st.toks) && st.toks[i].kind == tWord && !st.toks[i].used && st.toks[i].mk != nil &&
		st.toks[i].mk.class == mStreet && realStreetTypes[st.toks[i].mk.canon]) {
		return false
	}
	// "ул.садовая" opens a new name, so it is a prefix, not a postfix
	j := i + 1 + st.toks[i].tail
	for st.isPunct(j, ".") {
		j++
	}
	return !(j < len(st.toks) && st.toks[j].kind == tWord && st.toks[j].mk == nil && !st.toks[j].used)
}

func (st *state) street() {
	var cands []cand
	covered := -1 // a type word inside a collected name is not a new street: "жк Экспо Бульвар"
	for i, t := range st.toks {
		if i <= covered || t.kind != tWord || t.used || t.mk == nil || (t.mk.class != mStreet && t.mk.class != mLiving) {
			continue
		}
		typ := t.mk.canon
		c := cand{typ: typ, mk: i, isLiving: t.mk.class == mLiving}
		c.a, c.b = st.collectForward(i, typ)
		if numberedAreaTypes[typ] && st.availNum(i-1) && c.a >= 0 && c.a == c.b && st.toks[c.a].kind == tNum {
			c.a = -1 // "6 шағын ауданы 10": 6 is the area, 10 the house
		}
		if c.a < 0 {
			c.a, c.b = st.collectBackward(i, typ)
		}
		if c.a < 0 {
			continue
		}
		covered = max(covered, c.b)
		cands = append(cands, c)
	}

	var street, living *cand
	for i := range cands {
		c := &cands[i]
		if realStreetTypes[c.typ] && street == nil {
			street = c
		}
	}
	for i := range cands {
		c := &cands[i]
		if c == street {
			continue
		}
		if street == nil && !c.isLiving {
			street = c
		} else if living == nil {
			living = c
		}
	}

	untyped := false
	if living != nil {
		st.markUsed(min(living.mk, living.a), max(living.mk, living.b))
		if street == nil {
			untyped = st.untypedStreet()
		}
		if street == nil && !untyped && living.isLiving {
			street = living // ЖК without a street is the street: "Астана жк Северное сияние дом 7"
		} else {
			st.set(LivingArea, st.text(living.a, living.b), SourceString, living.a, living.b)
		}
	}
	if street != nil && living == nil && !untyped && !realStreetTypes[street.typ] && !street.numbered {
		st.markUsed(min(street.mk, street.a), max(street.mk, street.b))
		if st.untypedStreet() { // "мкр Самал-2, Жолдасбекова 97"
			st.set(LivingArea, st.text(street.a, street.b), SourceString, street.a, street.b)
			return
		}
		for i := min(street.mk, street.a); i <= max(street.mk, street.b); i++ {
			st.toks[i].used = false
		}
	}
	if street != nil {
		st.set(StreetType, street.typ, SourceString, street.mk, street.mk)
		st.set(StreetName, st.text(street.a, street.b), SourceString, street.a, street.b)
		st.markUsed(min(street.mk, street.a), max(street.mk, street.b))
		st.streetE = max(street.mk, street.b) + 1
		return
	}
	if !untyped {
		st.untypedStreet()
	}
}

// untypedStreet finds words followed by a house number ("Кулманова 133", "Назарбаева8", "майлин д 10", "саина, 84").
func (st *state) untypedStreet() bool {
	for i := 0; i < len(st.toks); i++ {
		t := st.toks[i]
		if t.kind != tWord || t.used || t.mk != nil {
			continue
		}
		j := i
		for j+1 < len(st.toks) {
			n := st.toks[j+1]
			if n.kind == tWord && !n.used && n.mk == nil {
				j++
				continue
			}
			if n.kind == tPunct && (n.text == "-" || n.text == ".") && j+2 < len(st.toks) && st.toks[j+2].kind == tWord && !st.toks[j+2].used {
				j++
				continue
			}
			break
		}
		// "самал-2, 59": a number after a hyphen belongs to the name when the house number comes later
		if st.isPunct(j+1, "-") && glued(st.toks[j], st.toks[j+1]) && st.availNum(j+2) && glued(st.toks[j+1], st.toks[j+2]) &&
			st.laterNumberExists(j+2) {
			j += 2
		}
		h := j + 1
		for st.isPunct(h, ".") {
			h++
		}
		// a whole part of the line may be the street: "алматы, саина, 84"
		if st.isPunct(h, ",") && (i == 0 || st.toks[i-1].kind != tWord || st.toks[i-1].used) {
			h++
		}
		if st.availNum(h) || st.houseAt(h) {
			st.set(StreetName, st.text(i, j), SourceString, i, j)
			st.markUsed(i, j)
			st.streetE = j + 1
			return true
		}
		i = j
	}
	return false
}
