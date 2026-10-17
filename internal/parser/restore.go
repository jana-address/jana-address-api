package parser

import (
	"strings"

	"github.com/jana-address/jana-address-api/internal/parser/dict"
	"github.com/jana-address/jana-address-api/internal/parser/text"
)

// restore fills missing reference fields from the built-in directory.
func (st *state) restore() {
	if city, ok := st.vals[City]; ok {
		if p, ok := st.d.Cities[text.Key(city)]; ok {
			st.setDir(Province, p.Province)
			st.setDir(District, p.District)
		} else if p, ok := st.settlement(text.Key(city)); ok {
			st.setDir(Province, p.Province)
			st.setDir(District, p.District)
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
		w := strings.Fields(strings.NewReplacer("-", " ").Replace(text.Fold(v)))
		if len(w) == 0 {
			continue
		}
		if d, ok := dict.AlmatyAreas[w[0]]; ok {
			st.setDir(District, d)
			return
		}
	}
}
