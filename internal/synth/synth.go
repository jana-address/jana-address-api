// Package synth generates labeled Kazakhstan delivery addresses the way people
// type them in a hurry: typos, missing commas, glued words, Kazakh and Latin
// spellings, details in any order. Output is deterministic for a seed.
package synth

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/jana-address/jana-address-api/internal/eval"
)

// Fields are the labeled keys of every record, in output order.
var Fields = []string{"province", "city", "district", "living_area", "street_type", "street_name",
	"house", "apartment", "entrance", "floor", "landmark"}

// StreetTypes are the canonical street_type values.
var StreetTypes = []string{"улица", "проспект", "микрорайон", "переулок", "квартал", "бульвар", "шоссе", "жилой комплекс"}

// Tags is the tag vocabulary in output order.
var Tags = []string{"fast", "careful", "typo", "no-commas", "lowercase", "glued", "camel", "latin", "kazakh",
	"bilingual", "landmark", "mkr", "zhk", "living-area", "district", "oblast", "no-city", "dup-city",
	"old-name", "fraction", "letter", "postcode", "country", "extra-spaces", "abbreviations",
	"reversed-order", "block", "office", "village", "superglued"}

const (
	mkrType = "микрорайон"
	zhkType = "жилой комплекс"
)

type mode int

const (
	modeRu mode = iota
	modeKz
	modeLat
)

type kind int

const (
	kindStreet kind = iota
	kindMkr
	kindZhK
	kindArea // microdistrict or ЖК plus a street
)

// Generate returns n labeled addresses with ids syn_00001.., deterministic for a seed.
func Generate(n int, seed uint64) []eval.Gold {
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	out := make([]eval.Gold, 0, n)
	for i := range n {
		var b *builder
		var raw string
		// Redraw the rare record that happens to contain a reserved string.
		for b == nil || banned(raw) {
			b = &builder{r: r, tags: map[string]bool{}, exp: map[string]*string{}, in: map[string]bool{}}
			raw = b.record()
		}
		g := eval.Gold{ID: fmt.Sprintf("syn_%05d", i+1), Raw: raw, Expected: map[string]*string{},
			InString: []string{}, Tags: []string{}}
		for _, f := range Fields {
			g.Expected[f] = b.exp[f]
			if b.in[f] && b.exp[f] != nil {
				g.InString = append(g.InString, f)
			}
		}
		for _, t := range Tags {
			if b.tags[t] {
				g.Tags = append(g.Tags, t)
			}
		}
		out = append(out, g)
	}
	return out
}

type builder struct {
	r     *rand.Rand
	fast  bool
	lower bool
	camel bool
	mode  mode
	tags  map[string]bool
	exp   map[string]*string
	in    map[string]bool
}

// chunk is one comma-separable part of the address.
type chunk struct {
	text   string
	kind   byte // 's' street, 'h' house, 'd' detail, 'c' city, 'o' other
	glue   bool // no separator before the next chunk
	spaced bool // ends with a spaced house letter ("12 а"), so a following number needs a comma
}

func (b *builder) p(x float64) bool { return b.r.Float64() < x }

// pf is p with separate odds for the fast and careful profiles.
func (b *builder) pf(fast, careful float64) bool {
	if b.fast {
		return b.p(fast)
	}
	return b.p(careful)
}

func (b *builder) pick(xs []string) string { return xs[b.r.IntN(len(xs))] }

// areaZhK leaves out the ЖК names that end in a street type word for a ЖК written before its street:
// "жк экспо бульвар валиханова" reads as ЖК Экспо Бульвар on Валиханова and as ЖК Экспо on бульвар Валиханова.
func areaZhK(names []string) []string {
	var out []string
	for _, n := range names {
		if !strings.HasSuffix(strings.ToLower(n), " бульвар") {
			out = append(out, n)
		}
	}
	return out
}

func (b *builder) tag(ts ...string) {
	for _, t := range ts {
		b.tags[t] = true
	}
}

func (b *builder) set(field, v string, written bool) {
	b.exp[field] = &v
	if written {
		b.in[field] = true
	}
}

// cs applies the record's capitalization habits to one token.
func (b *builder) cs(s string) string {
	if b.lower {
		return strings.ToLower(s)
	}
	if !b.fast {
		return s
	}
	switch x := b.r.Float64(); {
	case x < 0.22:
		return strings.ToLower(s)
	case x < 0.24:
		return strings.ToUpper(s)
	case x < 0.30:
		return capFirst(s)
	}
	return s
}

// word renders a marker or type word and records what kind of form it is.
func (b *builder) word(s string, k byte) string {
	switch k {
	case 'k':
		b.tag("kazakh")
	case 'l':
		b.tag("latin")
	case 't':
		b.tag("typo")
	}
	if abbrevs[strings.Trim(strings.ToLower(s), " .")] {
		b.tag("abbreviations")
	}
	return b.cs(s)
}

func (b *builder) record() string {
	if b.p(0.05) {
		return b.superglued()
	}
	b.fast = b.p(0.8)
	if b.fast {
		b.tag("fast")
		b.lower = b.p(0.5)
	} else {
		b.tag("careful")
	}
	switch x := b.r.Float64(); {
	case x < 0.05 && b.fast, x < 0.015:
		b.mode = modeLat
	case x < 0.11:
		b.mode = modeKz
	}
	if b.lower {
		b.tag("lowercase")
	}

	c := pickCity(b.r)
	if c.Village {
		b.tag("village")
	}
	noCity := b.p(0.07)
	withOblast := !c.big() && b.mode != modeLat && b.p(0.15)
	withDistrict := false
	if !noCity && len(c.Districts) > 0 && b.mode != modeLat {
		pd := 0.15
		if !c.big() {
			pd = 0.25
		}
		withDistrict = b.p(pd)
	}

	k := b.pickKind(c)

	// Details first, so the layout can depend on them.
	office := !c.Village && b.p(0.012)
	aptP := 0.72
	switch {
	case c.Village:
		aptP = 0.2
	case k == kindZhK:
		aptP = 0.92
	}
	apt := !office && b.p(aptP)
	ent := apt && b.p(0.36)
	floor := apt && b.p(0.36) || office && b.p(0.5)
	block := b.p(0.025)
	withLandmark := b.mode != modeLat && b.p(0.08)
	nDetails := btoi(apt || office) + btoi(ent) + btoi(floor)

	cityEnd := !noCity && b.pf(0.06, 0.03)
	detailsFirst := !cityEnd && nDetails > 0 && b.p(0.04)
	plainCity := !noCity && !cityEnd && !detailsFirst && !withDistrict && k == kindStreet && b.mode != modeLat
	b.camel = plainCity && b.fast && !b.lower && b.mode == modeRu && b.p(0.14)
	glueCity := plainCity && !b.camel && b.fast && b.p(0.06)
	dup := !noCity && !b.camel && !glueCity && b.fast && b.p(0.035)
	if cityEnd || detailsFirst {
		b.tag("reversed-order")
	}

	var pre []chunk
	if b.pf(0.03, 0.09) {
		b.tag("postcode")
		pre = append(pre, chunk{text: fmt.Sprintf("%s%02d", c.Post, b.r.IntN(100)), kind: 'o'})
	}
	if b.pf(0.03, 0.08) {
		b.tag("country")
		f := countries[b.r.IntN(len(countries))]
		if b.mode == modeLat {
			f = form{"Kazakhstan", 'l'}
		}
		ch := chunk{text: b.word(f.s, f.k), kind: 'o'}
		if len(pre) > 0 && b.p(0.5) {
			pre = append([]chunk{ch}, pre...)
		} else {
			pre = append(pre, ch)
		}
	}

	var oblast, city, district, area, street, house, blk, landmark []chunk
	province := c.Province
	if withOblast {
		b.tag("oblast")
		var txt string
		txt, province = b.oblast(c.Province)
		oblast = []chunk{{text: txt, kind: 'o'}}
		b.set("province", province, true)
	}
	if noCity {
		b.tag("no-city")
	} else {
		var t string
		switch {
		case b.camel:
			t = c.Name
		case glueCity:
			t = b.cs(c.Name)
			if b.p(0.12) {
				if s, ok := b.typoInner(t); ok { // the city is glued to the street, see typoInner
					t = s
					b.tag("typo")
				}
			}
		default:
			t = b.cityText(c)
		}
		if dup {
			b.tag("dup-city")
			t += " " + b.cs(c.Name)
		}
		city = []chunk{{text: t, kind: 'c', glue: b.camel || glueCity}}
		if b.camel || glueCity {
			b.tag("glued")
		}
		if b.camel {
			b.tag("camel")
		}
		b.set("city", c.Name, true)
		b.set("province", province, c.big() || withOblast)
	}
	if withDistrict {
		b.tag("district")
		d := c.Districts[b.r.IntN(len(c.Districts))]
		txt, written := b.district(d)
		district = []chunk{{text: txt, kind: 'o'}}
		b.set("district", written, true)
	}

	// Street-like element.
	if k == kindArea {
		b.tag("living-area")
		var typ string
		var n Name
		if len(c.ZhK) > 0 && (len(c.Mkr) == 0 || b.p(0.35)) {
			typ, n = zhkType, Name{Ru: b.pick(areaZhK(c.ZhK))}
			b.tag("zhk")
		} else {
			typ, n = mkrType, c.Mkr[b.r.IntN(len(c.Mkr))]
			b.tag("mkr")
		}
		t, w, _ := b.typed(typ, n, 0)
		area = []chunk{{text: t, kind: 'o'}}
		b.set("living_area", w, true)
	}
	var st string
	var sn Name
	omit := 0.30
	if !b.fast {
		omit = 0.08
	}
	switch k {
	case kindMkr:
		b.tag("mkr")
		st, sn = mkrType, c.Mkr[b.r.IntN(len(c.Mkr))]
		omit /= 2
	case kindZhK:
		b.tag("zhk")
		st, sn, omit = zhkType, Name{Ru: b.pick(c.ZhK)}, 0
	default:
		s := c.Streets[b.r.IntN(len(c.Streets))]
		// A glued street must be able to drop its type word.
		for i := 0; (b.camel || glueCity) && needsType(s.Type, s.Name) && i < 20; i++ {
			s = c.Streets[b.r.IntN(len(c.Streets))]
		}
		st, sn = s.Type, s.Name
	}
	if b.camel || glueCity {
		omit = 1
	}
	if detailsFirst {
		omit = 0
	}
	stText, stName, shown := b.typed(st, sn, omit)
	if shown {
		b.set("street_type", st, true)
	}
	b.set("street_name", stName, true)

	// House: bare when glued to the street, with a marker when it comes before the street.
	houseMode := 0
	switch {
	case detailsFirst:
		houseMode = 1
	case b.camel:
		houseMode = 2
	case b.fast && endsLetter(stText) && strings.HasSuffix(stText, stName) && b.mode != modeLat && b.p(0.05):
		houseMode = 2
		b.tag("glued")
	}
	hText, hLabel, spaced := b.house(houseMode)
	street = []chunk{{text: stText, kind: 's', glue: houseMode == 2}}
	house = []chunk{{text: hText, kind: 'h', spaced: spaced}}
	b.set("house", hLabel, true)

	if block {
		b.tag("block")
		blk = []chunk{{text: b.mark(strconv.Itoa(1+b.r.IntN(6)), blockM, true), kind: 'd'}}
	}

	var details []chunk
	if office {
		b.tag("office")
		details = append(details, chunk{text: b.mark(strconv.Itoa(1+b.r.IntN(400)), officeM, true), kind: 'd'})
	}
	if apt {
		v := 1 + b.r.IntN(120)
		if b.p(0.15) {
			v = 1 + b.r.IntN(400)
		}
		details = append(details, b.detail("apartment", v, aptM))
	}
	if ent {
		v := 1 + b.r.IntN(6)
		if b.p(0.1) {
			v = 1 + b.r.IntN(12)
		}
		details = append(details, b.detail("entrance", v, entM))
	}
	if floor {
		v := 1 + b.r.IntN(9)
		if b.p(0.25) {
			v = 1 + b.r.IntN(20)
		}
		details = append(details, b.detail("floor", v, floorM))
	}
	if len(details) == 3 && b.p(0.3) {
		details = []chunk{details[1], details[2], details[0]} // подъезд, этаж, кв
	}
	if len(details) >= 2 && b.pf(0.25, 0.1) {
		orig := slices.Clone(details)
		for slices.Equal(details, orig) {
			b.r.Shuffle(len(details), func(i, j int) { details[i], details[j] = details[j], details[i] })
		}
		b.tag("reversed-order")
	}

	if withLandmark {
		b.tag("landmark")
		lm := b.landmark()
		b.set("landmark", lm, true)
		if b.p(0.3) {
			lm = "(" + lm + ")"
		}
		landmark = []chunk{{text: lm, kind: 'o'}}
	}

	var parts []chunk
	add := func(cs ...[]chunk) {
		for _, c := range cs {
			parts = append(parts, c...)
		}
	}
	switch {
	case cityEnd:
		add(pre, area, street, house, blk, details, district, city, oblast, landmark)
	case detailsFirst:
		add(pre, oblast, city, district, details, house, blk, street, area, landmark)
	default:
		add(pre, oblast, city, district, area, street, house, blk, details, landmark)
	}
	raw := b.join(parts)
	if b.fast && !strings.Contains(raw, ",") {
		b.tag("no-commas")
	}
	return raw
}

func btoi(v bool) int {
	if v {
		return 1
	}
	return 0
}

func (b *builder) pickKind(c *City) kind {
	x := b.r.Float64()
	k := kindStreet
	// Only the big cities have a ЖК list, so they get a larger ЖК share.
	mkrTo, zhkTo := 0.88, 0.92
	if len(c.ZhK) > 0 {
		mkrTo, zhkTo = 0.86, 0.93
	}
	switch {
	case c.MkrShare > 0 && x < c.MkrShare:
		k = kindMkr
	case x < 0.70:
	case x < mkrTo:
		k = kindMkr
	case x < zhkTo:
		k = kindZhK
	default:
		k = kindArea
	}
	if k == kindZhK && len(c.ZhK) == 0 {
		k = kindMkr
	}
	if k == kindMkr && len(c.Mkr) == 0 {
		k = kindStreet
	}
	if k == kindArea && len(c.Mkr) == 0 && len(c.ZhK) == 0 {
		k = kindStreet
	}
	return k
}

func (b *builder) detail(field string, v int, m markers) chunk {
	s := strconv.Itoa(v)
	b.set(field, s, true)
	return chunk{text: b.mark(s, m, true), kind: 'd'}
}

// join puts the chunks together with the separators of the profile.
func (b *builder) join(parts []chunk) string {
	noComma := b.fast && b.p(0.7)
	extra := b.fast && b.p(0.06)
	extraDone := false
	var sb strings.Builder
	for i, ch := range parts {
		if i > 0 {
			prev := parts[i-1]
			if !prev.glue {
				sep := b.sep(noComma, prev, ch)
				// Never leave two bare numbers side by side, and never write "85,2".
				if (endsDigit(prev.text) || prev.spaced) && startsDigit(ch.text) && !strings.Contains(sep, ", ") {
					sep = ", "
				}
				if extra && (b.p(0.3) || !extraDone && i == len(parts)-1) {
					if sep == "," {
						sep = ",  "
					} else {
						sep = strings.Replace(sep, " ", "  ", 1)
					}
					extraDone = true
				}
				sb.WriteString(sep)
			}
		}
		sb.WriteString(ch.text)
	}
	if extraDone {
		b.tag("extra-spaces")
	}
	return sb.String()
}

func (b *builder) sep(noComma bool, prev, next chunk) string {
	if !b.fast {
		if prev.kind == 's' && next.kind == 'h' && startsDigit(next.text) && b.p(0.5) {
			return " "
		}
		return ", "
	}
	if noComma {
		return " "
	}
	switch x := b.r.Float64(); {
	case x < 0.45:
		return ", "
	case x < 0.65:
		return ","
	case x < 0.68:
		return " , "
	}
	return " "
}

// oblast writes a region and returns the text and the expected value: a district or a new region is
// expected as written in Russian ("Абайская область", "область Жетысу"), a Kazakh form gives the official name.
func (b *builder) oblast(prov string) (string, string) {
	forms := Provinces[prov]
	s := b.pick(forms)
	if b.mode == modeKz {
		for _, f := range forms {
			if strings.Contains(f, "облысы") {
				s = f
			}
		}
	}
	if strings.Contains(s, "облысы") {
		b.tag("kazakh")
		if b.fast && b.p(0.3) {
			s = deKaz(s)
		}
	}
	if !strings.Contains(s, "область") && !strings.Contains(s, "облысы") {
		b.tag("abbreviations")
	}
	want := prov
	if strings.HasPrefix(prov, "область ") && !strings.Contains(s, "облысы") {
		w := strings.Fields(s)
		switch {
		case strings.HasSuffix(w[0], "ская"):
			want = w[0] + " область"
		case w[0] == "область" || w[0] == "обл.":
			want = "область " + strings.Join(w[1:], " ")
		}
	}
	return b.cs(s), want
}

func (b *builder) cityText(c *City) string {
	if c.Village {
		return b.villageText(c)
	}
	switch {
	case b.mode == modeLat:
		return b.latinCity(c)
	case b.mode == modeKz && b.p(0.9):
		return b.kazCity(c, " қаласы", " қ.")
	}
	x := b.r.Float64()
	if !b.fast {
		switch {
		case x < 0.55:
			return b.prefixed(list("г. ", "г. ", "г.", "город "), c.Name)
		case x < 0.85:
			return c.Name
		case x < 0.91:
			return b.kazCity(c, " қаласы", " қ.")
		case x < 0.96:
			return b.bilingual(c)
		case len(c.Old) > 0:
			b.tag("old-name")
			return b.prefixed(list("г. ", ""), b.pick(c.Old))
		}
		return c.Name
	}
	switch {
	case x < 0.28:
		return b.cs(c.Name)
	case x < 0.38:
		if s, ok := b.typo(c.Name); ok {
			b.tag("typo")
			return b.cs(s)
		}
		return b.cs(c.Name)
	case x < 0.60:
		name := c.Name
		if b.p(0.08) {
			if s, ok := b.typo(name); ok {
				b.tag("typo")
				name = s
			}
		}
		if b.p(0.06) {
			b.tag("typo")
			return b.word("горд ", 't') + b.cs(name)
		}
		return b.prefixed(list("г ", "г ", "г.", "г. ", "г. ", "город ", "гор ", "гор. "), name)
	case x < 0.69:
		return b.kazCity(c, " қаласы", " қ.")
	case x < 0.72:
		return b.bilingual(c)
	case x < 0.78:
		return b.latinCity(c)
	case x < 0.85 && len(c.Old) > 0:
		b.tag("old-name")
		if b.p(0.3) {
			return b.prefixed(list("г ", "г. ", "г."), b.pick(c.Old))
		}
		return b.cs(b.pick(c.Old))
	}
	return b.cs(c.Name)
}

func (b *builder) prefixed(prefixes []string, name string) string {
	return b.word(b.pick(prefixes), 0) + b.cs(name)
}

func (b *builder) kazCity(c *City, suffixes ...string) string {
	b.tag("kazakh")
	k := c.Kaz
	if k == "" {
		k = c.Name
	}
	var s string
	switch x := b.r.Float64(); {
	case x < 0.5 || c.Kaz == "" && x < 0.85:
		s = k + suffixes[0]
	case x < 0.65 || c.Kaz == "":
		s = k + suffixes[1]
		b.tag("abbreviations")
	default:
		s = k
	}
	if b.fast && b.p(0.3) {
		s = deKaz(s)
	}
	return b.cs(s)
}

func (b *builder) bilingual(c *City) string {
	b.tag("bilingual", "kazakh")
	k := c.Kaz
	if k == "" {
		k = c.Name
	}
	n := 3
	if c.Kaz == "" {
		n = 2
	}
	switch b.r.IntN(n) {
	case 0:
		return b.prefixed(list("город ", "г. "), c.Name) + "/" + b.cs(k+" қаласы")
	case 1:
		b.tag("abbreviations")
		return b.cs("г. "+c.Name) + " (" + b.cs(k+" қ.") + ")"
	}
	return b.cs(c.Name) + " / " + b.cs(k)
}

func (b *builder) latinCity(c *City) string {
	b.tag("latin")
	s := c.Latin[0]
	if len(c.Latin) > 1 && b.p(0.3) {
		s = b.pick(c.Latin[1:])
	}
	if strings.HasPrefix(s, "~") {
		b.tag("old-name")
		s = s[1:]
	}
	return b.cs(s)
}

func (b *builder) villageText(c *City) string {
	switch {
	case b.mode == modeLat:
		return b.latinCity(c)
	case b.mode == modeKz || b.p(0.1):
		return b.kazCity(c, " ауылы", " а.")
	case len(c.Old) > 0 && b.p(0.08):
		b.tag("old-name")
		return b.prefixed(list("с. ", "пос. ", ""), b.pick(c.Old))
	}
	name := c.Name
	if b.fast && b.p(0.08) {
		if s, ok := b.typo(name); ok {
			b.tag("typo")
			name = s
		}
	}
	if !b.fast {
		return b.prefixed(list("с. ", "село ", "п. ", "пос. "), name)
	}
	return b.prefixed(list("с. ", "с.", "с ", "село ", "пос. ", "п. ", "поселок ", "аул ", "", "", ""), name)
}

// district writes a district and returns the text and the name as written, without "район".
func (b *builder) district(d District) (string, string) {
	if b.mode == modeKz || b.p(0.18) {
		b.tag("kazakh")
		s, name := d.Kz+" ауданы", d.Kz
		if b.p(0.2) {
			s = d.Kz + " ауд."
			b.tag("abbreviations")
		}
		if b.fast && b.p(0.3) {
			s, name = deKaz(s), deKaz(name)
		}
		return b.cs(s), name
	}
	n := d.Name
	var forms []string
	switch {
	case !b.fast:
		forms = list(n+" район", n+" р-н")
	case strings.HasSuffix(n, "ий"):
		forms = list(n+" р-н", n+" р-н", n+" р-н", n+" район", n+" район", "р-н "+n, "район "+n)
	default:
		forms = list("р-н "+n, "район "+n, n+" р-н", n+" район")
	}
	s := b.pick(forms)
	if strings.Contains(s, "р-н") {
		b.tag("abbreviations")
	}
	return b.cs(s), n
}

// typed renders a name with an optional type word. It returns the text, the
// name exactly as written and whether a type word was written.
func (b *builder) typed(typ string, n Name, omit float64) (string, string, bool) {
	nm := n.Ru
	kazName := false
	switch {
	case b.mode == modeLat:
		if !isLatin(nm) {
			nm = translit(nm)
		}
		b.tag("latin")
	case n.Kz != "" && !b.camel && (b.mode == modeKz && b.p(0.9) || b.fast && b.p(0.12) || !b.fast && b.p(0.06)):
		nm, kazName = n.Kz, true
		b.tag("kazakh")
		if b.fast && b.p(0.25) {
			nm = deKaz(nm)
		}
	}
	if b.fast && b.p(0.10) || !b.fast && b.p(0.02) {
		if s, ok := b.typo(nm); ok {
			nm = s
			b.tag("typo")
		}
	}
	if b.camel {
		nm = capFirst(nm)
	} else {
		nm = b.cs(nm)
	}
	written, shown := nm, nm
	if typ == zhkType && b.mode != modeLat && b.p(0.25) {
		if b.p(0.6) {
			shown = `"` + nm + `"`
		} else {
			shown = "«" + nm + "»"
		}
	}

	if !needsType(typ, n) && b.p(omit) {
		return shown, written, false
	}

	tf := typeWords[typ]
	postShare := 0.08
	switch {
	case typ == "шоссе":
		postShare = 0.7
	case typ == "переулок":
		postShare = 0.5
	case isNumber(n.Ru):
		postShare = 0.4
	}
	var pre, post string
	var k byte
	switch {
	case b.mode == modeLat:
		pre, k = b.pick(tf.latPre), 'l'
	case (kazName || b.mode == modeKz) && b.p(0.7):
		post, k = b.pick(tf.kzPost), 'k'
	case !b.fast:
		if b.p(postShare) {
			post = b.pick(tf.post)
		} else {
			pre = b.pick(tf.careful)
		}
	case b.p(0.05):
		pre, k = b.pick(tf.typoPre), 't'
	case b.p(postShare):
		post = b.pick(tf.post)
	default:
		pre = b.pick(tf.pre)
	}
	if pre != "" {
		return b.word(pre, k) + shown, written, true
	}
	return shown + b.word(post, k), written, true
}

// needsType reports names that are unreadable without a type word: numbers
// and names of a settlement.
func needsType(typ string, n Name) bool {
	return isNumber(n.Ru) || typ != mkrType && strings.ContainsAny(n.Ru, "0123456789") ||
		cityWords[strings.ToLower(n.Ru)] || cityWords[strings.ToLower(n.Kz)]
}

// house renders a house number. markerMode: 0 any, 1 prefix marker, 2 bare.
// It returns the text, the compact label and whether it ends with a spaced letter.
func (b *builder) house(markerMode int) (string, string, bool) {
	n := 1 + b.r.IntN(60)
	switch x := b.r.Float64(); {
	case x < 0.3:
		n = 1 + b.r.IntN(150)
	case x < 0.4:
		n = 1 + b.r.IntN(400)
	}
	num := strconv.Itoa(n)
	written, label, spaced := num, num, false
	switch x := b.r.Float64(); {
	case x < 0.08:
		b.tag("letter")
		// no "Д": "12д" would read as the glued marker "12 дом"
		letters := list("а", "а", "б", "б", "в", "г", "А", "Б")
		if b.mode == modeLat {
			letters = list("a", "b", "v", "A", "B")
		}
		l := b.pick(letters)
		label = num + l
		written = label
		// only а and б stand apart: "18 г туркестан" reads as г. Туркестан, "17 в ақын сара" as the preposition
		if b.p(0.3) && strings.ContainsAny(l, "аАбБabAB") {
			written, spaced = num+" "+l, true
		}
	case x < 0.15:
		b.tag("fraction")
		m := strconv.Itoa(1 + b.r.IntN(20))
		label = num + "/" + m
		written = label
		switch y := b.r.Float64(); {
		case y < 0.15:
			written = num + " / " + m
		case y < 0.25:
			written = num + "/ " + m
		}
	}
	bare := markerMode == 2 || markerMode == 0 && b.pf(0.55, 0.45)
	if bare {
		return written, label, spaced
	}
	// A postfix marker would read oddly after "12 а"; keep the letter attached then.
	allowPost := markerMode == 0
	if allowPost && spaced {
		written, spaced = label, false
	}
	return b.mark(written, houseM, allowPost), label, false
}

// mark attaches a marker word ("кв", "подъезд", "д") to a number.
func (b *builder) mark(num string, m markers, allowPost bool) string {
	var forms []string
	var k byte
	post := false
	switch {
	case b.mode == modeLat:
		forms, k = m.latPre, 'l'
		if allowPost && len(m.latPost) > 0 && b.fast && b.p(0.15) {
			forms, post = m.latPost, true
		}
	case !b.fast:
		forms = m.careful
	case b.mode == modeKz && b.p(0.8) || b.p(0.04):
		forms, k = m.kzPre, 'k'
		if allowPost && len(m.kzPost) > 0 && b.p(0.4) {
			forms, post = m.kzPost, true
		}
	case b.p(0.04):
		forms, k = m.typoPre, 't'
		if allowPost && len(m.typoPost) > 0 && b.p(0.3) {
			forms, post = m.typoPost, true
		}
	default:
		forms = m.pre
		if allowPost && len(m.post) > 0 && b.p(0.2) {
			forms, post = m.post, true
		}
	}
	f := b.pick(forms)
	w := b.word(f, k)
	if post {
		if !strings.HasPrefix(f, " ") {
			if !endsDigit(num) {
				return num + " " + w // "7гдом" is unreadable
			}
			b.tag("glued")
		}
		return num + w
	}
	if !strings.HasSuffix(f, " ") && !strings.HasSuffix(f, ".") {
		b.tag("glued")
	}
	return w + num
}

func (b *builder) landmark() string {
	var s string
	if b.mode == modeKz || b.p(0.25) {
		s = b.pick(landmarksKz)
		b.tag("kazakh")
	} else {
		s = b.pick(landmarksRu)
	}
	if strings.Contains(s, "%d") {
		s = strings.Replace(s, "%d", strconv.Itoa(1+b.r.IntN(80)), 1)
	}
	return b.cs(s)
}

// typo makes one keyboard slip: drop, swap, neighbour key or double letter. A name of four letters
// keeps all of them: "асу" or "акс" is not readable as Аксу.
func (b *builder) typo(s string) (string, bool) { return b.slip(s, false) }

// typoInner is typo away from the first and the last letter, for a word glued to the next one: a slip
// at the border of "алматыфтолеби" cannot be told to belong to the city or to the street.
func (b *builder) typoInner(s string) (string, bool) { return b.slip(s, true) }

func (b *builder) slip(s string, inner bool) (string, bool) {
	rs := []rune(s)
	var idx []int
	for i, c := range rs {
		if unicode.IsLetter(c) {
			idx = append(idx, i)
		}
	}
	if len(idx) < 3 {
		return s, false
	}
	last := idx[len(idx)-1]
	if inner {
		idx = idx[1 : len(idx)-1]
	}
	for range 8 {
		i := idx[b.r.IntN(len(idx))]
		out := slices.Clone(rs)
		switch b.r.IntN(4) {
		case 0:
			if i == 0 || len(rs) <= 4 {
				continue
			}
			out = slices.Delete(out, i, i+1)
		case 1:
			if i+1 >= len(rs) || !unicode.IsLetter(rs[i+1]) || rs[i] == rs[i+1] || inner && i+1 == last {
				continue
			}
			out[i], out[i+1] = out[i+1], out[i]
		case 2:
			nb := b.neighbour(rs[i])
			if nb == 0 {
				continue
			}
			out[i] = nb
		case 3:
			nb := b.neighbour(rs[i])
			if nb == 0 || b.p(0.5) {
				nb = unicode.ToLower(rs[i])
			}
			out = slices.Insert(out, i+1, nb)
		}
		if t := string(out); t != s {
			return t, true
		}
	}
	return s, false
}

var keyRows = []string{"йцукенгшщзхъ", "фывапролджэ", "ячсмитьбю", "qwertyuiop", "asdfghjkl", "zxcvbnm"}

func (b *builder) neighbour(c rune) rune {
	lc := unicode.ToLower(c)
	for _, row := range keyRows {
		rs := []rune(row)
		i := slices.Index(rs, lc)
		if i < 0 {
			continue
		}
		var opts []rune
		if i > 0 {
			opts = append(opts, rs[i-1])
		}
		if i+1 < len(rs) {
			opts = append(opts, rs[i+1])
		}
		nb := opts[b.r.IntN(len(opts))]
		if unicode.IsUpper(c) {
			nb = unicode.ToUpper(nb)
		}
		return nb
	}
	return 0
}

// cumulative city weights for pickCity
var cityCum = func() []float64 {
	out := make([]float64, len(cities))
	t := 0.0
	for i, c := range cities {
		t += c.Weight
		out[i] = t
	}
	return out
}()

func pickCity(r *rand.Rand) *City {
	x := r.Float64() * cityCum[len(cityCum)-1]
	i := sort.SearchFloat64s(cityCum, x)
	if i >= len(cities) {
		i = len(cities) - 1
	}
	return &cities[i]
}

// cityWords are names that read as a settlement.
var cityWords = func() map[string]bool {
	m := map[string]bool{}
	for _, c := range cities {
		for _, s := range append([]string{c.Name, c.Kaz}, c.Old...) {
			if s != "" {
				m[strings.ToLower(s)] = true
			}
		}
	}
	return m
}()

func capFirst(s string) string {
	rs := []rune(s)
	for i, c := range rs {
		if unicode.IsLetter(c) {
			rs[i] = unicode.ToUpper(c)
			break
		}
	}
	return string(rs)
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isLatin(s string) bool {
	for _, c := range s {
		if unicode.Is(unicode.Cyrillic, c) {
			return false
		}
	}
	return true
}

func endsDigit(s string) bool {
	rs := []rune(s)
	return len(rs) > 0 && unicode.IsDigit(rs[len(rs)-1])
}

func startsDigit(s string) bool {
	for _, c := range s {
		return unicode.IsDigit(c)
	}
	return false
}

func endsLetter(s string) bool {
	rs := []rune(s)
	return len(rs) > 0 && unicode.IsLetter(rs[len(rs)-1])
}

var deKazMap = map[rune]rune{'ә': 'а', 'ғ': 'г', 'қ': 'к', 'ң': 'н', 'ө': 'о', 'ұ': 'у', 'ү': 'у', 'һ': 'х', 'і': 'и',
	'Ә': 'А', 'Ғ': 'Г', 'Қ': 'К', 'Ң': 'Н', 'Ө': 'О', 'Ұ': 'У', 'Ү': 'У', 'Һ': 'Х', 'І': 'И'}

// deKaz spells Kazakh with Russian letters, as on a phone without a Kazakh layout.
func deKaz(s string) string {
	return strings.Map(func(c rune) rune {
		if r, ok := deKazMap[c]; ok {
			return r
		}
		return c
	}, s)
}

var translitMap = map[rune]string{'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e", 'ж': "zh",
	'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s",
	'т': "t", 'у': "u", 'ф': "f", 'х': "kh", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "sch", 'ъ': "", 'ы': "y",
	'ь': "", 'э': "e", 'ю': "yu", 'я': "ya", 'ә': "a", 'ғ': "g", 'қ': "k", 'ң': "n", 'ө': "o", 'ұ': "u",
	'ү': "u", 'һ': "h", 'і': "i"}

// translit writes a Cyrillic name in Latin letters the common informal way.
func translit(s string) string {
	var sb strings.Builder
	for _, c := range s {
		t, ok := translitMap[unicode.ToLower(c)]
		if !ok {
			sb.WriteRune(c)
			continue
		}
		if unicode.IsUpper(c) && t != "" {
			t = strings.ToUpper(t[:1]) + t[1:]
		}
		sb.WriteString(t)
	}
	return sb.String()
}
