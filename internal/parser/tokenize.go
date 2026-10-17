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
				(j+2 == n || r[j+2] == ',' || (j+3 < n && r[j+2] == ' ' && r[j+3] == ',') ||
					(r[j+2] == ' ' && (strings.ContainsRune("аАбБ", r[j+1]) || (!strings.ContainsRune("иИ", r[j+1]) && markerAt(r, j+3))))) {
				j += 2 // "10 а", "162 ж кв 15" -> "10а", "162ж" ("10 в подъезде" keeps the preposition)
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
					if singleLetterAt(r, m) {
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

// markerAt reports whether a marker word starts at r[i]: "кв", "подъезд", "эт".
func markerAt(r []rune, i int) bool {
	j := i
	for j < len(r) && isLetter(r[j]) {
		j++
	}
	if j == i {
		return false
	}
	m, ok := dict.Markers[text.Fold(string(r[i:j]))]
	return ok && m.Class != dict.Noise && m.Class != dict.Landmark
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

func splitGlued(d *dict.Directory, r []rune, toks []token) []token {
	out := make([]token, 0, len(toks))
	for _, t := range toks {
		if t.kind != tWord {
			out = append(out, t)
			continue
		}
		out = append(out, splitWord(d, r, t.s, t.e, false)...)
	}
	return out
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

// splitWord splits one glued word; afterCity allows short type words right after a city name.
func splitWord(d *dict.Directory, r []rune, s, e int, afterCity bool) []token {
	if e-s > maxGluedLen {
		return []token{wordToken(r, s, e)}
	}
	// 1. a case change: "ТаукеХана", "АлматыАбая"
	for i := s + 1; i < e; i++ {
		lowerUpper := i-s >= 2 && unicode.IsLower(r[i-1]) && unicode.IsUpper(r[i])
		acronym := i-s >= 2 && unicode.IsUpper(r[i-1]) && unicode.IsUpper(r[i]) && i+1 < e && unicode.IsLower(r[i+1]) // "ЖКХайвил"
		if lowerUpper || acronym {
			left := splitWord(d, r, s, i, afterCity)
			_, city := d.LookupCity(left[len(left)-1].fold)
			return append(left, splitWord(d, r, i, e, city)...)
		}
	}
	w := wordToken(r, s, e)
	if e-s < 3 || d.KnownWord(w.fold) {
		return []token{w}
	}
	split := func(p int, leftIsCity bool) []token {
		return append([]token{wordToken(r, s, p)}, splitWord(d, r, p, e, leftIsCity)...)
	}
	// 2. a city at the start, the longest first: "алматыабая", "шымкентабй", "оралқала…"
	exactCity := false
	for p := e - 1; p >= s+4; p-- {
		left, rest := text.Fold(string(r[s:p])), text.Fold(string(r[p:e]))
		_, city := d.Cities[left]
		if !city {
			continue
		}
		exactCity = true
		if (p-s >= 5 || startsWithCityWord(rest)) && (restOK(rest) || len([]rune(rest)) == 3 && !adjectiveRest(rest)) {
			return split(p, true)
		}
	}
	// 3. a city with one typo at the start: "асстанаакжол", "шпмалғанул"; not for adjectives like "Алматинская"
	for p := e - 1; p >= s+5 && !exactCity && !afterCity; p-- {
		left, rest := text.Fold(string(r[s:p])), text.Fold(string(r[p:e]))
		var buf [text.MaxFuzzyLen]rune
		lr := text.ToRunes(left, buf[:0])
		if len(lr) >= text.MaxFuzzyLen || !restOK(rest) || adjectiveWord(rest) && !startsWithCityWord(rest) {
			continue
		}
		if d.NearCity(lr) {
			return split(p, true)
		}
	}
	// 4. a type word at the start: "каласыабая", "мкрдом", after a city also "ул", "пр-т", "горд"
	for p := e - 1; p >= s+1; p-- {
		left, rest := text.Fold(string(r[s:p])), text.Fold(string(r[p:e]))
		if (dict.GlueMarkers[left] && p-s >= 3 || afterCity && dict.AfterCityMarkers[left]) && restOK(rest) {
			return split(p, afterCity && (dict.GlueMarkers[left] || dict.AfterCityMarkers[left]))
		}
	}
	// 5. a type word at the end: "абайкошеси", "самалмкр", "қабанбайбатыркөш"
	for p := s + 3; p <= e-2; p++ {
		right := text.Fold(string(r[p:e]))
		if dict.GlueMarkers[right] || right == text.Fold("көш") {
			return append(splitWord(d, r, s, p, afterCity), wordToken(r, p, e))
		}
	}
	return []token{w}
}
