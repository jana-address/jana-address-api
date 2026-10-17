package parser

import "strings"

// units binds numbers to house/apartment/entrance/floor markers.
func (st *state) units() {
	var ms []int
	for i, t := range st.toks {
		if t.kind == tWord && !t.used && t.mk.isUnit() {
			ms = append(ms, i)
		}
	}
	bound := map[int]bool{}
	for _, m := range ms {
		j := m + 1
		for st.isPunct(j, ".") {
			j++
		}
		if st.toks[m].mk.class == mBlock && j < len(st.toks) && st.toks[j].kind == tWord && len([]rune(st.toks[j].text)) == 1 {
			st.toks[m].used, st.toks[j].used, bound[m] = true, true, true
		}
	}
	after, before := 0, 0
	bind := func(m, n int) {
		bound[m] = true
		if st.toks[m].mk.class == mBlock { // "корпус 2", "блок 3": not an output field, just consume the number
			st.toks[m].used, st.toks[n].used = true, true
			return
		}
		f := map[mclass]Field{mHouse: House, mApartment: Apartment, mEntrance: Entrance, mFloor: Floor}[st.toks[m].mk.class]
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
	// houseOnLeft: the number at l follows a street name, so it is the house ("Абая 10 кв 5", "абая10кв5")
	houseOnLeft := func(l int) bool {
		p := l - 1
		for st.isPunct(p, ".") {
			p--
		}
		if st.isPunct(p, "-") && p == l-1 && glued(st.toks[p], st.toks[l]) && p > 0 && st.toks[p-1].kind == tWord {
			return true // "Ақсай-1 үй 51б": 1 belongs to the name
		}
		if p < 0 || st.toks[p].kind != tWord {
			return false
		}
		if st.toks[p].mk == nil {
			return true
		}
		if st.toks[p].mk.class == mStreet && numberedAreaTypes[st.toks[p].mk.canon] {
			return true // "мкр.12 дом 117": 12 is the microdistrict
		}
		// a postfix street type after its name: "Естай көшесі 38 кв. 37"
		return st.toks[p].mk.class == mStreet && p > 0 && st.toks[p-1].kind == tWord && st.toks[p-1].mk == nil
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
			if t.kind != tNum && !st.isPunct(k, ".") && !(t.kind == tWord && t.mk.isUnit()) {
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
	neighbors := func(m int) (l, r int) {
		l, r = -1, -1
		i := m - 1
		for i >= 0 && st.isPunct(i, ".:№#") {
			i--
		}
		if st.availNum(i) && st.toks[m].mk.class != mBlock {
			l = i
		}
		i = m + 1
		for i < len(st.toks) && st.isPunct(i, ".:№#") {
			i++
		}
		if st.availNum(i) {
			r = i
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
				if before > after && !houseOnLeft(l) { // "Муканова 241 кв 34 5 этаж"
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
