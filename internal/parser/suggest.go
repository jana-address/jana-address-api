package parser

import (
	"strings"

	"github.com/jana-address/jana-address-api/internal/parser/dict"
)

// Suggestion is the address written back in a clean form for the operator:
// "Астана, проспект Мәңгілік Ел, д. 55, кв. 12".
// The street name in the output fields stays verbatim; only the suggestion
// uses the directory spelling of a well-known street.

func title(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// Suggest renders the parsed fields as a clean address line.
func (o Output) Suggest() string {
	v := o.Values
	var parts []string
	if p := v[Province]; p != "" && p != v[City] {
		parts = append(parts, p)
	}
	if c := v[City]; c != "" {
		parts = append(parts, c)
	}
	if d := v[District]; d != "" && o.Sources[District] == SourceString {
		if strings.HasSuffix(d, "ий") {
			parts = append(parts, d+" район")
		} else {
			parts = append(parts, "район "+d)
		}
	}
	if la := v[LivingArea]; la != "" {
		parts = append(parts, la)
	}
	if name := v[StreetName]; name != "" {
		typ := v[StreetType]
		if s, ok := dict.MatchStreet(v[City], name); ok {
			name = s.Name
			if typ == "" {
				typ = s.Type
			}
		} else {
			name = title(name)
		}
		if typ != "" {
			name = typ + " " + name
		}
		parts = append(parts, name)
	}
	for _, f := range []struct {
		f      Field
		prefix string
	}{{House, "д. "}, {Apartment, "кв. "}, {Entrance, "подъезд "}, {Floor, "этаж "}} {
		if x := v[f.f]; x != "" {
			parts = append(parts, f.prefix+x)
		}
	}
	if l := v[Landmark]; l != "" {
		parts = append(parts, "("+l+")")
	}
	return strings.Join(parts, ", ")
}
