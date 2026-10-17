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
	st := &state{c: c, toks: splitGlued(c.r, tokenize(c.r)), vals: map[Field]string{}, src: map[Field]string{}, streetE: -1}
	st.classify()
	st.segment()
	st.admin()
	st.units()
	st.landmarks()
	st.street()
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

// ---------- stages ----------

func (st *state) classify() {
	for i := range st.toks {
		t := &st.toks[i]
		if t.kind != tWord {
			continue
		}
		// "шағын аудан(ы)" = микрорайон (two words)
		if (t.fold == "шагын" || t.fold == "ыкшам") && i+1 < len(st.toks) && (st.toks[i+1].fold == "аудан" || st.toks[i+1].fold == "ауданы") {
			t.mk = markers["мкр"]
			st.toks[i+1].mk = t.mk
			st.toks[i+1].used = true // second word is part of the marker
			t.tail = 1
			continue
		}
		// ЖК written in several tokens: "ж.к.", "ж/к", "жилой комплекс", "тұрғын үй кешені"
		if n := st.livingTail(i); n > 0 {
			t.mk = markers["жк"]
			t.tail = n
			for k := i + 1; k <= i+n; k++ {
				st.toks[k].used = true
				st.toks[k].mk = nil
			}
			continue
		}
		if m, ok := markers[t.fold]; ok && t.mk == nil {
			t.mk = m
			if m.class == mNoise {
				if i == 0 || st.toks[i-1].kind == tPunct || (st.toks[i-1].mk != nil && st.toks[i-1].mk.class == mNoise) {
					t.used = true
					continue
				}
				t.mk = nil
			}
		}
		// typo in a long marker word: "подлъезд", "кваритра"; a typo right before a known city: "гролд актобе"
		if t.mk == nil && !adminKnown(mCity, t.fold) {
			if m := fuzzyMarker(t.fold); m != nil {
				t.mk = m
			} else if i+1 < len(st.toks) && adminKnown(mCity, st.toks[i+1].fold) &&
				osaDistanceLimit([]rune(t.fold), []rune("город"), 2) <= 2 {
				t.mk = markers["город"]
			}
		}
		// an initial inside a street name: "ул. Д. Кунаева", "пр. С. Сейфуллина"
		if len([]rune(t.text)) == 1 && st.isPunct(i+1, ".") && i+2 < len(st.toks) && st.toks[i+2].kind == tWord {
			p := i - 1
			for st.isPunct(p, ".") {
				p--
			}
			if p >= 0 && st.toks[p].mk != nil && st.toks[p].mk.class == mStreet {
				t.mk = nil
				continue
			}
		}
		// "с." before a name is a village, otherwise a preposition
		if t.fold == "с" && st.isPunct(i+1, ".") {
			t.mk = &marker{class: mSettlement}
		}
		if t.fold == "п" {
			j := i + 1
			for st.isPunct(j, ".") {
				j++
			}
			if j < len(st.toks) && st.toks[j].kind == tWord && markers[st.toks[j].fold] == nil {
				t.mk = markers["пос"]
			}
		}
	}
}

// livingTail returns how many tokens after i complete a multi-token ЖК marker.
func (st *state) livingTail(i int) int {
	at := func(k int, w string) bool { return k < len(st.toks) && st.toks[k].fold == fold(w) }
	switch {
	case at(i, "ж"):
		if (st.isPunct(i+1, ".") || st.isPunct(i+1, "/")) && at(i+2, "к") {
			if st.isPunct(i+3, ".") {
				return 3
			}
			return 2
		}
	case at(i, "жилой"):
		if i+1 < len(st.toks) && osaDistanceLimit([]rune(st.toks[i+1].fold), []rune("комплекс"), 2) <= 2 {
			return 1
		}
	case at(i, "тұрғын"):
		if at(i+1, "үй") && at(i+2, "кешені") {
			return 2
		}
	}
	return 0
}

// segment numbers tokens by ',', ';', brackets and a '/' between two words (bilingual pairs).
func (st *state) segment() {
	seg := 0
	for i := range st.toks {
		t := &st.toks[i]
		if t.kind == tPunct {
			sep := strings.Contains(",;()", t.text)
			if t.text == "/" && !t.used && i > 0 && i+1 < len(st.toks) && st.toks[i-1].kind == tWord && st.toks[i+1].kind == tWord {
				sep = true
			}
			if sep {
				seg++
				t.seg = -1
				continue
			}
		}
		t.seg = seg
	}
}

func (st *state) segRange(seg int) (int, int) {
	a, b := -1, -1
	for i, t := range st.toks {
		if t.seg == seg {
			if a < 0 {
				a = i
			}
			b = i
		}
	}
	return a, b
}

// postfixAdmin markers usually follow the name: "Алматы қаласы", "Тұран ауданы", "Турксибский район".
var postfixAdmin = map[string]bool{}

func init() {
	for _, w := range []string{"қаласы", "қалас", "ауданы", "аудан", "облысы", "район", "р-н", "рн", "область", "обл", "обл-ть", "ауылы", "ауыл", "ауд", "қ", "қала"} {
		postfixAdmin[fold(w)] = true
	}
}

func (st *state) keyOf(a, b int) string {
	var parts []string
	for i := a; i <= b; i++ {
		if st.toks[i].kind == tWord {
			parts = append(parts, st.toks[i].fold)
		}
	}
	return strings.Join(parts, " ")
}

func adminKnown(class mclass, k string) bool {
	switch class {
	case mRegion:
		_, ok := regions[k]
		return ok
	case mDistrict:
		_, ok := districts[k]
		return ok
	}
	_, ok := lookupCity(k)
	return ok
}

// wordRun returns consecutive unmarked words next to the marker m in direction dir (+1/-1).
func (st *state) wordRun(m, dir int) (int, int) {
	j := m + dir
	for dir > 0 && st.isPunct(j, ".") {
		j++
	}
	a, b := -1, -1
	for k := j; k >= 0 && k < len(st.toks); k += dir {
		t := st.toks[k]
		if t.kind != tWord || t.used || t.mk != nil || t.seg != st.toks[m].seg {
			break
		}
		if a < 0 {
			a, b = k, k
		}
		if dir > 0 {
			b = k
		} else {
			a = k
		}
	}
	return a, b
}

// adminName picks the name of an admin marker: the longest directory match next to it,
// otherwise a single word when more text follows ("г. Арысь Абая 10" -> "Арысь").
func (st *state) adminName(m int) (int, int) {
	class := st.toks[m].mk.class
	fa, fb := st.wordRun(m, +1)
	ba, bb := st.wordRun(m, -1)
	gluedAfter := m > 0 && st.toks[m-1].kind == tWord && glued(st.toks[m-1], st.toks[m])
	backward := ba >= 0 && (postfixAdmin[st.toks[m].fold] || fa < 0 || gluedAfter)
	fwdKnown := func() bool {
		for b := fb; fa >= 0 && b >= fa; b-- {
			if adminKnown(class, st.keyOf(fa, b)) {
				return true
			}
		}
		return false
	}
	if backward {
		for a := ba; a <= bb; a++ {
			k := st.keyOf(a, bb)
			if _, city := cities[k]; city && class == mDistrict && fwdKnown() {
				break
			}
			if adminKnown(class, k) {
				return a, bb
			}
		}
	}
	if fa >= 0 { // a known name after the marker wins over an unknown one before it: "Астана р-н Нура"
		for b := fb; b >= fa; b-- {
			if adminKnown(class, st.keyOf(fa, b)) {
				return fa, b
			}
		}
	}
	if backward {
		if ba == 0 || st.toks[ba-1].kind == tPunct {
			return ba, bb
		}
		return bb, bb
	}
	if fa < 0 {
		return -1, -1
	}
	if fb > fa && st.availNum(fb+1) {
		return fa, fa
	}
	return fa, fb
}

func (st *state) admin() {
	// 1. explicit markers: "г. Алматы", "Алматы қаласы", "Тұран ауданы", "Карагандинская обл"
	for i := range st.toks {
		t := st.toks[i]
		if t.kind != tWord || t.used || !t.mk.isAdmin() {
			continue
		}
		a, b := st.adminName(i)
		if a < 0 {
			continue
		}
		k, name := st.keyOf(a, b), st.text(a, b)
		switch t.mk.class {
		case mRegion:
			if r, ok := regions[k]; ok {
				st.set(Province, r, SourceString, a, b)
			} else {
				st.set(Province, name, SourceString, a, b)
			}
		case mDistrict:
			st.setDistrict(k, name, a, b)
		default:
			st.setCity(k, name, a, b)
		}
		st.markUsed(min(i, a), max(i, b))
	}

	maxSeg := 0
	for _, t := range st.toks {
		maxSeg = max(maxSeg, t.seg)
	}
	for seg := 0; seg <= maxSeg; seg++ {
		a, b := st.segRange(seg)
		if a < 0 {
			continue
		}
		// skip leading postcodes: "050000 Алматы ул Абая 10"
		for a <= b && (st.toks[a].used || (st.toks[a].kind == tNum && st.toks[a].ignore) || st.isPunct(a, ".")) {
			a++
		}
		if a > b {
			continue
		}
		clean := st.toks[a].kind == tWord // only unmarked words, no numbers
		for i := a; i <= b; i++ {
			t := st.toks[i]
			if t.used || t.kind == tNum || (t.kind == tWord && t.mk != nil) {
				clean = false
			}
		}
		k := st.keyOf(a, b)
		switch {
		// 2. a segment that is just a known name: "Алматы", "Турксибский"
		case clean && (cities[k] != place{} || (seg == 0 && adminKnown(mCity, k))): // close spelling only in the first segment
			st.setCity(k, st.text(a, b), a, b)
			st.markUsed(a, b)
		case clean && regions[k] != "" && (len([]rune(k)) == 3 || strings.HasSuffix(k, "ская")):
			st.set(Province, regions[k], SourceString, a, b)
			st.markUsed(a, b)
		case clean && strings.HasSuffix(k, "ий") && adminKnown(mDistrict, k):
			st.setDistrict(k, st.text(a, b), a, b)
			st.markUsed(a, b)
		default:
			// 3. a known city inside a street segment: "Алматы Сатпаева 5", "050000 Алматы ул Абая 10"
			for i := a; i <= b; i++ {
				t := st.toks[i]
				if t.kind != tWord || t.used || t.mk != nil || !adminKnown(mCity, t.fold) {
					continue
				}
				// a close spelling counts only before another word: "алматыы абая 10" is a city,
				// "Сатпаева 22" is a street (one letter from the town Сатпаев)
				areaNext := i+2 <= b && st.toks[i+1].kind == tNum && st.toks[i+2].mk != nil && numberedAreaTypes[st.toks[i+2].mk.canon]
				if _, exact := cities[t.fold]; !exact && i+1 <= b && st.toks[i+1].kind != tWord && !areaNext {
					continue
				}
				p := i - 1
				for st.isPunct(p, ".") {
					p--
				}
				if p >= 0 && st.toks[p].mk != nil && st.toks[p].mk.class == mStreet {
					continue // "ул. Астана" is a street, not the city
				}
				if cur, set := st.vals[City]; set {
					// the city written twice: "алматы алматы жандосова 58"
					if pl, _ := lookupCity(t.fold); pl.name == cur && i < b && st.toks[i+1].kind == tWord {
						st.toks[i].used = true
					}
					break
				}
				st.setCity(t.fold, t.text, i, i)
				st.toks[i].used = true
			}
		}
	}
}

func (st *state) setCity(k, name string, a, b int) {
	if p, ok := lookupCity(k); ok {
		st.set(City, p.name, SourceString, a, b)
		st.setDir(Province, p.province)
		st.setDir(District, p.district)
		return
	}
	st.set(City, ruSpelling(name), SourceString, a, b)
}

func (st *state) setDistrict(k, name string, a, b int) {
	if d, ok := districts[k]; ok {
		st.set(District, d.name, SourceString, a, b)
		st.setDir(City, d.city)
		st.setDir(Province, d.province)
		return
	}
	st.set(District, name, SourceString, a, b)
}

// units binds numbers to house/apartment/entrance/floor markers.
func (st *state) units() {
	var ms []int
	for i, t := range st.toks {
		if t.kind == tWord && !t.used && t.mk.isUnit() {
			ms = append(ms, i)
		}
	}
	bound := map[int]bool{}
	for _, m := range ms {
		j := m + 1
		for st.isPunct(j, ".") {
			j++
		}
		if st.toks[m].mk.class == mBlock && j < len(st.toks) && st.toks[j].kind == tWord && len([]rune(st.toks[j].text)) == 1 {
			st.toks[m].used, st.toks[j].used, bound[m] = true, true, true
		}
	}
	after, before := 0, 0
	bind := func(m, n int) {
		bound[m] = true
		if st.toks[m].mk.class == mBlock { // "корпус 2", "блок 3": not an output field, just consume the number
			st.toks[m].used, st.toks[n].used = true, true
			return
		}
		f := map[mclass]Field{mHouse: House, mApartment: Apartment, mEntrance: Entrance, mFloor: Floor}[st.toks[m].mk.class]
		v := compactNum(st.toks[n].text)
		if f != House {
			v = strings.SplitN(v, "/", 2)[0]
		}
		if _, ok := st.vals[f]; ok { // repeated marker ("кв 5 кв 6"): keep the first value
			st.toks[m].used = true
			return
		}
		st.vals[f], st.src[f] = v, SourceString
		st.addSpan(f, min(m, n), max(m, n))
		st.toks[m].used, st.toks[n].used = true, true
		bound[m] = true
		if n > m {
			after++
		} else {
			before++
		}
	}
	// houseOnLeft: the number at l follows a street name, so it is the house ("Абая 10 кв 5", "абая10кв5")
	houseOnLeft := func(l int) bool {
		p := l - 1
		for st.isPunct(p, ".") {
			p--
		}
		if p < 0 || st.toks[p].kind != tWord {
			return false
		}
		if st.toks[p].mk == nil {
			return true
		}
		// a postfix street type after its name: "Естай көшесі 38 кв. 37"
		return st.toks[p].mk.class == mStreet && p > 0 && st.toks[p-1].kind == tWord && st.toks[p-1].mk == nil
	}
	// attachedRight returns the number written right after the marker, also across a dot: "кв5", "кв.5"
	attachedRight := func(m int) int {
		j := m + 1
		if st.isPunct(j, ".") && glued(st.toks[m], st.toks[j]) {
			j++
		}
		if st.availNum(j) && glued(st.toks[j-1], st.toks[j]) {
			return j
		}
		return -1
	}
	// chainEndsWithMarker: in "41д136пәтер" numbers come before markers, in "д73кв.11под6эт4" after them
	chainEndsWithMarker := func(m int) bool {
		last := m
		for k := m + 1; k < len(st.toks) && glued(st.toks[k-1], st.toks[k]); k++ {
			t := st.toks[k]
			if t.kind != tNum && !st.isPunct(k, ".") && !(t.kind == tWord && t.mk.isUnit()) {
				break
			}
			last = k
		}
		return st.toks[last].kind == tWord
	}
	// 1. glued: "11дом", "4подьезд", "кв12"
	for _, m := range ms {
		left := m > 0 && st.availNum(m-1) && glued(st.toks[m-1], st.toks[m])
		right := attachedRight(m)
		switch {
		case left && right >= 0 && !chainEndsWithMarker(m):
			bind(m, right)
		case left:
			bind(m, m-1)
		case right >= 0:
			bind(m, right)
		}
	}
	after, before = 0, 0 // "4подъезд 12этаж" says nothing about "25 кв 300"
	neighbors := func(m int) (l, r int) {
		l, r = -1, -1
		i := m - 1
		for i >= 0 && st.isPunct(i, ".:№#") {
			i--
		}
		if st.availNum(i) && st.toks[m].mk.class != mBlock {
			l = i
		}
		i = m + 1
		for i < len(st.toks) && st.isPunct(i, ".:№#") {
			i++
		}
		if st.availNum(i) {
			r = i
		}
		return
	}
	// 2. single option first, repeat until stable; 3. then by the dominant direction
	for changed := true; changed; {
		changed = false
		for _, m := range ms {
			if bound[m] {
				continue
			}
			l, r := neighbors(m)
			switch {
			case l >= 0 && r < 0:
				bind(m, l)
				changed = true
			case r >= 0 && l < 0:
				bind(m, r)
				changed = true
			}
		}
		if changed {
			continue
		}
		for _, m := range ms {
			if bound[m] {
				continue
			}
			l, r := neighbors(m)
			if l >= 0 && r >= 0 {
				if before > after && !houseOnLeft(l) { // "Муканова 241 кв 34 5 этаж"
					bind(m, l)
				} else {
					bind(m, r)
				}
				changed = true
				break
			}
		}
	}
}

// landmarks: from a trigger word to the next comma, marker or number.
func (st *state) landmarks() {
	var parts []string
	for i := 0; i < len(st.toks); i++ {
		t := st.toks[i]
		if t.kind == tWord && !t.used && postfixLandmark[t.fold] {
			a := i
			for a > 0 && st.toks[a-1].kind == tWord && !st.toks[a-1].used && st.toks[a-1].mk == nil && i-a < 3 {
				a--
			}
			if a < i {
				parts = append(parts, st.text(a, i))
				st.addSpan(Landmark, a, i)
				st.markUsed(a, i)
				continue
			}
		}
		if t.kind != tWord || t.used || t.mk == nil || t.mk.class != mLandmark {
			continue
		}
		j := i
		for j+1 < len(st.toks) {
			n := st.toks[j+1]
			if n.used || (n.kind == tPunct && n.text != "\"") || n.kind == tNum || (n.mk != nil && n.mk.class != mLandmark) {
				break
			}
			j++
		}
		if j+1 < len(st.toks) && st.availNum(j+1) && numberedPlace(st.toks[j].fold) &&
			(j+2 == len(st.toks) || st.isPunct(j+2, ",;")) {
			j++
		}
		parts = append(parts, st.text(i, j))
		st.addSpan(Landmark, i, j)
		st.markUsed(i, j)
		i = j
	}
	if len(parts) > 0 {
		st.vals[Landmark], st.src[Landmark] = strings.Join(parts, ", "), SourceString
	}
}

var postfixLandmark = map[string]bool{}

func init() {
	for _, w := range []string{"жанында", "қасында", "маңында", "қарсы", "қарсысында", "артында"} {
		postfixLandmark[fold(w)] = true
	}
}

// numberedPlace reports whether a landmark noun usually carries a number: "школы 12", "садика 5".
func numberedPlace(f string) bool {
	for _, p := range []string{"школ", "гимназ", "лице", "сад", "детсад", "поликлиник", "больниц", "мектеп", "колледж"} {
		if strings.HasPrefix(f, p) {
			return true
		}
	}
	return false
}

type cand struct {
	typ      string
	mk       int // marker token index
	a, b     int // name token range
	numbered bool
	isLiving bool
}

func (st *state) laterNumberExists(i int) bool {
	if _, ok := st.vals[House]; ok {
		return true
	}
	for j := i + 1; j < len(st.toks); j++ {
		if st.availNum(j) {
			return true
		}
	}
	return false
}

// collectForward reads a street name after a type marker: "ул. Курильская", "Мкр Аксай 3а".
func (st *state) collectForward(m int, typ string) (int, int) {
	j := m + 1 + st.toks[m].tail
	for st.isPunct(j, ".\"") {
		j++
	}
	a, b := -1, -1
	for k := j; k < len(st.toks); k++ {
		t := st.toks[k]
		if t.used || t.seg != st.toks[m].seg && t.seg >= 0 {
			break
		}
		switch t.kind {
		case tWord:
			if t.mk != nil && typ == "жилой комплекс" && a >= 0 && t.mk.class == mStreet &&
				!(k+1 < len(st.toks) && st.toks[k+1].kind == tWord && st.toks[k+1].mk == nil && !st.toks[k+1].used) {
				break
			}
			if t.mk != nil {
				return a, b
			}
			if typ == "жилой комплекс" && a >= 0 && isLatin(st.toks[b].text) != isLatin(t.text) {
				return a, b
			}
			if !realStreetTypes[typ] && a >= 0 && st.postfixStreetAt(k+1) {
				return a, b
			}
		case tNum:
			if t.ignore {
				return a, b
			}
			if numberedAreaTypes[typ] {
				hyphened := a >= 0 && st.isPunct(k-1, "-") && glued(st.toks[k-1], t) // "Самал-2"
				if !(hyphened || st.laterNumberExists(k)) {
					return a, b
				}
				if a < 0 {
					a = k
				}
				return a, k
			}
			// "ул. 8 Марта": a number right after the type followed by a word
			if a < 0 && k+1 < len(st.toks) && st.toks[k+1].kind == tWord && st.toks[k+1].mk == nil && !st.toks[k+1].used &&
				!adminKnown(mCity, st.toks[k+1].fold) {
				break
			}
			return a, b
		case tPunct:
			if (t.text == "-" || t.text == "\"") && a >= 0 && k+1 < len(st.toks) && st.toks[k+1].kind != tPunct {
				continue
			}
			if t.text == "." && a >= 0 && len([]rune(st.toks[k-1].text)) == 1 {
				continue
			}
			return a, b
		}
		if a < 0 {
			a = k
		}
		b = k
	}
	return a, b
}

// collectBackward reads a name before a postfix type: "Рыбинский переулок", "Нурсити 2 микрорайон", "66квартал".
func (st *state) collectBackward(m int, typ string) (int, int) {
	a, b := -1, -1
	for k := m - 1; k >= 0; k-- {
		t := st.toks[k]
		if t.used || t.seg != st.toks[m].seg {
			break
		}
		ok := false
		switch t.kind {
		case tWord:
			ok = t.mk == nil
		case tNum:
			ok = numberedAreaTypes[typ] && !t.ignore
		case tPunct:
			ok = t.text == "-" && a >= 0 || strings.Contains("\"«»", t.text)
		}
		if !ok {
			break
		}
		if t.kind == tPunct && t.text != "-" {
			continue // a quote around the name
		}
		if b < 0 {
			b = k
		}
		a = k
	}
	return a, b
}

// postfixStreetAt reports whether a real street type word starts at i: "көшесі", "көш.", "ул.".
func (st *state) postfixStreetAt(i int) bool {
	return i < len(st.toks) && st.toks[i].kind == tWord && !st.toks[i].used && st.toks[i].mk != nil &&
		st.toks[i].mk.class == mStreet && realStreetTypes[st.toks[i].mk.canon]
}

func isLatin(s string) bool {
	for _, r := range s {
		if isLetter(r) {
			return r < 0x250
		}
	}
	return false
}

func hasLetter(s string) bool {
	for _, r := range s {
		if isLetter(r) {
			return true
		}
	}
	return false
}

func (st *state) street() {
	var cands []cand
	covered := -1 // a type word inside a collected name is not a new street: "жк Экспо Бульвар"
	for i, t := range st.toks {
		if i <= covered || t.kind != tWord || t.used || t.mk == nil || (t.mk.class != mStreet && t.mk.class != mLiving) {
			continue
		}
		typ := t.mk.canon
		c := cand{typ: typ, mk: i, isLiving: t.mk.class == mLiving}
		c.a, c.b = st.collectForward(i, typ)
		if numberedAreaTypes[typ] && st.availNum(i-1) && c.a >= 0 && c.a == c.b && st.toks[c.a].kind == tNum {
			c.a = -1 // "6 шағын ауданы 10": 6 is the area, 10 the house
		}
		if c.a < 0 {
			c.a, c.b = st.collectBackward(i, typ)
		}
		if c.a < 0 {
			continue
		}
		covered = max(covered, c.b)
		cands = append(cands, c)
	}

	var street, living *cand
	for i := range cands {
		c := &cands[i]
		if realStreetTypes[c.typ] && street == nil {
			street = c
		}
	}
	for i := range cands {
		c := &cands[i]
		if c == street {
			continue
		}
		if street == nil && !c.isLiving {
			street = c
		} else if living == nil {
			living = c
		}
	}

	untyped := false
	if living != nil {
		st.markUsed(min(living.mk, living.a), max(living.mk, living.b))
		if street == nil {
			untyped = st.untypedStreet()
		}
		if street == nil && !untyped && living.isLiving {
			street = living // ЖК without a street is the street: "Астана жк Северное сияние дом 7"
		} else {
			st.set(LivingArea, st.text(living.a, living.b), SourceString, living.a, living.b)
		}
	}
	if street != nil && living == nil && !untyped && !realStreetTypes[street.typ] && !street.numbered {
		st.markUsed(min(street.mk, street.a), max(street.mk, street.b))
		if st.untypedStreet() { // "мкр Самал-2, Жолдасбекова 97"
			st.set(LivingArea, st.text(street.a, street.b), SourceString, street.a, street.b)
			return
		}
		for i := min(street.mk, street.a); i <= max(street.mk, street.b); i++ {
			st.toks[i].used = false
		}
	}
	if street != nil {
		st.set(StreetType, street.typ, SourceString, street.mk, street.mk)
		st.set(StreetName, st.text(street.a, street.b), SourceString, street.a, street.b)
		st.markUsed(min(street.mk, street.a), max(street.mk, street.b))
		st.streetE = max(street.mk, street.b) + 1
		return
	}
	if !untyped {
		st.untypedStreet()
	}
}

// untypedStreet finds words followed by a house number ("Кулманова 133", "Назарбаева8", "майлин д 10", "саина, 84").
func (st *state) untypedStreet() bool {
	for i := 0; i < len(st.toks); i++ {
		t := st.toks[i]
		if t.kind != tWord || t.used || t.mk != nil {
			continue
		}
		j := i
		for j+1 < len(st.toks) {
			n := st.toks[j+1]
			if n.kind == tWord && !n.used && n.mk == nil {
				j++
				continue
			}
			if n.kind == tPunct && (n.text == "-" || n.text == ".") && j+2 < len(st.toks) && st.toks[j+2].kind == tWord && !st.toks[j+2].used {
				j++
				continue
			}
			break
		}
		h := j + 1
		for st.isPunct(h, ".") {
			h++
		}
		// a whole part of the line may be the street: "алматы, саина, 84"
		if st.isPunct(h, ",") && (i == 0 || st.toks[i-1].kind == tPunct || st.toks[i-1].used) {
			h++
		}
		if st.availNum(h) || st.houseAt(h) {
			st.set(StreetName, st.text(i, j), SourceString, i, j)
			st.markUsed(i, j)
			st.streetE = j + 1
			return true
		}
		i = j
	}
	return false
}

// houseAt reports whether a house marker or the bound house number starts at i: "д 10", "д72", "41үй".
func (st *state) houseAt(i int) bool {
	if i < 0 || i >= len(st.toks) {
		return false
	}
	t := st.toks[i]
	if t.kind == tWord && t.mk != nil && t.mk.class == mHouse {
		return true
	}
	h, ok := st.vals[House]
	return ok && t.kind == tNum && t.used && compactNum(t.text) == h
}

func (st *state) house() {
	if _, ok := st.vals[House]; !ok {
		from := st.streetE
		if from < 0 {
			from = 0
		}
		for k := from; k < len(st.toks); k++ {
			if st.availNum(k) {
				st.vals[House], st.src[House] = compactNum(st.toks[k].text), SourceString
				st.addSpan(House, k, k)
				st.toks[k].used = true
				break
			}
		}
	}
	// "дом 5, 12" — a single leftover number after the house is the apartment
	if _, ok := st.vals[Apartment]; ok {
		return
	}
	if _, ok := st.vals[House]; !ok {
		return
	}
	left := -1
	for k := range st.toks {
		if st.availNum(k) {
			if left >= 0 {
				return
			}
			left = k
		}
	}
	if left >= 0 && strings.IndexFunc(st.toks[left].text, isLetter) < 0 && !strings.Contains(st.toks[left].text, "/") {
		st.vals[Apartment], st.src[Apartment] = st.toks[left].text, SourceString
		st.addSpan(Apartment, left, left)
		st.toks[left].used = true
	}
}

// restore fills missing reference fields from the built-in directory.
func (st *state) restore() {
	if city, ok := st.vals[City]; ok {
		if p, ok := cities[key(city)]; ok {
			st.setDir(Province, p.province)
			st.setDir(District, p.district)
		}
	}
	if _, ok := st.vals[District]; ok || st.vals[City] != "Алматы" {
		return
	}
	for _, f := range []Field{LivingArea, StreetName} {
		v, ok := st.vals[f]
		if !ok || (f == StreetName && st.vals[StreetType] != "микрорайон") {
			continue
		}
		w := strings.Fields(strings.NewReplacer("-", " ").Replace(fold(v)))
		if len(w) == 0 {
			continue
		}
		if d, ok := almatyAreas[w[0]]; ok {
			st.setDir(District, d)
			return
		}
	}
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
