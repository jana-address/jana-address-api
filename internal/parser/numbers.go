package parser

import (
	"strings"

	"github.com/jana-address/jana-address-api/internal/parser/dict"
)

// units binds numbers to house/apartment/entrance/floor markers.
func (st *state) units() {
	var ms []int
	for i, t := range st.toks {
		if t.kind == tWord && !t.used && t.mk.IsUnit() {
			ms = append(ms, i)
		}
	}
	bound := map[int]bool{}
	for _, m := range ms {
		j := m + 1
		for st.isPunct(j, ".") {
			j++
		}
		if st.toks[m].mk.Class == dict.Block && j < len(st.toks) && st.toks[j].kind == tWord && len([]rune(st.toks[j].text)) == 1 {
			st.toks[m].used, st.toks[j].used, bound[m] = true, true, true
		}
	}
	after, before := 0, 0
	bind := func(m, n int) {
		bound[m] = true
		if st.toks[m].mk.Class == dict.Block { // "корпус 2", "блок 3": not an output field, just consume the number
			st.toks[m].used, st.toks[n].used = true, true
			return
		}
		f := map[dict.Class]Field{dict.House: House, dict.Apartment: Apartment, dict.Entrance: Entrance, dict.Floor: Floor}[st.toks[m].mk.Class]
		v := compactNum(st.toks[n].text)
		if f != House {
			v = strings.SplitN(v, "/", 2)[0]
		}
		if _, ok := st.vals[f]; ok { // repeated marker ("кв 5 кв 6"): keep the first value
			st.toks[m].used = true
			return
		}
		st.vals[f], st.src[f] = v, SourceString
		st.addSpan(f, min(m, n), max(m, n))
		st.toks[m].used, st.toks[n].used = true, true
		bound[m] = true
		if n > m {
			after++
		} else {
			before++
		}
	}
	// attachedRight returns the number written right after the marker, also across a dot: "кв5", "кв.5"
	attachedRight := func(m int) int {
		j := m + 1
		if st.isPunct(j, ".") && glued(st.toks[m], st.toks[j]) {
			j++
		}
		if st.availNum(j) && glued(st.toks[j-1], st.toks[j]) {
			return j
		}
		return -1
	}
	// chainEndsWithMarker: in "41д136пәтер" numbers come before markers, in "д73кв.11под6эт4" after them
	chainEndsWithMarker := func(m int) bool {
		last := m
		for k := m + 1; k < len(st.toks) && glued(st.toks[k-1], st.toks[k]); k++ {
			t := st.toks[k]
			if t.kind != tNum && !st.isPunct(k, ".") && !(t.kind == tWord && t.mk.IsUnit()) {
				break
			}
			last = k
		}
		return st.toks[last].kind == tWord
	}
	// 1. glued: "11дом", "4подьезд", "кв12"
	for _, m := range ms {
		left := m > 0 && st.availNum(m-1) && glued(st.toks[m-1], st.toks[m])
		right := attachedRight(m)
		switch {
		case left && right >= 0 && !chainEndsWithMarker(m):
			bind(m, right)
		case left:
			bind(m, m-1)
		case right >= 0:
			bind(m, right)
		}
	}
	after, before = 0, 0 // "4подъезд 12этаж" says nothing about "25 кв 300"
	// numAfter returns the free number right after the marker at m, also across ".", ":", "№": "кв. 5", "№ 3"
	numAfter := func(m int) int {
		i := m + 1
		for i < len(st.toks) && st.isPunct(i, ".:№#") {
			i++
		}
		if st.availNum(i) {
			return i
		}
		return -1
	}
	// closedRight: the number at n is taken by the unit marker written after it, which has no other number
	// of its own: "4 подъезд,", "76 кв подъезд12", "19 эт 290 кв." (290 is the apartment, so 19 is the floor)
	var closedRight func(n, depth int) bool
	closedRight = func(n, depth int) bool {
		k := n + 1
		if depth > 4 || k >= len(st.toks) || bound[k] || st.toks[k].used ||
			!st.toks[k].mk.IsUnit() || st.toks[k].mk.Class == dict.Block {
			return false
		}
		next := numAfter(k)
		return next < 0 || closedRight(next, depth+1)
	}
	// A block takes the number on its left only when none follows it, or the one that follows has its own
	// marker after it, and that number is not the house after a street: "206 дом 4 блок кв 48" is block 4,
	// so is "5 блок 4 подъезд"; in "Абая 12 корпус" 12 stays the house.
	neighbors := func(m int) (l, r int) {
		l, r = -1, numAfter(m)
		i := m - 1
		for i >= 0 && st.isPunct(i, ".:№#") {
			i--
		}
		if !st.availNum(i) {
			return
		}
		if st.toks[m].mk.Class != dict.Block {
			return i, r
		}
		if (r < 0 || closedRight(r, 0)) && !st.houseOnLeft(i) {
			return i, -1
		}
		return
	}
	// 2. single option first, repeat until stable; 3. then by the dominant direction
	for changed := true; changed; {
		changed = false
		for _, m := range ms {
			if bound[m] {
				continue
			}
			l, r := neighbors(m)
			switch {
			case l >= 0 && r < 0:
				bind(m, l)
				changed = true
			case r >= 0 && l < 0:
				bind(m, r)
				changed = true
			}
		}
		if changed {
			continue
		}
		for _, m := range ms {
			if bound[m] {
				continue
			}
			l, r := neighbors(m)
			if l >= 0 && r >= 0 {
				if before > after && !st.houseOnLeft(l) { // "Муканова 241 кв 34 5 этаж"
					bind(m, l)
				} else {
					bind(m, r)
				}
				changed = true
				break
			}
		}
	}
}

// houseOnLeft reports whether the number at l follows a street name, so it is the house ("Абая 10 кв 5", "абая10кв5",
// "ахмирово, 49 кв 149")
func (st *state) houseOnLeft(l int) bool {
	p := l - 1
	for st.isPunct(p, ".,\"") {
		p--
	}
	if st.isPunct(p, "-") && p == l-1 && glued(st.toks[p], st.toks[l]) && p > 0 && st.toks[p-1].kind == tWord {
		return true // "Ақсай-1 үй 51б": 1 belongs to the name
	}
	if p < l-1 && st.areaNumber(p) {
		return true // "мкр 5, 33 кв 109", "мкр Степной-1, 12 кв 61": the number after the comma is the house
	}
	if p < 0 || st.toks[p].kind != tWord {
		return false
	}
	if st.toks[p].mk == nil {
		return true
	}
	if st.toks[p].mk.Class == dict.Street && dict.NumberedAreaTypes[st.toks[p].mk.Canon] {
		return true // "мкр.12 дом 117": 12 is the microdistrict
	}
	// a postfix street type or ЖК after its name: "Естай көшесі 38 кв. 37", "Comfort City жк 47 кв 67",
	// "«Esentai City» ТҮК 23 кв 3"
	q := p - 1
	if st.toks[p].mk.Class == dict.Living && st.isPunct(q, "\"") {
		q--
	}
	return (st.toks[p].mk.Class == dict.Street || st.toks[p].mk.Class == dict.Living) && q >= 0 && st.toks[q].kind == tWord && st.toks[q].mk == nil
}

// areaNumber reports whether the number at i is the own number of a microdistrict or quarter, not a house:
// "мкр 5", "мкр.12", "Степной-1", "Самал-2".
func (st *state) areaNumber(i int) bool {
	if i < 1 || st.toks[i].kind != tNum {
		return false
	}
	if st.isPunct(i-1, "-") && glued(st.toks[i-1], st.toks[i]) && i > 1 && st.toks[i-2].kind == tWord && glued(st.toks[i-2], st.toks[i-1]) {
		return true
	}
	p := i - 1
	for st.isPunct(p, ".") {
		p--
	}
	if p < 0 || st.toks[p].kind != tWord {
		return false
	}
	m := st.toks[p].mk
	if m == nil || m.Class != dict.Street || !dict.NumberedAreaTypes[m.Canon] {
		return false
	}
	if p > 0 && st.toks[p-1].used && st.toks[p-1].mk == m {
		p-- // the first word of "ықшам ауданы"
	}
	if p == 0 {
		return true
	}
	// a postfix type after its name or number keeps the house after it: "Салтанат мкр. 49, 5 блок",
	// "5 мкр 50, 3 блок"; in "д 32/14 мкрн 11" the number before the type is the house
	b := st.toks[p-1]
	nameBefore := b.kind == tWord && b.mk == nil && !b.used
	numberBefore := st.availNum(p-1) && !st.afterUnit(p-1)
	return !nameBefore && !numberBefore
}

// afterUnit reports whether the token at i follows a unit marker: "д 32", "кв. 5".
func (st *state) afterUnit(i int) bool {
	p := i - 1
	for st.isPunct(p, ".:№#") {
		p--
	}
	return p >= 0 && st.toks[p].kind == tWord && st.toks[p].mk.IsUnit()
}

// houseAt reports whether a house marker or the bound house number starts at i: "д 10", "д72", "41үй".
func (st *state) houseAt(i int) bool {
	if i < 0 || i >= len(st.toks) {
		return false
	}
	t := st.toks[i]
	if t.kind == tWord && t.mk != nil && t.mk.Class == dict.House {
		return true
	}
	h, ok := st.vals[House]
	return ok && t.kind == tNum && t.used && compactNum(t.text) == h
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
