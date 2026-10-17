package parser

import (
	"strings"
	"unicode"
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
			} else if j+1 < n && r[j] == ' ' && singleLetterAt(r, j+1) && strings.ContainsRune("аАбБвВ", r[j+1]) &&
				(j+2 == n || r[j+2] == ',' || (r[j+2] == ' ' && !strings.ContainsRune("вВ", r[j+1]))) {
				j += 2 // "10 а" -> house "10а" ("10 в подъезде" keeps the preposition)
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
		text := string(r[i:j])
		t := token{kind: k, text: text, fold: fold(text), s: i, e: j}
		if k == tNum {
			digits := 0
			for _, c := range text {
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

// compactNum drops spaces inside a number token ("10 а" -> "10а").
func compactNum(s string) string { return strings.ReplaceAll(s, " ", "") }

// glued reports whether two tokens touch without a space.
func glued(a, b token) bool { return a.e == b.s }
