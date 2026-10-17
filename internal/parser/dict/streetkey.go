package dict

import (
	"strings"
	"unicode"

	"github.com/jana-address/jana-address-api/internal/parser/text"
)

// typePhrases are type words of two words, folded: "шағын ауданы" is a microdistrict, not a district.
var typePhrases = map[string]string{
	"шагын ауданы": "микрорайон", "шагын аудан": "микрорайон", "ыкшам ауданы": "микрорайон", "ыкшам аудан": "микрорайон",
	"туйык кошеси": "переулок", "тас жолы": "шоссе", "жилой комплекс": "жилой комплекс",
}

// otherTypes are street type words that are not output types; they are dropped from the key.
var otherTypes = map[string]bool{
	"проезд": true, "тупик": true, "площадь": true, "набережная": true, "аллея": true, "тракт": true, "дорога": true,
	"аланы": true, "мкрн": true, "микрорайона": true, "улицы": true, "тук": true, // ТҮК: тұрғын үй кешені, a ЖК
}

// ordinal endings after a number: "8-й микрорайон", "5-я линия"
var ordinal = map[string]bool{"й": true, "я": true, "е": true, "ый": true, "ой": true, "ая": true, "ий": true, "го": true}

// StreetKey is the matching key of a street or living-area name and the street type written in it:
// "улица Абая" -> "абая", "улица"; "Абай даңғылы" -> "абай", "проспект"; "микрорайон Аксай-3А" ->
// "аксай 3а", "микрорайон"; "Д. Кунаева" -> "кунаева". Punctuation counts as a space and initials are
// dropped. The parser and cmd/osmref use the same key, so a written name meets the directory one.
func StreetKey(name string) (key, typ string) {
	f := []rune(text.Fold(name))
	for i, r := range f {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			f[i] = ' '
		}
	}
	w := strings.Fields(string(f))
	strip := func(i int) bool { // removes a type word or phrase at i (0 or the end), reports whether it did
		if len(w) < 2 {
			return false
		}
		if i == 0 && len(w) > 2 {
			if t, ok := typePhrases[w[0]+" "+w[1]]; ok {
				typ, w = pick(typ, t), w[2:]
				return true
			}
		}
		if i > 0 && len(w) > 2 {
			if t, ok := typePhrases[w[i-1]+" "+w[i]]; ok {
				typ, w = pick(typ, t), w[:i-1]
				return true
			}
		}
		x := w[i]
		if m, ok := Markers[x]; ok && len([]rune(x)) > 1 && (m.Class == Street || m.Class == Living) {
			typ = pick(typ, m.Canon)
		} else if !otherTypes[x] {
			return false
		}
		if i == 0 {
			w = w[1:]
		} else {
			w = w[:i]
		}
		return true
	}
	for strip(0) || strip(len(w)-1) {
	}
	for len(w) > 1 && len([]rune(w[0])) == 1 && unicode.IsLetter([]rune(w[0])[0]) {
		w = w[1:] // initials: "д кунаева"
	}
	out := w[:0]
	for i, x := range w {
		if ordinal[x] && i > 0 && isDigits(w[i-1]) {
			continue
		}
		out = append(out, x)
	}
	return strings.Join(out, " "), typ
}

func pick(have, t string) string {
	if have != "" {
		return have
	}
	return t
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// HouseKey is the matching form of a house number: lower case without spaces, "12 А" -> "12а".
func HouseKey(h string) string {
	return strings.ToLower(strings.Join(strings.Fields(h), ""))
}

// MinStreetShare is the share of a street's buildings that must lie in one district to restore the
// district from the street alone; a street split between districts more evenly gives no district.
const MinStreetShare = 0.75
