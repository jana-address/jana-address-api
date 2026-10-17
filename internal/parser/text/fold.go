package text

import (
	"strings"
	"unicode"
)

var foldMap = map[rune]string{
	'ё': "е", 'ә': "а", 'ғ': "г", 'қ': "к", 'ң': "н", 'ө': "о", 'ұ': "у", 'ү': "у", 'һ': "х", 'і': "и",
	'ъ': "", 'ь': "",
}

// Fold builds a matching key: lower case, Kazakh letters folded to Russian ones.
// It is never used for output values.
func Fold(s string) string {
	var b strings.Builder
	for _, ch := range strings.ToLower(s) {
		if m, ok := foldMap[ch]; ok {
			b.WriteString(m)
			continue
		}
		b.WriteRune(ch)
	}
	return b.String()
}

// Key is the folded form of a name with its words joined by single spaces.
func Key(s string) string { return strings.Join(strings.Fields(Fold(s)), " ") }

var kkToRu = strings.NewReplacer("Ә", "А", "ә", "а", "Ғ", "Г", "ғ", "г", "Қ", "К", "қ", "к", "Ң", "Н", "ң", "н",
	"Ө", "О", "ө", "о", "Ұ", "У", "ұ", "у", "Ү", "У", "ү", "у", "Һ", "Х", "һ", "х", "І", "И", "і", "и")

// RuSpelling replaces Kazakh letters with Russian ones, keeping the case.
func RuSpelling(s string) string { return kkToRu.Replace(s) }

// IsLatin reports whether the first letter of s is Latin.
func IsLatin(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return r < 0x250
		}
	}
	return false
}
