package parser

import (
	"strings"
	"unicode/utf8"

	"github.com/jana-address/jana-address-api/internal/parser/dict"
	"github.com/jana-address/jana-address-api/internal/parser/text"
)

// landmarks: from a trigger word to the next comma, marker or number.
func (st *state) landmarks() {
	var parts []string
	for i := 0; i < len(st.toks); i++ {
		t := st.toks[i]
		if t.kind == tWord && !t.used && postfixLandmark[t.fold] {
			a := i
			for a > 0 && st.toks[a-1].kind == tWord && !st.toks[a-1].used && st.toks[a-1].mk == nil && i-a < 3 {
				if _, city := st.d.Cities[st.toks[a-1].fold]; a < i && city {
					break // "Шымкент емхананың қасында": the city stays
				}
				a--
			}
			if a > 0 && a == i-1 && st.landmarkNum[a-1] {
				a-- // "64 мектептің жанында"
			}
			a += st.typedName(a, i-1) // "ул Говрова балабақшаның жанында": the typed street keeps its name
			if a < i {
				parts = append(parts, st.text(a, i))
				st.addSpan(Landmark, a, i)
				st.markUsed(a, i)
				continue
			}
		}
		if t.kind != tWord || t.used || t.mk == nil || t.mk.Class != dict.Landmark {
			continue
		}
		j := i
		for j+1 < len(st.toks) {
			n := st.toks[j+1]
			if n.used || (n.kind == tPunct && n.text != "\"") || n.kind == tNum || (n.mk != nil && n.mk.Class != dict.Landmark) {
				break
			}
			j++
		}
		// the number of a school or kindergarten: "возле школы 12", "(рядом со школой 39)", "напротив школы №60"
		k := j + 1
		sign := st.isPunct(k, "№#")
		if sign {
			k++
		}
		if st.availNum(k) && numberedPlace(st.toks[j].fold) && (sign || k+1 == len(st.toks) || st.isPunct(k+1, ",;()")) {
			j = k
		}
		parts = append(parts, st.text(i, j))
		st.addSpan(Landmark, i, j)
		st.markUsed(i, j)
		i = j
	}
	if len(parts) > 0 {
		st.vals[Landmark], st.src[Landmark] = strings.Join(parts, ", "), SourceString
	}
}

// landmarkNumbers keeps the number of a school or kindergarten before a Kazakh postfix landmark away from
// the unit markers: in "57 үй 12 мектептің жанында" and "18, 6 пәтер 50 мектептің жанында" the house is
// 57 and the apartment 6. The number is kept only when a house number can come before it in the line, and
// not when a marker written before it takes it: "Абая 64 мектептің жанында", "29 мкр 52 мектептің
// жанында" and "д 30 кв 88 мектептің жанында" keep their house and apartment.
func (st *state) landmarkNumbers() {
	for i := 2; i < len(st.toks); i++ {
		t, w, n := st.toks[i], st.toks[i-1], i-2
		if t.kind != tWord || t.used || !postfixLandmark[t.fold] || w.kind != tWord || w.used || w.mk != nil ||
			!numberedPlace(w.fold) || !st.availNum(n) || st.areaNumber(n) {
			continue
		}
		earlier := false
		for k := 0; k < n && !earlier; k++ {
			earlier = st.availNum(k) && !st.areaNumber(k) && !st.numberedAreaAt(k+1)
		}
		if !earlier || st.unitTakes(n) {
			continue
		}
		st.toks[n].used = true
		st.landmarkNum[n] = true
	}
}

// unitTakes reports whether the unit marker written before the number at n takes it. Numbers and markers
// alternate, and the chain shows how the line writes them: from a marker, "д 30 кв 88", every marker takes
// the number after it; from a number, "8 эт. 84 кв. 62" or "20 үй 64", the one before it, so the last
// number is free. A chain from a house after its street with a marker other than the house one is written
// from a marker too: in "Абая 5 кв 12" 5 is the house and 12 the apartment.
func (st *state) unitTakes(n int) bool {
	cur, first := n, -1
	for depth := 0; depth < 8; depth++ {
		p := cur - 1
		for st.isPunct(p, ".:№#") {
			p--
		}
		if p < 0 || st.toks[p].kind != tWord || !st.toks[p].mk.IsUnit() || st.toks[p].used {
			break
		}
		l := p - 1
		for st.isPunct(l, ".:№#") {
			l--
		}
		if !st.availNum(l) || st.areaNumber(l) {
			return true // "мкрн. 4 үй 14": 4 is the microdistrict
		}
		cur, first = l, p
	}
	if first < 0 {
		return false
	}
	return st.toks[first].mk.Class != dict.House && st.houseOnLeft(cur)
}

// numberedAreaAt reports whether a microdistrict or quarter marker starts at i: "5 мкр", "29 ықшам ауданы".
func (st *state) numberedAreaAt(i int) bool {
	if i >= len(st.toks) || st.toks[i].kind != tWord {
		return false
	}
	m := st.toks[i].mk
	return m != nil && m.Class == dict.Street && dict.NumberedAreaTypes[m.Canon]
}

// typedName returns how many of the words a..b, before a Kazakh landmark word, name the street or the
// area typed right before them, 0 when no street or area type precedes: in "ул. Мустафина Магнумның
// жанында" Мустафина is the street. The name is the longest run the city's map knows as a street, two
// words before a title ("ул. Кабанбай батыра"), or one word; at least one word stays the landmark. A
// type after a name types that name and is skipped.
func (st *state) typedName(a, b int) int {
	typed := false
	for m := a - 1; m >= 0 && m >= a-4 && !typed; m-- {
		t := st.toks[m]
		if t.kind != tWord || t.mk == nil || t.mk.Class != dict.Street && t.mk.Class != dict.Living {
			continue
		}
		if kazakhStreetPostfix[t.fold] || postfixAreaMarkers[t.fold] || m > 0 && st.toks[m-1].kind == tWord &&
			st.toks[m-1].mk == nil && !st.toks[m-1].used && utf8.RuneCountInString(st.toks[m-1].fold) > 1 {
			break // the type follows its name: "Уалиханов көшесі Small дүкенінің жанында"
		}
		j := m + 1 + t.tail
		for st.isPunct(j, ".\"") {
			j++
		}
		typed = j == a
	}
	if !typed || a >= b {
		return 0
	}
	for n := b - a; n >= 1; n-- {
		if st.streetVotes(st.vals[City], a, a+n-1) > 0 {
			return n
		}
	}
	if a+1 < b && streetTitles[st.toks[a+1].fold] {
		return 2
	}
	return 1
}

var postfixLandmark = map[string]bool{}

func init() {
	for _, w := range []string{"жанында", "қасында", "маңында", "қарсы", "қарсысында", "артында"} {
		postfixLandmark[text.Fold(w)] = true
	}
}

// numberedPlace reports whether a landmark noun usually carries a number: "школы 12", "садика 5".
func numberedPlace(f string) bool {
	// folded forms: "балабақша" (kindergarten), "емхана" (clinic), "аурухана" (hospital)
	for _, p := range []string{"школ", "гимназ", "лице", "сад", "детсад", "поликлиник", "больниц", "мектеп", "колледж",
		"балабакш", "емхан", "аурухан"} {
		if strings.HasPrefix(f, p) {
			return true
		}
	}
	return false
}
