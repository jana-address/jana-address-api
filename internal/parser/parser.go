// Package parser splits a free-form Kazakhstan address into structured fields.
// It is a pure function of the input string: no caching, no lookups by record id.
package parser

import (
	"github.com/jana-address/jana-address-api/internal/addr"
	"github.com/jana-address/jana-address-api/internal/kato"
	"github.com/jana-address/jana-address-api/internal/parser/dict"
	"github.com/jana-address/jana-address-api/internal/parser/text"
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

// Levels of a value taken from the directory: how it was matched, from the most to the least exact.
const (
	LevelHouse      = "house"      // the building lies in the district (OSM)
	LevelStreet     = "street"     // most buildings of the street lie in one district, or its only type (OSM)
	LevelArea       = "area"       // a microdistrict or living area of the district or city
	LevelSettlement = "settlement" // the region and rayon of the written town or village (KATO)
	LevelDistrict   = "district"   // the city and region of the written district
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
	Levels     map[Field]string // for directory values only
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

// KATOStats describes what the classifier added to the directory.
type KATOStats = dict.KATOStats

// UseKATO builds a new directory from the base tables plus the classifier and makes it current.
// Records parsed afterwards see the new version; records in flight finish on the old one.
//
// Classifier names are used only where a settlement is certain: after "с.", "п.", "ауылы", "г.",
// or as a separate part next to a written district or region. Thousands of villages are called
// "Абай", "Достык", "Береке", like streets, so a bare word before a house number is never a village.
func UseKATO(recs []kato.Record) KATOStats { return dict.UseKATO(recs) }

// Parse parses one raw address string.
func Parse(raw string) Output {
	c := text.Clean(raw)
	d := dict.Current()
	st := &state{d: d, c: c, toks: splitGlued(d, c.Runes, tokenize(c.Runes)), vals: map[Field]string{}, src: map[Field]string{}, written: map[Field]string{}, lvl: map[Field]string{}, streetE: -1}
	st.classify()
	st.segment()
	st.admin()
	st.units()
	st.landmarks()
	st.street()
	st.livingPart()
	st.house()
	st.restore()
	st.writtenForms()
	return Output{Values: st.vals, Sources: st.src, Levels: st.lvl, Spans: st.sortedSpans(), Confidence: st.confidence()}
}
