package parser

import (
	"strings"
	"unicode"
)

// Words typed without spaces: "АлматыАбая", "Алматықаласыабая", "самалмкр".
// A word is split at a case change, after a city name at its start,
// and around marker words that are often glued to their neighbours.

// glueMarkers can start or end a glued word.
var glueMarkers = map[string]bool{}

func init() {
	for _, w := range []string{"қаласы", "қалас", "город", "мкр", "мкрн", "микрорайон", "улица", "көшесі", "даңғылы",
		"проспект", "ауданы", "район", "ықшамаудан", "шағынаудан"} {
		glueMarkers[fold(w)] = true
	}
}

// adjectiveStart marks a rest that continues the city name as an adjective: "Таразская", "Шымкентский".
var adjectiveStart = []string{"ск", "цк", "ин", "ов", "ев", "ая", "ий", "ый", "ой", "ое", "ие"}

func knownWord(f string) bool {
	if _, ok := markers[f]; ok {
		return true
	}
	if _, ok := regions[f]; ok {
		return true
	}
	if _, ok := districts[f]; ok {
		return true
	}
	_, ok := lookupCity(f)
	return ok
}

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
	text := string(r[s:e])
	return token{kind: tWord, text: text, fold: fold(text), s: s, e: e}
}

func splitGlued(r []rune, toks []token) []token {
	out := make([]token, 0, len(toks))
	for _, t := range toks {
		if t.kind != tWord {
			out = append(out, t)
			continue
		}
		out = append(out, splitWord(r, t.s, t.e)...)
	}
	return out
}

func splitWord(r []rune, s, e int) []token {
	// 1. a case change: "ТаукеХана", "АлматыАбая"
	for i := s + 1; i < e; i++ {
		lowerUpper := unicode.IsLower(r[i-1]) && unicode.IsUpper(r[i])
		acronym := i-s >= 2 && unicode.IsUpper(r[i-1]) && unicode.IsUpper(r[i]) && i+1 < e && unicode.IsLower(r[i+1]) // "ЖКХайвил"
		if lowerUpper || acronym {
			return append(splitWord(r, s, i), splitWord(r, i, e)...)
		}
	}
	w := wordToken(r, s, e)
	if e-s < 6 || knownWord(w.fold) {
		return []token{w}
	}
	// 2. a city or a marker at the start, the longest first: "алматыабая", "каласыабая"
	for p := e - 3; p >= s+3; p-- {
		left, rest := fold(string(r[s:p])), fold(string(r[p:e]))
		_, city := cities[left]
		if (city && p-s >= 5 || glueMarkers[left]) && plausibleRest(rest) {
			return append([]token{wordToken(r, s, p)}, splitWord(r, p, e)...)
		}
	}
	// 3. a marker at the end: "абайкошеси", "самалмкр"
	for p := s + 3; p <= e-3; p++ {
		if glueMarkers[fold(string(r[p:e]))] {
			return append(splitWord(r, s, p), wordToken(r, p, e))
		}
	}
	return []token{w}
}
