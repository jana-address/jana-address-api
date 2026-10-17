// Package text is the string work of the parser that knows nothing about addresses:
// cleaning with offsets back to the raw string, matching keys and edit distance.
package text

import (
	"unicode"
	"unicode/utf8"
)

// Cleaned is the normalized input with a map back to the raw string.
// Start/End are UTF-16 offsets in the raw string for every cleaned rune,
// so spans can be sliced directly in JavaScript.
type Cleaned struct {
	Runes []rune
	Start []int
	End   []int
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

// Capital reports whether s starts with an upper-case letter.
func Capital(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsUpper(r)
}

// Clean drops invisible characters, unifies dashes/quotes and collapses whitespace.
// Only the first maxRunes characters are parsed.
func Clean(raw string) Cleaned {
	var c Cleaned
	u := 0
	pending, spaceAt := false, 0
	for _, ch := range raw {
		if len(c.Runes) >= maxRunes {
			break
		}
		w := 1
		if ch >= 0x10000 {
			w = 2
		}
		switch {
		case isInvisible(ch):
		case unicode.IsSpace(ch):
			if len(c.Runes) > 0 && !pending {
				pending, spaceAt = true, u
			}
		default:
			if pending {
				c.Runes = append(c.Runes, ' ')
				c.Start = append(c.Start, spaceAt)
				c.End = append(c.End, spaceAt+1)
				pending = false
			}
			c.Runes = append(c.Runes, mapChar(ch))
			c.Start = append(c.Start, u)
			c.End = append(c.End, u+w)
		}
		u += w
	}
	return c
}
