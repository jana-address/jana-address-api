package parser

import (
	"strings"

	"github.com/jana-address/jana-address-api/internal/parser/dict"
	"github.com/jana-address/jana-address-api/internal/parser/text"
)

// latinPairs spell Russian and Kazakh letters in the usual Latin transcription, longest first.
var latinPairs = []struct{ lat, cyr string }{
	{"shch", "щ"}, {"sch", "щ"}, {"zh", "ж"}, {"kh", "х"}, {"sh", "ш"}, {"ch", "ч"}, {"ts", "ц"}, {"yu", "ю"}, {"ya", "я"}, {"yo", "е"}, {"ye", "е"},
	{"a", "а"}, {"b", "б"}, {"c", "к"}, {"d", "д"}, {"e", "е"}, {"f", "ф"}, {"g", "г"}, {"h", "х"}, {"i", "и"}, {"j", "ж"},
	{"k", "к"}, {"l", "л"}, {"m", "м"}, {"n", "н"}, {"o", "о"}, {"p", "п"}, {"q", "к"}, {"r", "р"}, {"s", "с"}, {"t", "т"},
	{"u", "у"}, {"v", "в"}, {"w", "у"}, {"x", "кс"}, {"z", "з"},
}

// cyrillic respells a Latin name in Cyrillic for the directory lookups: "Kabanbay batyra" is
// "кабанбай батыра", "Zhandosova" "жандосова", "Yugo-Vostok" "юго-восток", "Satpayeva" "сатпаева". A "y"
// before a vowel starts "я", "ю", "е", after a vowel it is "й", elsewhere "ы". Output values keep the
// written spelling.
func cyrillic(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	vowel := false
	for i := 0; i < len(s); {
		if s[i] == 'y' && !(i+1 < len(s) && strings.IndexByte("auoe", s[i+1]) >= 0) {
			if vowel {
				b.WriteString("й")
				vowel = false
			} else {
				b.WriteString("ы")
				vowel = true
			}
			i++
			continue
		}
		done := false
		for _, p := range latinPairs {
			if strings.HasPrefix(s[i:], p.lat) {
				b.WriteString(p.cyr)
				vowel = strings.Contains("аеиоуюяе", p.cyr)
				i += len(p.lat)
				done = true
				break
			}
		}
		if !done {
			b.WriteByte(s[i])
			vowel = false
			i++
		}
	}
	return b.String()
}

// lookupName is the name as the directory spells it: Latin respelled in Cyrillic.
func lookupName(s string) string {
	if text.IsLatin(s) {
		return cyrillic(s)
	}
	return s
}

// lookupKey is the directory key of a written street or area name, Latin or Cyrillic.
func lookupKey(s string) string {
	k, _ := dict.StreetKey(lookupName(s))
	return k
}
