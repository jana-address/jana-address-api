package dict

import (
	"strings"

	"github.com/jana-address/jana-address-api/internal/kato"
	"github.com/jana-address/jana-address-api/internal/parser/text"
)

// KATOStats describes what the classifier added to the directory.
type KATOStats struct {
	Settlements int // names of towns and villages, Russian and Kazakh spellings
	Ambiguous   int // names shared by several settlements
	Districts   int // rural rayons added
}

// UseKATO builds a new directory from the base tables plus the classifier and makes it current.
// Records parsed afterwards see the new version; records in flight finish on the old one.
func UseKATO(recs []kato.Record) KATOStats {
	var st KATOStats
	d := newDirectory(func(d *Directory) { st = d.addKATO(recs) })
	current.Store(d)
	return st
}

func (d *Directory) addKATO(recs []kato.Record) KATOStats {
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
	if d.Settlements == nil {
		d.Settlements = map[string][]Place{}
	}
	seen := map[string]map[string]bool{} // key -> code set, to count each settlement once per spelling
	rayonsByName := map[string][]DistrictInfo{}
	for i := range recs {
		r := &recs[i]
		switch r.Kind {
		case kato.City, kato.Village:
			p := Place{Name: r.NameRUClean, Province: regionName(ancestor(r, isRegion))}
			if ry := ancestor(r, isRayon); ry != nil {
				p.District = ry.NameRUClean
			}
			for _, name := range []string{r.NameRUClean, r.NameKKClean} {
				k := text.Key(name)
				if k == "" {
					continue
				}
				if seen[k] == nil {
					seen[k] = map[string]bool{}
				}
				if !seen[k][r.Code] {
					seen[k][r.Code] = true
					d.Settlements[k] = append(d.Settlements[k], p)
				}
			}
		case kato.District:
			if !isRayon(r) || r.Code[:2] == "71" || r.Code[:2] == "75" || r.Code[:2] == "79" {
				continue // city districts of Астана, Алматы, Шымкент keep the hand-written names
			}
			info := DistrictInfo{Name: r.NameRUClean, Province: regionName(ancestor(r, isRegion))}
			for _, name := range []string{r.NameRUClean, r.NameKKClean} {
				if k := text.Key(name); k != "" {
					rayonsByName[k] = append(rayonsByName[k], info)
				}
			}
		}
	}
	for k, infos := range rayonsByName {
		if _, ok := d.Districts[k]; ok {
			continue // hand-written entries win
		}
		info := infos[0]
		for _, o := range infos[1:] {
			if o.Province != info.Province {
				info.Province = "" // the same rayon name in two regions: keep the name, not the region
			}
		}
		d.Districts[k] = info
		st.Districts++
	}
	for _, ps := range d.Settlements {
		st.Settlements++
		if len(ps) > 1 {
			st.Ambiguous++
		}
	}
	return st
}
