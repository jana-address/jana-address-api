package parser

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// cleaned is the normalized input with a map back to the raw string.
// start/end are UTF-16 offsets in the raw string for every cleaned rune,
// so spans can be sliced directly in JavaScript.
type cleaned struct {
	r     []rune
	start []int
	end   []int
}

func isInvisible(ch rune) bool {
	switch {
	case ch >= 0x200B && ch <= 0x200F, ch >= 0x202A && ch <= 0x202E, ch >= 0x2060 && ch <= 0x2064:
		return true
	case ch == 0xFEFF, ch == 0x00AD:
		return true
	case unicode.IsControl(ch) && !unicode.IsSpace(ch):
		return true
	}
	return false
}

func mapChar(ch rune) rune {
	switch {
	case ch >= 0x2010 && ch <= 0x2015, ch == 0x2212:
		return '-'
	case ch == '«', ch == '»', ch == '„', ch == '“', ch == '”', ch == '‘', ch == '’', ch == '`':
		return '"'
	case ch == '\\':
		return '/'
	}
	return ch
}

// maxRunes bounds the cleaned input. The longest real address is about 120 characters, and several
// stages are quadratic in the number of tokens, so a megabyte of commas must not reach them.
const maxRunes = 512

// capital reports whether s starts with an upper-case letter.
func capital(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsUpper(r)
}

// clean drops invisible characters, unifies dashes/quotes and collapses whitespace.
// Only the first maxRunes characters are parsed.
func clean(raw string) cleaned {
	var c cleaned
	u := 0
	pending, spaceAt := false, 0
	for _, ch := range raw {
		if len(c.r) >= maxRunes {
			break
		}
		w := 1
		if ch >= 0x10000 {
			w = 2
		}
		switch {
		case isInvisible(ch):
		case unicode.IsSpace(ch):
			if len(c.r) > 0 && !pending {
				pending, spaceAt = true, u
			}
		default:
			if pending {
				c.r = append(c.r, ' ')
				c.start = append(c.start, spaceAt)
				c.end = append(c.end, spaceAt+1)
				pending = false
			}
			c.r = append(c.r, mapChar(ch))
			c.start = append(c.start, u)
			c.end = append(c.end, u+w)
		}
		u += w
	}
	return c
}

var foldMap = map[rune]string{
	'ё': "е", 'ә': "а", 'ғ': "г", 'қ': "к", 'ң': "н", 'ө': "о", 'ұ': "у", 'ү': "у", 'һ': "х", 'і': "и",
	'ъ': "", 'ь': "",
}

// fold builds a matching key: lower case, Kazakh letters folded to Russian ones.
// It is never used for output values.
func fold(s string) string {
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

func isLatin(s string) bool {
	for _, r := range s {
		if isLetter(r) {
			return r < 0x250
		}
	}
	return false
}
