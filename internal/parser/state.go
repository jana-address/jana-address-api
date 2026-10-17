package parser

import (
	"math"
	"sort"
	"strings"

	"github.com/jana-address/jana-address-api/internal/parser/dict"
	"github.com/jana-address/jana-address-api/internal/parser/text"
)

type state struct {
	d        *dict.Directory // one directory version for the whole record
	c        text.Cleaned
	toks     []token
	vals     map[Field]string
	src      map[Field]string
	written  map[Field]string // output forms of fields written in the line, set by writtenForms
	spans    []Span
	conflict bool
	streetE  int      // token index after the street name, -1 if none
	segs     [][2]int // first and last token of each segment
}

// ---------- helpers ----------

func (st *state) text(a, b int) string { // tokens a..b inclusive, verbatim from the cleaned string
	return string(st.c.Runes[st.toks[a].s:st.toks[b].e])
}

func (st *state) addSpan(f Field, a, b int) {
	if a < 0 || b < a || b >= len(st.toks) {
		return
	}
	st.spans = append(st.spans, Span{Field: f, Start: st.c.Start[st.toks[a].s], End: st.c.End[st.toks[b].e-1]})
}

// set stores a value; a different value for an already set field is a conflict and is ignored.
// A value written in the line replaces one taken from the directory ("село Боралдай, Іле ауданы").
func (st *state) set(f Field, v, src string, a, b int) {
	if v == "" {
		return
	}
	if old, ok := st.vals[f]; ok && !(src == SourceString && st.src[f] == SourceDirectory) {
		if text.Key(old) != text.Key(v) {
			st.conflict = true
			return
		}
	} else {
		st.vals[f], st.src[f] = v, src
	}
	if src == SourceString {
		st.addSpan(f, a, b)
	}
}

func (st *state) setDir(f Field, v string) {
	if _, ok := st.vals[f]; !ok && v != "" {
		st.vals[f], st.src[f] = v, SourceDirectory
	}
}

func (st *state) markUsed(a, b int) {
	for i := a; i <= b && i < len(st.toks); i++ {
		st.toks[i].used = true
	}
}

func (st *state) isPunct(i int, chars string) bool {
	return i >= 0 && i < len(st.toks) && st.toks[i].kind == tPunct && strings.Contains(chars, st.toks[i].text)
}

func (st *state) availNum(i int) bool {
	return i >= 0 && i < len(st.toks) && st.toks[i].kind == tNum && !st.toks[i].used && !st.toks[i].ignore
}

func (st *state) confidence() float64 {
	if len(st.vals) == 0 {
		return 0 // nothing parsed: null fields and confidence 0, as the contract says
	}
	has := func(f Field) bool { _, ok := st.vals[f]; return ok }
	s := 0.2
	if has(StreetName) {
		s += 0.3
	}
	if has(House) {
		s += 0.2
	}
	if has(City) {
		s += 0.15
	}
	if has(Province) {
		s += 0.05
	}
	if has(Apartment) || has(Entrance) || has(Floor) {
		s += 0.05
	}
	unexplained := 0
	for _, t := range st.toks {
		if !t.used && !t.ignore && t.kind != tPunct {
			unexplained++
		}
	}
	s *= math.Pow(0.88, float64(min(unexplained, 5)))
	if st.conflict {
		s *= 0.8
	}
	if !has(StreetName) && !has(House) {
		s = math.Min(s, 0.3)
	}
	s = math.Max(0.05, math.Min(0.99, s))
	return math.Round(s*100) / 100
}

func (st *state) sortedSpans() []Span {
	sp := append([]Span(nil), st.spans...)
	sort.SliceStable(sp, func(i, j int) bool { return sp[i].Start < sp[j].Start })
	out := sp[:0]
	end := -1
	for _, s := range sp {
		if s.Start >= end {
			out = append(out, s)
			end = s.End
		}
	}
	return out
}
