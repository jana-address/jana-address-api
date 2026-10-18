package dict

import (
	_ "embed"
	"strings"
	"sync"
)

// The streets and microdistricts of Караганда and Актобе by city district, from the akim decisions on
// polling precincts (the header of each file gives the act): the map draws no district borders there.
var (
	//go:embed precincts/karaganda.tsv
	karagandaTSV string
	//go:embed precincts/aktobe.tsv
	aktobeTSV string
)

// PrecinctDistricts maps the streets of one city to its districts: by key and type, by key alone when every
// street of that key lies in one district, and by the last word of a longer name ("аманжолова").
type PrecinctDistricts struct {
	typed, any, surname map[string]string // "" marks a key found in both districts
}

// District finds the district of a street or microdistrict key written with the type typ ("" when none).
func (p *PrecinctDistricts) District(key, typ string) (string, bool) {
	if d := p.typed[key+"\x00"+typ]; typ != "" && d != "" {
		return d, true
	}
	if d := p.any[key]; d != "" {
		return d, true
	}
	if d := p.surname[key]; !strings.Contains(key, " ") && d != "" {
		return d, true
	}
	return "", false
}

// Precincts returns the district lists by the city name the parser outputs.
var Precincts = sync.OnceValue(func() map[string]*PrecinctDistricts {
	return map[string]*PrecinctDistricts{"Караганда": loadPrecincts(karagandaTSV), "Актобе": loadPrecincts(aktobeTSV)}
})

func loadPrecincts(tsv string) *PrecinctDistricts {
	p := &PrecinctDistricts{typed: map[string]string{}, any: map[string]string{}, surname: map[string]string{}}
	put := func(m map[string]string, k, d string) {
		if old, ok := m[k]; ok && old != d {
			d = ""
		}
		m[k] = d
	}
	for _, line := range strings.Split(tsv, "\n") {
		name, d, ok := strings.Cut(line, "\t")
		if !ok || strings.HasPrefix(line, "#") {
			continue
		}
		k, typ := StreetKey(name)
		if k == "" {
			continue
		}
		if typ != "" {
			put(p.typed, k+"\x00"+typ, d)
		}
		put(p.any, k, d)
		if i := strings.LastIndexByte(k, ' '); i >= 0 {
			put(p.surname, k[i+1:], d)
		}
	}
	return p
}
