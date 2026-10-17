package parser

import "sync/atomic"

// directory is one immutable version of the place tables. Parse takes the current version once,
// so a weekly classifier update never mixes two versions inside one record.
type directory struct {
	cities    map[string]place
	regions   map[string]string
	districts map[string]districtInfo
	// settlements from the classifier by name; several entries are namesakes in different regions
	settlements map[string][]place
	cityByLen   [maxFuzzyLen][]cityEntry // city keys bucketed by length in runes, for typo search
}

// current is set at the end of the lexicon init, after the base tables are filled.
var current atomic.Pointer[directory]

// newDirectory copies the hand-written base tables and applies extra entries without overriding them.
func newDirectory(extend func(d *directory)) *directory {
	d := &directory{
		cities:    make(map[string]place, len(baseCities)),
		regions:   make(map[string]string, len(baseRegions)),
		districts: make(map[string]districtInfo, len(baseDistricts)),
	}
	for k, v := range baseCities {
		d.cities[k] = v
	}
	for k, v := range baseRegions {
		d.regions[k] = v
	}
	for k, v := range baseDistricts {
		d.districts[k] = v
	}
	if extend != nil {
		extend(d)
	}
	for k, p := range d.cities {
		if r := []rune(k); len(r) < maxFuzzyLen {
			d.cityByLen[len(r)] = append(d.cityByLen[len(r)], cityEntry{r, p})
		}
	}
	return d
}
