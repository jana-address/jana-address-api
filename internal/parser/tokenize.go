package parser

import (
	"strings"
	"unicode"

	"github.com/jana-address/jana-address-api/internal/parser/dict"
	"github.com/jana-address/jana-address-api/internal/parser/text"
)

type kind uint8

const (
	tWord kind = iota
	tNum
	tPunct
)

type token struct {
	kind   kind
	text   string // as in the cleaned string
	fold   string
	s, e   int // rune range in the cleaned string
	ignore bool
	mk     *dict.Marker
	used   bool
	seg    int
	tail   int // following tokens that belong to this marker
}

func isLetter(r rune) bool { return unicode.IsLetter(r) }
func isDigit(r rune) bool  { return unicode.IsDigit(r) }

// singleLetterAt reports whether r[i] is a letter that is not followed by another letter
// ("12а" keeps the letter, "12дом" does not).
func singleLetterAt(r []rune, i int) bool {
	return i < len(r) && isLetter(r[i]) && (i+1 == len(r) || !isLetter(r[i+1]))
}

// tokenize splits the cleaned runes into words, numbers and punctuation.
// Glued forms like "66квартал", "Назарбаева8", "кв12" become separate tokens.
func tokenize(r []rune) []token {
	var toks []token
	n := len(r)
	for i := 0; i < n; {
		ch := r[i]
		if ch == ' ' {
			i++
			continue
		}
		j := i + 1
		k := tPunct
		switch {
		case isDigit(ch):
			k = tNum
			for j < n && isDigit(r[j]) {
				j++
			}
			if singleLetterAt(r, j) && (r[j] != 'д' || afterHouseMarker(r, i)) { // "55д" is "55 дом", "11Д" and "д 11д" keep the letter
				j++
			} else if j+1 < n && r[j] == ' ' && singleLetterAt(r, j+1) &&
				(j+2 == n || strings.ContainsRune(",;()", r[j+2]) || (j+3 < n && r[j+2] == ' ' && strings.ContainsRune(",;()", r[j+3])) ||
					(r[j+2] == ' ' && (strings.ContainsRune("аАбБ", r[j+1]) || (!strings.ContainsRune("иИ", r[j+1]) && markerAt(r, j+3))))) {
				j += 2 // "10 а", "162 ж кв 15", "114 г (возле" -> "10а", "162ж", "114г" ("10 в подъезде" keeps the preposition)
			}
			// fraction: "55/6", "208 / 2"
			q := j
			if q < n && r[q] == ' ' {
				q++
			}
			if q < n && r[q] == '/' {
				q++
				if q < n && r[q] == ' ' {
					q++
				}
				if q < n && isDigit(r[q]) {
					m := q
					for m < n && isDigit(r[m]) {
						m++
					}
					if singleLetterAt(r, m) && (r[m] != 'д' || afterHouseMarker(r, i)) { // "68/12д" is "68/12 дом" too
						m++
					}
					j = m
				}
			}
		case isLetter(ch):
			k = tWord
			for j < n && (isLetter(r[j]) || (r[j] == '-' && j+1 < n && isLetter(r[j+1]))) {
				j++
			}
		}
		txt := string(r[i:j])
		t := token{kind: k, text: txt, fold: text.Fold(txt), s: i, e: j}
		if k == tNum {
			digits := 0
			for _, c := range txt {
				if isDigit(c) {
					digits++
				}
			}
			// postcodes (050006, 100008) and phone numbers are not address numbers
			if digits >= 6 {
				t.ignore = true
			}
		}
		toks = append(toks, t)
		i = j
	}
	return toks
}

// afterHouseMarker reports whether the number starting at r[i] follows "д", "дом", "үй" or "d".
func afterHouseMarker(r []rune, i int) bool {
	j := i
	for j > 0 && (r[j-1] == ' ' || r[j-1] == '.') {
		j--
	}
	k := j
	for k > 0 && isLetter(r[k-1]) {
		k--
	}
	if k == j {
		return false
	}
	m, ok := dict.Markers[text.Fold(string(r[k:j]))]
	return ok && m.Class == dict.House
}

// markerAt reports whether a marker word starts at r[i]: "кв", "подъезд", "эт", a ЖК written with a dot
// or a slash ("ж.к.", "ж/к"), or a mistyped unit word that keeps the ending of the marker ("кварира",
// "кваритра"; "подъезде" and "квартире" are inflected, so "10 в подъезде" keeps the preposition).
func markerAt(r []rune, i int) bool {
	j := i
	for j < len(r) && isLetter(r[j]) {
		j++
	}
	if j == i {
		return false
	}
	w := text.Fold(string(r[i:j]))
	if w == "ж" && j+1 < len(r) && (r[j] == '.' || r[j] == '/') && (r[j+1] == 'к' || r[j+1] == 'К') {
		return true
	}
	if m, ok := dict.Markers[w]; ok {
		return m.Class != dict.Noise && m.Class != dict.Landmark
	}
	return dict.FuzzyUnitTypo(w) != nil
}

// compactNum drops spaces inside a number token ("10 а" -> "10а").
func compactNum(s string) string { return strings.ReplaceAll(s, " ", "") }

// glued reports whether two tokens touch without a space.
func glued(a, b token) bool { return a.e == b.s }

// Words typed without spaces: "АлматыАбая", "Алматықаласыабая", "самалмкр".
// A word is split at a case change, after a city name at its start,
// and around marker words that are often glued to their neighbours.

// adjectiveStart marks a rest that continues the city name as an adjective: "Таразская", "Шымкентский".
var adjectiveStart = []string{"ск", "цк", "ин", "ов", "ев", "ая", "ий", "ый", "ой", "ое", "ие"}

func plausibleRest(f string) bool {
	if len([]rune(f)) < 4 {
		return false
	}
	for _, p := range adjectiveStart {
		if strings.HasPrefix(f, p) {
			return false
		}
	}
	return true
}

func wordToken(r []rune, s, e int) token {
	txt := string(r[s:e])
	return token{kind: tWord, text: txt, fold: text.Fold(txt), s: s, e: e}
}

// splitGlued splits the glued words; a word after a city and its type words is split as glued to that
// city: "алматы улпушкина", "астанақаласы улжансугурова".
func splitGlued(d *dict.Directory, r []rune, toks []token) []token {
	out := make([]token, 0, len(toks))
	city := ""
	for _, t := range toks {
		if t.kind != tWord {
			out = append(out, t)
			if t.kind == tNum {
				city = ""
			}
			continue
		}
		words := splitWord(d, r, t.s, t.e, "")
		if len(words) == 1 && city != "" {
			words = typeBeforeStreet(r, t.s, t.e, city, words)
		}
		out = append(out, words...)
		for _, w := range words {
			if p, ok := d.LookupCity(w.fold); ok {
				city = p.Name
			} else if m := dict.Markers[w.fold]; m == nil || m.Class != dict.City && m.Class != dict.Settlement {
				city = ""
			}
		}
	}
	return out
}

// typeBeforeStreet splits a short street type glued to a street the city's map knows, in a word of its own
// after the city: "алматы улпушкина", "актау пр-ттуран". A word the map knows whole, or one letter from a
// street it knows well, keeps its letters: "пригородный", "пермтина".
func typeBeforeStreet(r []rune, s, e int, city string, words []token) []token {
	whole := lookupKey(string(r[s:e]))
	if gluedStreet(city, text.Fold(string(r[s:e]))) || bySurname(city, whole) || closeStreet(city, whole) {
		return words
	}
	for p := s + 2; p <= min(s+4, e-4); p++ {
		left, rest := text.Fold(string(r[s:p])), string(r[p:e])
		if m := dict.Markers[left]; m == nil || m.Class != dict.Street {
			continue
		}
		if gluedStreet(city, text.Fold(rest)) || bySurname(city, lookupKey(rest)) {
			return []token{wordToken(r, s, p), wordToken(r, p, e)}
		}
	}
	return words
}

// adjectiveEnd marks a word that is an adjective made from a city name: "Алматинская", "Таразский".
var adjectiveEnd = []string{"ская", "ский", "ское", "ские", "ской", "ную", "ная", "ный", "ное"}

func adjectiveWord(f string) bool {
	for _, end := range adjectiveEnd {
		if strings.HasSuffix(f, end) {
			return true
		}
	}
	return false
}

// adjectiveRest: what is left after a city name only finishes an adjective ("ская" in "Таразская").
func adjectiveRest(f string) bool {
	for _, p := range adjectiveStart {
		if strings.HasPrefix(f, p) {
			return true
		}
	}
	return adjectiveWord(f)
}

// startsWithCityWord: a short city name is split off only before "қала", "город", "ауыл": "оралқала…"
func startsWithCityWord(f string) bool {
	for _, w := range []string{"кала", "город", "горд", "ауыл"} {
		if strings.HasPrefix(f, w) {
			return true
		}
	}
	return false
}

func restOK(f string) bool {
	_, marker := dict.Markers[f]
	return marker || plausibleRest(f)
}

// maxGluedLen bounds the words split by splitWord, which is quadratic in the word length.
// Real glued words are under 40 letters: "Алматықаласыабаякөшесі".
const maxGluedLen = 64

// splitWord splits one glued word; city is the city named right before it ("" for none), which allows short
// type words after it and keeps a street of that city whole.
func splitWord(d *dict.Directory, r []rune, s, e int, city string) []token {
	if e-s > maxGluedLen {
		return []token{wordToken(r, s, e)}
	}
	// 1. a case change: "ТаукеХана", "АлматыАбая"; a marker stays whole: "ZhK"
	for i := s + 1; i < e; i++ {
		lowerUpper := i-s >= 2 && unicode.IsLower(r[i-1]) && unicode.IsUpper(r[i])
		acronym := i-s >= 2 && unicode.IsUpper(r[i-1]) && unicode.IsUpper(r[i]) && i+1 < e && unicode.IsLower(r[i+1]) // "ЖКХайвил"
		if (lowerUpper || acronym) && dict.Markers[text.Fold(string(r[s:e]))] == nil {
			left := splitWord(d, r, s, i, city)
			next := ""
			if p, ok := d.LookupCity(left[len(left)-1].fold); ok {
				next = p.Name
			}
			return append(left, splitWord(d, r, i, e, next)...)
		}
	}
	w := wordToken(r, s, e)
	if e-s < 3 || d.KnownWord(w.fold) {
		return []token{w}
	}
	if city != "" && gluedStreet(city, w.fold) {
		return []token{w} // "протозанова" after Усть-Каменогорск keeps its "пр"
	}
	split := func(p int, leftCity string) []token {
		return append([]token{wordToken(r, s, p)}, splitWord(d, r, p, e, leftCity)...)
	}
	// prefixes that may be a city: the folded left part and the cities one letter from it, found once per word
	top := min(e-1, s+d.LongestCity+1+softSigns(r, s, e))
	var lefts [maxGluedLen + 1]struct {
		fold         string
		near, unique bool
		place        dict.Place
		done         bool
	}
	prefix := func(p int) (string, dict.Place, bool, bool) {
		l := &lefts[p-s]
		if !l.done {
			l.fold = text.Fold(string(r[s:p]))
			l.place, l.unique, l.near = d.NearCities(l.fold)
			l.done = true
		}
		return l.fold, l.place, l.unique, l.near
	}
	exactEnd, exactLen := s, 0 // the end and the folded length of the longest exact city at the start
	for p := top; p >= s+4 && exactEnd == s; p-- {
		if f := text.Fold(string(r[s:p])); d.Cities[f] != (dict.Place{}) {
			exactEnd, exactLen = p, len([]rune(f))
		}
	}
	// 2. a city at the start whose rest begins with a street the map knows there: "Аксукамзина" is Аксу
	// and Камзина, "уральскевразия" Уральск and Евразия, "алматдостык" Алматы and Достык. An exact city
	// goes first, then a close spelling, the longest first.
	if city == "" && !dict.OSM().KnownAnywhere(w.fold) {
		for p := min(e-3, exactEnd); p >= s+4; p-- {
			if pl, ok := d.Cities[text.Fold(string(r[s:p]))]; ok && streetFirst(d, r, p, e, pl.Name) {
				return split(p, pl.Name)
			}
		}
		// a letter more than an exact city goes with the street: "шымкентджангельдина" is Джангельдина and
		// "алматыддостык" keeps its typo, but a longer name may be mistyped: "лисаковмк…" is Лисаковск
		for p := min(e-3, top); p >= s+4; p-- {
			left, pl, unique, _ := prefix(p)
			if _, ok := d.Cities[left]; ok || exactLen > 0 && len([]rune(left)) < exactLen+2 {
				continue
			}
			if unique && streetFirst(d, r, p, e, pl.Name) {
				return split(p, pl.Name)
			}
		}
	}
	// 3. a city with one typo at the start: "асстанаакжол", "шпмалғанул"; not for adjectives like "Алматинская"
	// and not before a preposition: "Ермекво" is a name, not "Ермак во". Next to an exact city only a longer
	// name, before it: "лисаковмк…" is Лисаковск, not Лисаков
	for p := top; p >= max(s+5, exactEnd+2) && city == ""; p-- {
		rest := text.Fold(string(r[p:e]))
		if !restOK(rest) || adjectiveWord(rest) && !startsWithCityWord(rest) {
			continue
		}
		if m := dict.Markers[rest]; m != nil && m.Class == dict.Landmark && len([]rune(rest)) <= 2 {
			continue
		}
		left, pl, _, near := prefix(p)
		if near && (exactLen == 0 || len([]rune(left)) >= exactLen+2) {
			return split(p, cityName(pl, left))
		}
	}
	// 4. a city at the start, the longest first: "алматыабая", "шымкентабй", "оралқала…"
	for p := exactEnd; p >= s+4; p-- {
		left, rest := text.Fold(string(r[s:p])), text.Fold(string(r[p:e]))
		pl, ok := d.Cities[left]
		if !ok {
			continue
		}
		if (p-s >= 5 || startsWithCityWord(rest)) && (restOK(rest) || len([]rune(rest)) == 3 && !adjectiveRest(rest)) {
			return split(p, pl.Name)
		}
	}
	// 5. a type word at the start: "каласыабая", "мкрдом", after a city also "ул", "пр-т", "горд"
	for p := e - 1; p >= s+1; p-- {
		left, rest := text.Fold(string(r[s:p])), text.Fold(string(r[p:e]))
		if (dict.GlueMarkers[left] && p-s >= 3 || city != "" && dict.AfterCityMarkers[left]) && restOK(rest) {
			next := ""
			if dict.GlueMarkers[left] || dict.AfterCityMarkers[left] {
				next = city
			}
			return split(p, next)
		}
	}
	// 6. a type word at the end: "абайкошеси", "самалмкр", "қабанбайбатыркөш"
	for p := s + 3; p <= e-2; p++ {
		right := text.Fold(string(r[p:e]))
		if dict.GlueMarkers[right] || right == text.Fold("көш") {
			return append(splitWord(d, r, s, p, city), wordToken(r, p, e))
		}
	}
	return []token{w}
}

// softSigns counts the letters of r[s:e] that the folded key drops ("ь", "ъ"), so a bound on a key length
// becomes one on the written word.
func softSigns(r []rune, s, e int) int {
	n := 0
	for _, c := range r[s:e] {
		if strings.ContainsRune("ьъЬЪ", c) {
			n++
		}
	}
	return n
}

// cityName is the name of a city found by a close spelling, or the spelling itself when several cities are
// as close: the words after it are still read as glued to a city.
func cityName(p dict.Place, left string) string {
	if p.Name != "" {
		return p.Name
	}
	return left
}

// streetFirst reports whether the glued rest r[s:e] after a city starts with a street of that city, after
// the type words: "камзина", "улстроителей", "қаласыабая".
func streetFirst(d *dict.Directory, r []rune, s, e int, city string) bool {
	for _, t := range splitWord(d, r, s, e, city) {
		if dict.Markers[t.fold] != nil {
			continue
		}
		return len([]rune(t.fold)) >= 4 && gluedStreet(city, t.fold)
	}
	return false
}

// gluedStreet reports whether the city has a street written as the word f.
func gluedStreet(city, f string) bool { return dict.OSM().GluedStreet(city, f) }
