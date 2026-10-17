// Package parser splits a free-form Kazakhstan address into structured fields.
// It is a pure function of the input string: no caching, no lookups by record id.
package parser

import (
	"math"
	"sort"
	"strings"

	"github.com/jana-address/jana-address-api/internal/addr"
)

// Field names follow the output contract.
type Field string

const (
	Province   Field = "province"
	City       Field = "city"
	District   Field = "district"
	LivingArea Field = "living_area"
	StreetType Field = "street_type"
	StreetName Field = "street_name"
	House      Field = "house"
	Apartment  Field = "apartment"
	Entrance   Field = "entrance"
	Floor      Field = "floor"
	Landmark   Field = "landmark"
)

// Fields lists all parsed fields in output order.
var Fields = []Field{Province, City, District, LivingArea, StreetType, StreetName, House, Apartment, Entrance, Floor, Landmark}

// Source of a field value.
const (
	SourceString    = "string"
	SourceDirectory = "directory"
	SourceAbsent    = "absent"
)

// Span is a highlighted fragment of the raw string, in UTF-16 offsets.
type Span struct {
	Field Field `json:"field"`
	Start int   `json:"start"`
	End   int   `json:"end"`
}

// Output is the parse result with provenance.
type Output struct {
	Values     map[Field]string
	Sources    map[Field]string
	Spans      []Span
	Confidence float64
}

// Status buckets the confidence for the review UI.
func (o Output) Status() string {
	switch {
	case o.Confidence >= 0.8:
		return "confident"
	case o.Confidence >= 0.5:
		return "review"
	}
	return "unparsed"
}

// SourceOf returns the provenance of a field.
func (o Output) SourceOf(f Field) string {
	if s, ok := o.Sources[f]; ok {
		return s
	}
	return SourceAbsent
}

// Fill copies the values into the jury result.
func (o Output) Fill(r *addr.Result) {
	ptr := func(f Field) *string {
		if v, ok := o.Values[f]; ok && v != "" {
			return &v
		}
		return nil
	}
	r.Province, r.City, r.District, r.LivingArea = ptr(Province), ptr(City), ptr(District), ptr(LivingArea)
	r.StreetType, r.StreetName, r.House = ptr(StreetType), ptr(StreetName), ptr(House)
	r.Apartment, r.Entrance, r.Floor, r.Landmark = ptr(Apartment), ptr(Entrance), ptr(Floor), ptr(Landmark)
	r.Confidence = o.Confidence
}

type state struct {
	d        *directory // one directory version for the whole record
	c        cleaned
	toks     []token
	vals     map[Field]string
	src      map[Field]string
	spans    []Span
	conflict bool
	streetE  int // token index after the street name, -1 if none
}

// Parse parses one raw address string.
func Parse(raw string) Output {
	c := clean(raw)
	d := current.Load()
	st := &state{d: d, c: c, toks: d.splitGlued(c.r, tokenize(c.r)), vals: map[Field]string{}, src: map[Field]string{}, streetE: -1}
	st.classify()
	st.segment()
	st.admin()
	st.units()
	st.landmarks()
	st.street()
	st.livingPart()
	st.house()
	st.restore()
	return Output{Values: st.vals, Sources: st.src, Spans: st.sortedSpans(), Confidence: st.confidence()}
}

// ---------- helpers ----------

func (st *state) text(a, b int) string { // tokens a..b inclusive, verbatim from the cleaned string
	return string(st.c.r[st.toks[a].s:st.toks[b].e])
}

func (st *state) addSpan(f Field, a, b int) {
	if a < 0 || b < a || b >= len(st.toks) {
		return
	}
	st.spans = append(st.spans, Span{Field: f, Start: st.c.start[st.toks[a].s], End: st.c.end[st.toks[b].e-1]})
}

// set stores a value; a different value for an already set field is a conflict and is ignored.
func (st *state) set(f Field, v, src string, a, b int) {
	if v == "" {
		return
	}
	if old, ok := st.vals[f]; ok {
		if key(old) != key(v) {
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
