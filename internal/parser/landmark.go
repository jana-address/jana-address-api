package parser

import "strings"

// landmarks: from a trigger word to the next comma, marker or number.
func (st *state) landmarks() {
	var parts []string
	for i := 0; i < len(st.toks); i++ {
		t := st.toks[i]
		if t.kind == tWord && !t.used && postfixLandmark[t.fold] {
			a := i
			for a > 0 && st.toks[a-1].kind == tWord && !st.toks[a-1].used && st.toks[a-1].mk == nil && i-a < 3 {
				a--
			}
			if a < i {
				parts = append(parts, st.text(a, i))
				st.addSpan(Landmark, a, i)
				st.markUsed(a, i)
				continue
			}
		}
		if t.kind != tWord || t.used || t.mk == nil || t.mk.class != mLandmark {
			continue
		}
		j := i
		for j+1 < len(st.toks) {
			n := st.toks[j+1]
			if n.used || (n.kind == tPunct && n.text != "\"") || n.kind == tNum || (n.mk != nil && n.mk.class != mLandmark) {
				break
			}
			j++
		}
		if j+1 < len(st.toks) && st.availNum(j+1) && numberedPlace(st.toks[j].fold) &&
			(j+2 == len(st.toks) || st.isPunct(j+2, ",;")) {
			j++
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

var postfixLandmark = map[string]bool{}

func init() {
	for _, w := range []string{"жанында", "қасында", "маңында", "қарсы", "қарсысында", "артында"} {
		postfixLandmark[fold(w)] = true
	}
}

// numberedPlace reports whether a landmark noun usually carries a number: "школы 12", "садика 5".
func numberedPlace(f string) bool {
	for _, p := range []string{"школ", "гимназ", "лице", "сад", "детсад", "поликлиник", "больниц", "мектеп", "колледж"} {
		if strings.HasPrefix(f, p) {
			return true
		}
	}
	return false
}
