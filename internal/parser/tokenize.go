package parser

import "unicode"

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
	mk     *marker
	used   bool
	seg    int
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
			if singleLetterAt(r, j) {
				j++
			}
			if j+1 < n && r[j] == '/' && isDigit(r[j+1]) {
				m := j + 1
				for m < n && isDigit(r[m]) {
					m++
				}
				if singleLetterAt(r, m) {
					m++
				}
				j = m
			}
		case isLetter(ch):
			k = tWord
			for j < n && (isLetter(r[j]) || (r[j] == '-' && j+1 < n && isLetter(r[j+1]))) {
				j++
			}
		}
		text := string(r[i:j])
		t := token{kind: k, text: text, fold: fold(text), s: i, e: j}
		if k == tNum {
			digits := 0
			for _, c := range text {
				if isDigit(c) {
					digits++
				}
			}
			// postcodes (050006) and phone numbers are not address numbers
			if digits >= 7 || (digits == 6 && text[0] == '0') {
				t.ignore = true
			}
		}
		toks = append(toks, t)
		i = j
	}
	return toks
}

// glued reports whether two tokens touch without a space.
func glued(a, b token) bool { return a.e == b.s }
