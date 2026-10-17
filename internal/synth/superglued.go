package synth

import (
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// bannedParts must never appear in the output, compared lowercased without spaces.
var bannedParts = []string{"астанамангликел", "астанамангиликел", "астанамәңгілікел", "мангликел55",
	"мангиликел55", "мәңгілікел55", "астанақалас13мкр", "13мкр55д"}

func banned(raw string) bool {
	k := strings.ToLower(strings.ReplaceAll(raw, " ", ""))
	for _, p := range bannedParts {
		if strings.Contains(k, p) {
			return true
		}
	}
	return false
}

// Words glued into a superglued address. A prefix marker goes before its
// number, a postfix one after it.
var (
	sgCityWords = []form{{"қаласы", 'k'}, {"қаласы", 'k'}, {"қалас", 'k'}, {"қала", 'k'}, {"каласы", 'k'},
		{"калас", 'k'}, {"город", 0}, {"горд", 't'}}
	sgVillageWords = []form{{"ауылы", 'k'}, {"ауыл", 'k'}, {"село", 0}}
	sgMkrPost      = []form{{"мкр", 0}, {"мкр", 0}, {"мкр", 0}, {"мкрн", 0}, {"микрорайон", 0}, {"шағынаудан", 'k'}}
	sgMkrPre       = []form{{"мкр", 0}, {"мкр", 0}, {"мкрн", 0}}
	sgTypePre      = map[string][]form{
		"улица":    {{"ул", 0}, {"ул", 0}, {"ул.", 0}, {"улица", 0}},
		"проспект": {{"пр", 0}, {"пр.", 0}, {"пр-т", 0}, {"проспект", 0}},
		"переулок": {{"пер", 0}, {"пер.", 0}},
		"бульвар":  {{"б-р", 0}, {"бульвар", 0}},
		"шоссе":    {{"шоссе", 0}},
	}
	sgTypePost = map[string][]form{
		"улица":    {{"көшесі", 'k'}, {"кошеси", 'k'}, {"көш", 'k'}},
		"проспект": {{"даңғылы", 'k'}, {"дангылы", 'k'}},
		"переулок": {{"тұйығы", 'k'}},
		"бульвар":  {{"бульвары", 'k'}},
		"шоссе":    {{"тасжолы", 'k'}, {"шоссе", 0}},
	}
	sgHousePre  = []form{{"д", 0}, {"д", 0}, {"д.", 0}, {"дом", 0}, {"үй", 'k'}, {"уй", 'k'}}
	sgHousePost = []form{{"д", 0}, {"д", 0}, {"дом", 0}, {"үй", 'k'}, {"уй", 'k'}}
	sgAptPre    = []form{{"кв", 0}, {"кв", 0}, {"кв", 0}, {"кв.", 0}, {"квартира", 0}, {"пәтер", 'k'}}
	sgAptPost   = []form{{"кв", 0}, {"кв", 0}, {"кв", 0}, {"пәтер", 'k'}}
	sgEntPre    = []form{{"под", 0}, {"под", 0}, {"подъезд", 0}, {"кіреберіс", 'k'}}
	sgEntPost   = []form{{"под", 0}, {"подъезд", 0}, {"кіреберіс", 'k'}}
	sgFloorPre  = []form{{"эт", 0}, {"эт", 0}, {"этаж", 0}, {"қабат", 'k'}}
	sgFloorPost = []form{{"эт", 0}, {"этаж", 0}, {"қабат", 'k'}}
)

const (
	styleBare = iota // "абая12кв5": bare house, markers before the details
	stylePost        // "абая12дом5кв2под": every number followed by its marker
	stylePre         // "абая д12кв5": every number preceded by its marker
)

// sgw picks a glued word and records what kind it is; case is left alone.
func (b *builder) sgw(fs []form) string {
	f := fs[b.r.IntN(len(fs))]
	switch f.k {
	case 'k':
		b.tag("kazakh")
	case 't':
		b.tag("typo")
	}
	if abbrevs[strings.Trim(f.s, ".")] {
		b.tag("abbreviations")
	}
	return f.s
}

// superglued renders an address typed with almost no spaces, like
// "таразтолеби12кв5" or "актау7мкр14д3кв".
func (b *builder) superglued() string {
	b.fast = true
	b.lower = b.p(0.85)
	b.tag("fast", "superglued", "glued", "no-commas")
	if b.lower {
		b.tag("lowercase")
	}
	c := pickCity(b.r)
	numbered := b.p(0.3)
	for i := 0; numbered && len(numberedMkr(c)) == 0 && i < 100; i++ {
		c = pickCity(b.r)
	}
	numbered = numbered && len(numberedMkr(c)) > 0
	if c.Village {
		b.tag("village")
	}
	apt := b.p(0.7)
	ent := apt && b.p(0.43)
	floor := apt && b.p(0.36)

	var sb strings.Builder
	spaces := 0
	space := func(p float64) {
		if spaces < 2 && b.p(p) {
			sb.WriteByte(' ')
			spaces++
		}
	}

	// City, maybe with a glued city word.
	city := c.Name
	switch x := b.r.Float64(); {
	case c.Kaz == "" || x < 0.55:
	case x < 0.75:
		city = c.Kaz
	default:
		city = deKaz(c.Kaz)
	}
	if city != c.Name {
		b.tag("kazakh")
	}
	city = strings.ToLower(strings.ReplaceAll(city, " ", ""))
	if b.p(0.1) {
		if t, ok := b.typo(city); ok {
			city = t
			b.tag("typo")
		}
	}
	sb.WriteString(city)
	cityWord := b.p(0.35)
	if cityWord {
		if c.Village {
			sb.WriteString(b.sgw(sgVillageWords))
		} else {
			sb.WriteString(b.sgw(sgCityWords))
		}
		space(0.4)
	} else {
		space(0.12)
	}
	b.set("city", c.Name, true)
	b.set("province", c.Province, c.big())

	// Numbered microdistrict or street.
	style := styleBare
	spaceBeforeHouse := false
	if numbered {
		b.tag("mkr")
		n := b.pick(numberedMkr(c))
		b.set("street_type", mkrType, true)
		b.set("street_name", n, true)
		if b.p(0.75) {
			sb.WriteString(n + b.sgw(sgMkrPost))
			style = stylePost
			if b.p(0.25) {
				style = stylePre
			}
		} else {
			// "мкр13д55" would read as house 13д, so a space goes before the house.
			sb.WriteString(b.sgw(sgMkrPre) + n)
			style, spaceBeforeHouse = stylePre, true
		}
	} else {
		var s Street
		for range 100 {
			s = c.Streets[b.r.IntN(len(c.Streets))]
			if !needsType(s.Type, s.Name) && !(c.Name == "Астана" && s.Ru == "Мангилик Ел") {
				break
			}
		}
		nm := s.Ru
		if s.Kz != "" && b.p(0.2) {
			nm = s.Kz
			b.tag("kazakh")
			if b.p(0.4) {
				nm = deKaz(nm)
			}
		}
		nm = strings.ToLower(strings.ReplaceAll(nm, " ", ""))
		if b.p(0.3) {
			t, ok := "", false
			if b.p(0.6) {
				t, ok = b.dropVowel(nm)
			}
			if !ok {
				t, ok = b.typo(nm)
			}
			if ok {
				nm = t
				b.tag("typo")
			}
		}
		text := nm
		if b.p(0.25) {
			if b.p(0.6) {
				text = b.sgw(sgTypePre[s.Type]) + nm
			} else {
				text = nm + b.sgw(sgTypePost[s.Type])
			}
			b.set("street_type", s.Type, true)
		}
		b.set("street_name", nm, true)
		sb.WriteString(text)
		switch x := b.r.Float64(); {
		case x < 0.45:
		case x < 0.80:
			style = stylePost
		default:
			style, spaceBeforeHouse = stylePre, true // never "абаяд12"
		}
	}

	// House.
	if spaceBeforeHouse {
		sb.WriteByte(' ')
	}
	n := 1 + b.r.IntN(80)
	if b.p(0.2) {
		n = 1 + b.r.IntN(200)
	}
	label := strconv.Itoa(n)
	switch x := b.r.Float64(); {
	case x < 0.05:
		label += "/" + strconv.Itoa(1+b.r.IntN(20))
		b.tag("fraction")
	case x < 0.11 && !apt && style != stylePost:
		// A letter only at the very end: "12акв5" would be unclear.
		label += b.pick(list("а", "б", "в"))
		b.tag("letter")
	}
	b.set("house", label, true)
	switch style {
	case styleBare:
		sb.WriteString(label)
	case stylePost:
		sb.WriteString(label + b.sgw(sgHousePost))
	case stylePre:
		sb.WriteString(b.sgw(sgHousePre) + label)
	}

	// Details as one chain in the same marker style.
	type det struct {
		field     string
		v         int
		pre, post []form
	}
	var ds []det
	if apt {
		ds = append(ds, det{"apartment", 1 + b.r.IntN(150), sgAptPre, sgAptPost})
	}
	if ent {
		ds = append(ds, det{"entrance", 1 + b.r.IntN(8), sgEntPre, sgEntPost})
	}
	if floor {
		ds = append(ds, det{"floor", 1 + b.r.IntN(12), sgFloorPre, sgFloorPost})
	}
	if len(ds) == 3 && b.p(0.3) {
		ds = []det{ds[1], ds[2], ds[0]}
	}
	for i, d := range ds {
		if i == 0 {
			space(0.15)
		}
		v := strconv.Itoa(d.v)
		b.set(d.field, v, true)
		if style == stylePost {
			sb.WriteString(v + b.sgw(d.post))
		} else {
			sb.WriteString(b.sgw(d.pre) + v)
		}
	}

	raw := sb.String()
	if !b.lower {
		raw = capFirst(raw)
	}
	return raw
}

func numberedMkr(c *City) []string {
	var out []string
	for _, m := range c.Mkr {
		if isNumber(m.Ru) {
			out = append(out, m.Ru)
		}
	}
	return out
}

const vowels = "аеёиоуыэюяәөұүі"

// dropVowel removes one vowel other than the first letter: "ерубаева" -> "ербаева".
func (b *builder) dropVowel(s string) (string, bool) {
	rs := []rune(s)
	if len(rs) < 4 {
		return s, false
	}
	var idx []int
	for i := 1; i < len(rs); i++ {
		if strings.ContainsRune(vowels, unicode.ToLower(rs[i])) {
			idx = append(idx, i)
		}
	}
	if len(idx) == 0 {
		return s, false
	}
	i := idx[b.r.IntN(len(idx))]
	return string(slices.Delete(rs, i, i+1)), true
}
