package parser

import (
	"strings"
	"sync"
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
		if newPostcodeAt(r, i) {
			txt := string(r[i : i+7])
			toks = append(toks, token{kind: tNum, text: txt, fold: text.Fold(txt), s: i, e: i + 7, ignore: true})
			i += 7
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

// newPostcodeAt reports whether r[i:] opens a postcode of the new format: the Latin letter of the region
// and six signs, letters and digits by turn ("X09F4F0", "A15E3C4"). 2GIS writes it after the old one,
// "160011/X09F4F0"; it is neither a street nor a house.
func newPostcodeAt(r []rune, i int) bool {
	const shape = "LDDLDLD"
	if i+len(shape) > len(r) || i > 0 && (isLetter(r[i-1]) || isDigit(r[i-1])) {
		return false
	}
	if e := i + len(shape); e < len(r) && (isLetter(r[e]) || isDigit(r[e])) {
		return false
	}
	for k, c := range r[i : i+len(shape)] {
		latin := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		if shape[k] == 'L' && !latin || shape[k] == 'D' && (c < '0' || c > '9') {
			return false
		}
	}
	return true
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
		if n := len(out); len(words) == 1 && n > 0 && out[n-1].kind == tWord && !glued(out[n-1], t) {
			if p := secondCityWord(d, out[n-1].fold, r, t.s, t.e); p > t.s {
				words = []token{wordToken(r, t.s, p), wordToken(r, p, t.e)}
			}
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

// typeBeforeStreet splits a short street type glued to a street the city's map knows, also one letter off,
// in a word of its own after the city: "алматы улпушкина", "актау пр-ттуран", "павлодар улторайгыфрова". A word the map knows whole, or one letter from a
// street it knows well, keeps its letters: "пригородный", "пермтина".
func typeBeforeStreet(r []rune, s, e int, city string, words []token) []token {
	whole := lookupKey(string(r[s:e]))
	f := text.Fold(string(r[s:e]))
	// the key of "пр-тсарыарка" reads "тсарыарка", one letter from Сарыарка: a hyphenated type is no part of a name
	hyphen := strings.HasPrefix(f, "пр-т") || strings.HasPrefix(f, "б-р")
	if gluedStreet(city, f) || !hyphen && (bySurname(city, whole) || closeStreet(city, whole)) {
		return words
	}
	for p := s + 2; p <= min(s+4, e-4); p++ {
		left, rest := text.Fold(string(r[s:p])), string(r[p:e])
		if m := dict.Markers[left]; m == nil || m.Class != dict.Street {
			continue
		}
		// a street one letter off only after "ул": "пр" opens many words ("привокзальный")
		if gluedStreet(city, text.Fold(rest)) || bySurname(city, lookupKey(rest)) || left == text.Fold("ул") && closeStreet(city, lookupKey(rest)) {
			return []token{wordToken(r, s, p), wordToken(r, p, e)}
		}
	}
	if p := typeOpening(r, s, e, city); p > s {
		return []token{wordToken(r, s, p), wordToken(r, p, e)}
	}
	return words
}

// openingTypes are the short street types, folded, that open a mistyped street the map does not know.
var openingTypes = []string{"пр-т", text.Fold("бульвар"), "б-р", "ул", "пр"}

// typeOpening returns where a short street type ends in a word after the city that no city of the map
// knows, nor a street one or two letters from it that opens with that type: "улгголя", "пртауелсиздик",
// "б-рмир" (s when there is none). "привокзальный", "улытау" and "протзанова" keep their letters.
func typeOpening(r []rune, s, e int, city string) int {
	if e-s < 5 || !strings.ContainsRune("уУпПбБ", r[s]) {
		return s
	}
	f := text.Fold(string(r[s:e]))
	if gluedStreet("", f) || dict.OSM().KnownAnywhere(lookupKey(string(r[s:e]))) || dict.FuzzyMarker(f) != nil {
		return s // "прспект" is a mistyped type word
	}
	for _, left := range openingTypes {
		if !strings.HasPrefix(f, left) {
			continue
		}
		n := len([]rune(left))
		rest := []rune(f)[n:]
		if len(rest) < 3 || len(rest) == 3 && n <= 2 {
			return s
		}
		for k := n + 1; k <= min(len([]rune(f)), 10); k++ {
			if m := dict.Markers[string([]rune(f)[:k])]; m != nil {
				return s // "проспекта", "улицы"
			}
		}
		if startsLikeStreet(city, f, left) || startsLikeStreet("", f, left) || streetLike("", f) {
			return s // "Улиханов" is Уалиханова
		}
		for p := s + n; p < e && p <= s+n+1; p++ { // the folded type drops "ь"
			if text.Fold(string(r[s:p])) == left && isLetter(r[p]) && isLetter(r[p-1]) {
				return p
			}
		}
		return s
	}
	return s
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
		if acronym && capsWord(d, r, s, i, e) {
			i++ // a word in capitals keeps its last letter: "КАРАГАНДАпичугина", "БАТЫРшкольная"
		}
		if (lowerUpper || acronym) && dict.Markers[text.Fold(string(r[s:e]))] == nil && !caseTypoCity(d, r, s, e) {
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
	if city != "" && (gluedStreet(city, w.fold) || bySurname(city, lookupKey(w.text))) {
		return []token{w} // "протозанова" after Усть-Каменогорск keeps its "пр", "кошкарбаева" its "кош"
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
			if !l.near {
				if pl, ok := spacedCity(d, l.fold); ok {
					l.place, l.unique, l.near = pl, true, true
				}
			}
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
	// a city with a stray letter or a typo right before its type word: "уральскккаласы", "костанайцқала",
	// "усть-каменогорсқала" (not "усть-каменогорсқ" and "ала")
	if city == "" {
		for p := min(e-3, top); p >= s+4; p-- {
			if !startsWithTypeWord(r[p:e]) {
				continue
			}
			if left, pl, unique, _ := prefix(p); unique {
				return split(p, cityName(pl, left))
			}
		}
	}
	// 2. a city at the start whose rest begins with a street the map knows there: "Аксукамзина" is Аксу
	// and Камзина, "уральскевразия" Уральск and Евразия, "алматдостык" Алматы and Достык. An exact city
	// goes first, then a close spelling, the longest first.
	// A street one letter off counts when no split gives a street as written: "оралевразмя", "кокшетуабылайхна",
	// but "алматжандосова" is Алмат and Жандосова.
	if city == "" && !dict.OSM().KnownAnywhere(w.fold) {
		typoP, typoCity := 0, ""
		for p := min(e-3, exactEnd); p >= s+4; p-- {
			pl, ok := d.Cities[text.Fold(string(r[s:p]))]
			if !ok {
				continue
			}
			switch streetFirst(d, r, p, e, pl.Name) {
			case 2:
				return split(p, pl.Name)
			case 1:
				// not when the city's last letter is typed twice: "шымкенттташенова"; the street may start with
				// that letter when the rest after it is no street: "есиккунава" is Есик and Кунаева
				if typoP == 0 && (unicode.ToLower(r[p]) != unicode.ToLower(r[p-1]) || streetFirst(d, r, p+1, e, pl.Name) == 0) {
					typoP, typoCity = p, pl.Name
				}
			}
		}
		// a letter more than an exact city goes with the street: "шымкентджангельдина" is Джангельдина and
		// "алматыддостык" keeps its typo, but a longer name may be mistyped: "лисаковмк…" is Лисаковск
		for p := min(e-3, top); p >= s+4; p-- {
			left, pl, unique, _ := prefix(p)
			if _, ok := d.Cities[left]; ok || exactLen > 0 && len([]rune(left)) < exactLen+2 && !doubledEnd(left, exactLen) {
				continue
			}
			if !unique {
				continue
			}
			if k := shortCityStreet(r, s, p, e, pl.Name); k > 0 && exactLen == 0 {
				return split(p-k, pl.Name)
			}
			switch streetFirst(d, r, p, e, pl.Name) {
			case 2:
				return split(p, pl.Name)
			case 1:
				// not "көкшетауу" before "алиханв"; a name of four letters is close to one city only: "тарзрыскуловва"
				if typoP == 0 && len([]rune(left)) >= 4 && !(exactLen > 0 && doubledEnd(left, exactLen)) {
					typoP, typoCity = p, pl.Name
				}
			}
		}
		if typoP > 0 {
			return split(typoP, typoCity)
		}
	}
	// a street the map knows before a type word at the end is not a city: "кунаевакөш" is not Конаев,
	// "сатпаевакөш" not Сатпаев
	te := typeEnd(r, s, e)
	if te > s && gluedStreet("", text.Fold(string(r[s:te]))) {
		return []token{wordToken(r, s, te), wordToken(r, te, e)}
	}
	// 3. a city with one typo at the start: "асстанаакжол", "шпмалғанул"; not for adjectives like "Алматинская"
	// and not before a preposition: "Ермекво" is a name, not "Ермак во". Next to an exact city only a longer
	// name, before it: "лисаковмк…" is Лисаковск, not Лисаков
	for p := top; p >= max(s+5, exactEnd+2) && city == ""; p-- {
		rest := text.Fold(string(r[p:e]))
		adj := -1 // where a known street starts in an adjective rest
		if adjectiveWord(rest) && !startsWithCityWord(rest) {
			if adj = adjectiveStreet(rest); adj < 0 {
				// an area of the one city close to the left part: "астагаботанический" is Астана and Ботанический
				if _, pl, unique, near := prefix(p); !near || !unique || pl.Name == "" || !cityArea(pl.Name, rest) {
					continue
				}
				adj = 0
			}
		}
		if !restOK(rest) {
			continue
		}
		if m := dict.Markers[rest]; m != nil && m.Class == dict.Landmark && len([]rune(rest)) <= 2 {
			continue
		}
		left, pl, _, near := prefix(p)
		if near && (exactLen == 0 || len([]rune(left)) >= exactLen+2) {
			if adj == 0 {
				return []token{wordToken(r, s, p), wordToken(r, p, e)}
			} else if adj > 0 {
				return []token{wordToken(r, s, p), wordToken(r, p, p+adj), wordToken(r, p+adj, e)}
			}
			if k := shortCityStreet(r, s, p, e, pl.Name); k > 0 {
				return split(p-k, pl.Name)
			}
			return split(p, cityName(pl, left))
		}
	}
	// 3b. a city of four letters or five with one typo before a street the map knows: "актусейфуллина" is
	// Актау and Сейфуллина
	if p := s + 4; city == "" && exactEnd == s && e-p >= 5 {
		if left, _, unique, _ := prefix(p); unique && len([]rune(left)) == 4 && dict.OSM().KnownAnywhere(text.Fold(string(r[p:e]))) {
			return []token{wordToken(r, s, p), wordToken(r, p, e)}
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
		short := city != "" && dict.AfterCityMarkers[left]
		if !(dict.GlueMarkers[left] && p-s >= 3 || short) {
			continue
		}
		if !restOK(rest) && !(len([]rune(left)) >= 4 && left != kalas && len([]rune(rest)) == 3 && !adjectiveRest(rest)) &&
			!(short && (gluedStreet(city, rest) || streetLike(city, rest))) {
			continue // "каласыабя", "улицааай", "ауылаба": the longest type word goes, a short street stays ("қаласаин"
			// is қала Саина); "улинтернациональная" after a city is ул Интернациональная
		}
		if short && p-s <= 3 && (startsLikeStreet(city, w.fold, left) || streetLike(city, w.fold)) {
			break // "протзанова", "кошкаррбаева", "улдала", "прмитина": a street of the city that starts with a type word
		}
		if left == kalas && p+1 < e && city != "" && sStreet(city, rest) {
			p-- // "қаласейфуллн" is қала and Сейфуллина
			left = text.Fold(string(r[s:p]))
		}
		next := ""
		if dict.GlueMarkers[left] || dict.AfterCityMarkers[left] {
			next = city
		}
		return split(p, next)
	}
	// 6. a type word at the end: "абайкошеси", "самалмкр", "қабанбайбатыркөш"
	if te > s {
		return append(splitWord(d, r, s, te, city), wordToken(r, te, e))
	}
	return []token{w}
}

// postfixGlued are Kazakh street types written after the name that end glued words too: "мирабульвары",
// "ботаническийтұйығы", "темирлановскоетасжолы".
var postfixGlued = map[string]bool{}

func init() {
	for _, w := range []string{"бульвары", "тұйығы", "тасжолы", "шоссе"} {
		postfixGlued[text.Fold(w)] = true
	}
}

// adjectiveStreet finds a street the map knows in an adjective rest after a mistyped city, maybe after
// "ул": 0 for the whole rest, 2 after "ул", -1 for none. "жезказгнпромышленная" is Жезказган and
// Промышленная, not an adjective of the city.
func adjectiveStreet(rest string) int {
	if dict.OSM().KnownAnywhere(rest) {
		return 0
	}
	if r := []rune(rest); len(r) > 6 && string(r[:2]) == "ул" && dict.OSM().KnownAnywhere(string(r[2:])) {
		return 2
	}
	return -1
}

// doubledEnd reports whether the folded left part is an exact city of n letters with its last letter typed
// twice: "алматыы" before "желтоксан" is Алматы, the street keeps its own letters.
func doubledEnd(left string, n int) bool {
	r := []rune(left)
	return len(r) == n+1 && r[n] == r[n-1]
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

// streetFirst tells whether the glued rest r[s:e] after a city starts with a street of that city, after
// the type words: "камзина", "улстроителей", "қаласыабая".
// It gives 2 for a street as written, 1 for one a letter off ("евразмя", "интернациональна" before
// "көшесі") or the first word of a street of two ("кабанбай" before "батыра") and 0 for none.
func streetFirst(d *dict.Directory, r []rune, s, e int, city string) int {
	for _, t := range splitWord(d, r, s, e, city) {
		if dict.Markers[t.fold] != nil {
			continue
		}
		n := len([]rune(t.fold))
		switch {
		case n >= 4 && gluedStreet(city, t.fold):
			return 2
		case n >= 6 && streetLike(city, t.fold), n == 3 && shortStreetLike(city, t.fold), n >= 5 && len(firstWords()[city][t.fold]) > 0:
			return 1
		}
		return 0
	}
	return 0
}

// gluedStreet reports whether the city has a street written as the word f.
func gluedStreet(city, f string) bool { return dict.OSM().GluedStreet(city, f) }

var kalas = text.Fold("қалас")

// typeWordStarts are the type words after a city that a stray letter may separate from it.
var typeWordStarts = [][]rune{[]rune("кала"), []rune("қала"), []rune("город"), []rune("горд"), []rune("ауыл"), []rune("мкр")}

// startsWithTypeWord reports whether the rest of a glued word opens with a city type word or "мкр".
func startsWithTypeWord(r []rune) bool {
next:
	for _, w := range typeWordStarts {
		if len(r) < len(w) {
			continue
		}
		for i, c := range w {
			if unicode.ToLower(r[i]) != c {
				continue next
			}
		}
		return true
	}
	return false
}

// capsWord reports whether r[s:i+1] is a word in capitals of five letters or more that a lowercase word
// follows, and not a city or a type word with the first letter of a capitalized name: "КАРАГАНДАпичугина"
// against "АЛМАТЫАбая", "МКРСамал" and a known street after it: "СМЕЙШакарима".
func capsWord(d *dict.Directory, r []rune, s, i, e int) bool {
	if i+1-s < 5 {
		return false
	}
	for _, c := range r[s : i+1] {
		if !unicode.IsUpper(c) {
			return false
		}
	}
	f := text.Fold(string(r[s:i]))
	_, city := d.Cities[f]
	return !city && dict.Markers[f] == nil && !gluedStreet("", text.Fold(string(r[i:e])))
}

// gluedKeys holds, per city ("" for all cities), the street keys of the map without spaces, and the last
// words of the streets of two words, by their first letter: the streets that a short type word glued before
// them may hide ("улы дала", "протозанова", "кошкарбаева").
var gluedKeys = sync.OnceValue(func() map[string]map[rune][][]rune {
	out := map[string]map[rune][][]rune{"": {}}
	for city, streets := range dict.OSM().Streets {
		m := map[rune][][]rune{}
		add := func(name string) {
			k := []rune(strings.ReplaceAll(name, " ", ""))
			if len(k) >= 5 && len(k) < text.MaxFuzzyLen {
				m[k[0]] = append(m[k[0]], k)
				out[""][k[0]] = append(out[""][k[0]], k)
			}
		}
		for key := range streets {
			name, typ, _ := strings.Cut(key, "\x00")
			if typ != dict.AnyType {
				continue
			}
			if first, _, _ := strings.Cut(name, " "); dict.Markers[first] != nil {
				continue // "ул кунаева" is a type word written into the name
			}
			add(name)
			if i := strings.LastIndexByte(name, ' '); i > 0 {
				add(name[i+1:])
			}
		}
		out[city] = m
	}
	return out
})

// cityGlued returns the glued street keys of the city that start with the letter of f.
func cityGlued(city string, f []rune) [][]rune {
	if len(f) < 2 {
		return nil
	}
	m, ok := gluedKeys()[city]
	if !ok {
		m = gluedKeys()[""]
	}
	return m[f[0]]
}

// startsLikeStreet reports whether the folded word f is a street of the city whose name opens with the type
// word left, alone or before a type word, also one letter off (two in a long name): "протзанова",
// "улыдаадангылы", "улдала".
func startsLikeStreet(city, f, left string) bool {
	w := []rune(f)
	for _, k := range cityGlued(city, w) {
		if !strings.HasPrefix(string(k), left) {
			continue
		}
		lim := 0
		if len(k) >= 9 {
			lim = 2
		} else if len(k) >= 6 {
			lim = 1
		}
		for n := len(k) - lim; n <= len(k)+lim && n <= len(w); n++ {
			rest := string(w[n:])
			if (rest == "" || dict.Markers[rest] != nil || postfixGlued[rest]) && text.DistanceLimit(w[:n], k, lim) <= lim {
				return true
			}
		}
	}
	return false
}

// streetLike reports whether the folded word f is a street of the city written without spaces, also one
// letter off: "сейфуллн" for Сейфуллина, "смагулова".
func streetLike(city, f string) bool {
	w := []rune(f)
	if len(w) < 5 || len(w) >= text.MaxFuzzyLen {
		return false
	}
	for _, k := range cityGlued(city, w) {
		if text.DistanceLimit(w, k, 1) <= 1 {
			return true
		}
	}
	return false
}

// sStreet reports whether the rest after "қалас" reads better with that "с": "қаласейфуллн" is қала
// Сейфуллина, "қаласатпаева" қала Сатпаева, "қаласмагуловакөш" қала Смагулова.
func sStreet(city, rest string) bool {
	if r := []rune(rest); typeEnd(r, 0, len(r)) > 0 {
		rest = string(r[:typeEnd(r, 0, len(r))])
	}
	return gluedStreet(city, "с"+rest) && !gluedStreet(city, rest) || streetLike(city, "с"+rest) && !streetLike(city, rest)
}

// cityPairs maps, per directory, the first word of the cities of two words to their second words, and
// the first letter to the first words of five letters or more.
var cityPairs sync.Map // *dict.Directory -> *pairIndex

type pairIndex struct {
	seconds map[string][]string
	firsts  map[rune][]string
}

func pairsOf(d *dict.Directory) *pairIndex {
	if m, ok := cityPairs.Load(d); ok {
		return m.(*pairIndex)
	}
	m := map[string][]string{}
	add := func(k string) {
		first, second, ok := strings.Cut(k, " ")
		if ok && !strings.Contains(second, " ") && len([]rune(second)) >= 4 {
			m[first] = append(m[first], second)
		}
	}
	for k := range d.Cities {
		add(k)
	}
	for k := range d.Settlements {
		if _, ok := d.Cities[k]; !ok {
			add(k)
		}
	}
	idx := &pairIndex{seconds: m, firsts: map[rune][]string{}}
	for first := range m {
		if r := []rune(first); len(r) >= 5 {
			idx.firsts[r[0]] = append(idx.firsts[r[0]], first)
		}
	}
	cityPairs.Store(d, idx)
	return idx
}

// secondCityWord returns where the second word of a city of two words ends when the street is glued to it:
// "отеген батырмира" is Отеген батыр and Мира, also with a letter off in a long word ("баьыр", or "отенен"
// before the word as written); s when not.
func secondCityWord(d *dict.Directory, prev string, r []rune, s, e int) int {
	idx := pairsOf(d)
	seconds, maxLim := idx.seconds[prev], 1
	if len(seconds) == 0 {
		// a first word one letter off takes the second word as written: "отенен батырсадовый"
		var pbuf [text.MaxFuzzyLen]rune
		pr := text.ToRunes(prev, pbuf[:0])
		if len(pr) < 5 || len(pr) >= text.MaxFuzzyLen {
			return s
		}
		for _, first := range idx.firsts[pr[0]] {
			var fbuf [text.MaxFuzzyLen]rune
			if text.DistanceLimit(pr, text.ToRunes(first, fbuf[:0]), 1) <= 1 {
				seconds = append(seconds, idx.seconds[first]...)
			}
		}
		maxLim = 0
	}
	if len(seconds) == 0 {
		return s
	}
	for lim := 0; lim <= maxLim; lim++ { // the exact word first: "батырмира" is not "баты" and "рмира"
		for _, sw := range seconds {
			w := []rune(sw)
			if lim > 0 && len(w) < 5 {
				continue
			}
			for _, p := range []int{s + len(w), s + len(w) + lim, s + len(w) - lim} {
				var buf [text.MaxFuzzyLen]rune
				if p <= e-3 && p-s < text.MaxFuzzyLen && text.DistanceLimit(text.ToRunes(text.Fold(string(r[s:p])), buf[:0]), w, lim) <= lim {
					return p
				}
			}
		}
	}
	return s
}

// caseTypoCity reports whether a word with a case change inside is a city with letters swapped:
// "Усть-аКменогорск".
func caseTypoCity(d *dict.Directory, r []rune, s, e int) bool {
	f := text.Fold(string(r[s:e]))
	if len([]rune(f)) < 8 {
		return false
	}
	_, ok := d.LookupCity(f)
	return ok
}

// typeEnd returns where a street type glued at the end of r[s:e] starts, the longest first: "абайкошеси",
// "самалмкр" (s when there is none).
func typeEnd(r []rune, s, e int) int {
	for p := s + 3; p <= e-2; p++ {
		right := text.Fold(string(r[p:e]))
		if dict.GlueMarkers[right] || right == kosh || postfixGlued[right] {
			return p
		}
	}
	return s
}

var kosh = text.Fold("көш")

// shortStreetLike reports whether a word of three letters is one letter from a street of four letters of the
// city: "аба" and "абй" for Абая or Абай.
func shortStreetLike(city, f string) bool {
	m, ok := shortKeys()[city]
	if !ok {
		return false
	}
	var buf [4]rune
	w := text.ToRunes(f, buf[:0])
	for _, k := range m[w[0]] {
		if text.DistanceLimit(w, k, 1) <= 1 {
			return true
		}
	}
	return false
}

// shortKeys holds, per city, its streets of four letters that the map knows well by their first letter.
var shortKeys = sync.OnceValue(func() map[string]map[rune][][]rune {
	out := map[string]map[rune][][]rune{}
	for city, streets := range dict.OSM().Streets {
		m := map[rune][][]rune{}
		for key, ref := range streets {
			name, typ, _ := strings.Cut(key, "\x00")
			if k := []rune(name); typ == dict.AnyType && len(k) == 4 && ref.Votes >= 3 {
				m[k[0]] = append(m[k[0]], k)
			}
		}
		out[city] = m
	}
	return out
})

// cityArea reports whether the city's map has an area named by the folded word f, well voted.
func cityArea(city, f string) bool {
	a, ok := dict.OSM().Areas[city][f]
	return ok && a.Share >= dict.MinStreetShare
}

// shortCityStreet: the city written short, the letters taken for its end starting a street the city knows:
// "лисаковстроителй" is Лисаков(ск) and Строителй, not Лисаковст and Роителй. It gives how many letters go
// to the street, 0 for none.
func shortCityStreet(r []rune, s, p, e int, city string) int {
	if city == "" {
		return 0
	}
	known := func(f string) bool {
		return gluedStreet(city, f) || streetLike(city, f) || gluedStreet("", f) || streetLike("", f)
	}
	if known(text.Fold(string(r[p:e]))) {
		return 0
	}
	name := text.Fold(city)
	for k := 1; k <= 2 && p-k-s >= 5; k++ {
		left, rest := text.Fold(string(r[s:p-k])), text.Fold(string(r[p-k:e]))
		if len(left) < len(name) && strings.HasPrefix(name, left) && known(rest) {
			return k
		}
	}
	return 0
}

// spacedKeys are the folded names of the cities of two words or more written without spaces, per directory.
var spacedKeys struct {
	sync.Mutex
	d    *dict.Directory
	keys map[string]dict.Place
}

// spacedCity finds the one city of two words or more one letter from the folded word f written glued:
// "отегенбавтыр" is Отеген батыр.
func spacedCity(d *dict.Directory, f string) (dict.Place, bool) {
	w := []rune(f)
	if len(w) < 8 || strings.Contains(f, " ") {
		return dict.Place{}, false
	}
	spacedKeys.Lock()
	if spacedKeys.d != d {
		spacedKeys.d, spacedKeys.keys = d, map[string]dict.Place{}
		for k, pl := range d.Cities {
			if strings.Contains(k, " ") {
				spacedKeys.keys[strings.ReplaceAll(k, " ", "")] = pl
			}
		}
	}
	keys := spacedKeys.keys
	spacedKeys.Unlock()
	var found dict.Place
	for k, pl := range keys {
		if text.DistanceLimit(w, []rune(k), 1) <= 1 {
			if found.Name != "" && found.Name != pl.Name {
				return dict.Place{}, false
			}
			found = pl
		}
	}
	return found, found.Name != ""
}
