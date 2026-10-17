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

func (d *directory) knownWord(f string) bool {
	if _, ok := markers[f]; ok {
		return true
	}
	if _, ok := d.regions[f]; ok {
		return true
	}
	if _, ok := d.districts[f]; ok {
		return true
	}
	_, ok := d.lookupCityWithin(f, 1)
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

func (d *directory) splitGlued(r []rune, toks []token) []token {
	out := make([]token, 0, len(toks))
	for _, t := range toks {
		if t.kind != tWord {
			out = append(out, t)
			continue
		}
		out = append(out, d.splitWord(r, t.s, t.e, false)...)
	}
	return out
}

// afterCityMarkers are short type words that are split off only right after a city: "алматыул…", "астанагородпр-т…".
var afterCityMarkers = map[string]bool{}

func init() {
	for _, w := range []string{"ул", "пр", "пр-т", "пер", "мкр", "мкрн", "горд", "кала", "қала", "қалас",
		"ауылы", "ауыл", "село", "көш", "даңғ"} {
		afterCityMarkers[fold(w)] = true
	}
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
	_, marker := markers[f]
	return marker || plausibleRest(f)
}

// splitWord splits one glued word; afterCity allows short type words right after a city name.
func (d *directory) splitWord(r []rune, s, e int, afterCity bool) []token {
	// 1. a case change: "ТаукеХана", "АлматыАбая"
	for i := s + 1; i < e; i++ {
		lowerUpper := unicode.IsLower(r[i-1]) && unicode.IsUpper(r[i])
		acronym := i-s >= 2 && unicode.IsUpper(r[i-1]) && unicode.IsUpper(r[i]) && i+1 < e && unicode.IsLower(r[i+1]) // "ЖКХайвил"
		if lowerUpper || acronym {
			left := d.splitWord(r, s, i, afterCity)
			_, city := d.lookupCity(left[len(left)-1].fold)
			return append(left, d.splitWord(r, i, e, city)...)
		}
	}
	w := wordToken(r, s, e)
	if e-s < 3 || d.knownWord(w.fold) {
		return []token{w}
	}
	split := func(p int, leftIsCity bool) []token {
		return append([]token{wordToken(r, s, p)}, d.splitWord(r, p, e, leftIsCity)...)
	}
	// 2. a city at the start, the longest first: "алматыабая", "шымкентабй", "оралқала…"
	exactCity := false
	for p := e - 1; p >= s+4; p-- {
		left, rest := fold(string(r[s:p])), fold(string(r[p:e]))
		_, city := d.cities[left]
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
		left, rest := fold(string(r[s:p])), fold(string(r[p:e]))
		var buf [maxFuzzyLen]rune
		lr := toRunes(left, buf[:0])
		if len(lr) >= maxFuzzyLen || !restOK(rest) || adjectiveWord(rest) && !startsWithCityWord(rest) {
			continue
		}
		for n := len(lr) - 1; n <= len(lr)+1; n++ {
			if n < 0 || n >= maxFuzzyLen {
				continue
			}
			for _, c := range d.cityByLen[n] {
				if osaDistanceLimit(lr, c.r, 1) <= 1 {
					return split(p, true)
				}
			}
		}
	}
	// 4. a type word at the start: "каласыабая", "мкрдом", after a city also "ул", "пр-т", "горд"
	for p := e - 1; p >= s+1; p-- {
		left, rest := fold(string(r[s:p])), fold(string(r[p:e]))
		if (glueMarkers[left] && p-s >= 3 || afterCity && afterCityMarkers[left]) && restOK(rest) {
			return split(p, afterCity && (glueMarkers[left] || afterCityMarkers[left]))
		}
	}
	// 5. a type word at the end: "абайкошеси", "самалмкр", "қабанбайбатыркөш"
	for p := s + 3; p <= e-2; p++ {
		right := fold(string(r[p:e]))
		if glueMarkers[right] || right == fold("көш") {
			return append(d.splitWord(r, s, p, afterCity), wordToken(r, p, e))
		}
	}
	return []token{w}
}
