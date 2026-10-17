package parser

import (
	"strings"

	"github.com/jana-address/jana-address-api/internal/kato"
)

// KATOStats describes what the classifier added to the directory.
type KATOStats struct {
	Settlements int // names of towns and villages, Russian and Kazakh spellings
	Ambiguous   int // names shared by several settlements
	Districts   int // rural rayons added
}

// UseKATO builds a new directory from the base tables plus the classifier and makes it current.
// Records parsed afterwards see the new version; records in flight finish on the old one.
//
// Classifier names are used only where a settlement is certain: after "с.", "п.", "ауылы", "г.",
// or as a separate part next to a written district or region. Thousands of villages are called
// "Абай", "Достык", "Береке", like streets, so a bare word before a house number is never a village.
func UseKATO(recs []kato.Record) KATOStats {
	var st KATOStats
	d := newDirectory(func(d *directory) { st = d.addKATO(recs) })
	current.Store(d)
	return st
}

func (d *directory) addKATO(recs []kato.Record) KATOStats {
	byCode := make(map[string]*kato.Record, len(recs))
	for i := range recs {
		byCode[recs[i].Code] = &recs[i]
	}
	// ancestor walks up to the nearest record of the wanted kind
	ancestor := func(r *kato.Record, want func(*kato.Record) bool) *kato.Record {
		for p := byCode[r.Parent]; p != nil; p = byCode[p.Parent] {
			if want(p) {
				return p
			}
		}
		return nil
	}
	isRegion := func(r *kato.Record) bool { return r.Kind == kato.Region }
	// lower case: also "Район Шал акына"
	isRayon := func(r *kato.Record) bool {
		return r.Kind == kato.District && strings.Contains(strings.ToLower(r.NameRU), "район")
	}
	regionName := func(r *kato.Record) string {
		if r == nil {
			return ""
		}
		if strings.HasPrefix(r.NameRU, "г.") { // Астана, Алматы, Шымкент are their own province
			return r.NameRUClean
		}
		return r.NameRU
	}

	var st KATOStats
	if d.settlements == nil {
		d.settlements = map[string][]place{}
	}
	seen := map[string]map[string]bool{} // key -> code set, to count each settlement once per spelling
	rayonsByName := map[string][]districtInfo{}
	for i := range recs {
		r := &recs[i]
		switch r.Kind {
		case kato.City, kato.Village:
			p := place{name: r.NameRUClean, province: regionName(ancestor(r, isRegion))}
			if ry := ancestor(r, isRayon); ry != nil {
				p.district = ry.NameRUClean
			}
			for _, name := range []string{r.NameRUClean, r.NameKKClean} {
				k := key(name)
				if k == "" {
					continue
				}
				if seen[k] == nil {
					seen[k] = map[string]bool{}
				}
				if !seen[k][r.Code] {
					seen[k][r.Code] = true
					d.settlements[k] = append(d.settlements[k], p)
				}
			}
		case kato.District:
			if !isRayon(r) || r.Code[:2] == "71" || r.Code[:2] == "75" || r.Code[:2] == "79" {
				continue // city districts of Астана, Алматы, Шымкент keep the hand-written names
			}
			info := districtInfo{name: r.NameRUClean, province: regionName(ancestor(r, isRegion))}
			for _, name := range []string{r.NameRUClean, r.NameKKClean} {
				if k := key(name); k != "" {
					rayonsByName[k] = append(rayonsByName[k], info)
				}
			}
		}
	}
	for k, infos := range rayonsByName {
		if _, ok := d.districts[k]; ok {
			continue // hand-written entries win
		}
		info := infos[0]
		for _, o := range infos[1:] {
			if o.province != info.province {
				info.province = "" // the same rayon name in two regions: keep the name, not the region
			}
		}
		d.districts[k] = info
		st.Districts++
	}
	for _, ps := range d.settlements {
		st.Settlements++
		if len(ps) > 1 {
			st.Ambiguous++
		}
	}
	return st
}

// settlement resolves a town or village name; province and district written in the line pick one
// of several namesakes. It returns false when the name is unknown or still ambiguous.
func (st *state) settlement(k string) (place, bool) {
	ps := st.d.settlements[k]
	if len(ps) == 0 {
		return place{}, false
	}
	prov, dist := st.writtenValue(Province), st.writtenValue(District)
	var match []place
	for _, p := range ps {
		if (prov == "" || p.province == prov) && (dist == "" || p.district == dist) {
			match = append(match, p)
		}
	}
	if len(match) == 1 {
		return match[0], true
	}
	return place{}, false
}

// writtenValue returns a field value only when it came from the line itself.
func (st *state) writtenValue(f Field) string {
	if st.src[f] != SourceString {
		return ""
	}
	return st.vals[f]
}
