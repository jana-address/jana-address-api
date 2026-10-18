package dict

import (
	"sync/atomic"

	"github.com/jana-address/jana-address-api/internal/parser/text"
)

// Directory is one immutable version of the place tables. Parse takes the current version once,
// so a weekly classifier update never mixes two versions inside one record. The maps are only read
// once the directory is built.
type Directory struct {
	Cities    map[string]Place
	Regions   map[string]string
	Districts map[string]DistrictInfo
	// CityDistricts: districts of each city by name, so "район Алматы" of Актобе is not the one of Астана
	CityDistricts map[string]map[string]DistrictInfo
	// Settlements from the classifier by name; several entries are namesakes in different regions
	Settlements map[string][]Place
	// Areas: living areas by name; one name may be an area of several cities
	Areas     map[string][]Area
	cityByLen [text.MaxFuzzyLen][]cityEntry // city keys bucketed by length in runes, for typo search
	// LongestCity is the length in runes of the longest city key, which bounds the search for a city
	// glued to the next word
	LongestCity int
}

// current is set by init right after initPlaces fills the base tables.
var current atomic.Pointer[Directory]

func init() {
	initPlaces()
	current.Store(newDirectory(nil))
}

// Current returns the directory version that records parsed now must use.
func Current() *Directory { return current.Load() }

// SetCurrent makes d the version for records parsed afterwards; tests put back the one they replaced.
func SetCurrent(d *Directory) { current.Store(d) }

// KnownCityWord: a city from the hand table or a classifier settlement, exact spelling only.
func (d *Directory) KnownCityWord(k string) bool {
	if _, ok := d.Cities[k]; ok {
		return true
	}
	return len(d.Settlements[k]) > 0
}

// Town returns the only town (not a village) of the classifier called k: "Шу" is a town in Жамбылская
// область and a village in Туркестанская, "Шалкар" a town and nine villages.
func (d *Directory) Town(k string) (Place, bool) {
	var town Place
	n := 0
	for _, p := range d.Settlements[k] {
		if p.Town {
			town = p
			n++
		}
	}
	return town, n == 1
}

// newDirectory copies the hand-written base tables and applies extra entries without overriding them.
func newDirectory(extend func(d *Directory)) *Directory {
	d := &Directory{
		Cities:    make(map[string]Place, len(baseCities)),
		Regions:   make(map[string]string, len(baseRegions)),
		Districts: make(map[string]DistrictInfo, len(baseDistricts)),
		Areas:     make(map[string][]Area, len(baseAreas)),
	}
	for k, v := range baseAreas {
		d.Areas[k] = v
	}
	for k, v := range baseCities {
		d.Cities[k] = v
	}
	for k, v := range baseRegions {
		d.Regions[k] = v
	}
	d.CityDistricts = map[string]map[string]DistrictInfo{}
	for k, v := range baseDistricts {
		d.Districts[k] = v
		if v.City != "" {
			d.addCityDistrict(k, v)
		}
	}
	if extend != nil {
		extend(d)
	}
	for k, p := range d.Cities {
		if r := []rune(k); len(r) < text.MaxFuzzyLen {
			d.cityByLen[len(r)] = append(d.cityByLen[len(r)], cityEntry{r, p})
			d.LongestCity = max(d.LongestCity, len(r))
		}
	}
	return d
}

func (d *Directory) addCityDistrict(k string, v DistrictInfo) {
	if d.CityDistricts[v.City] == nil {
		d.CityDistricts[v.City] = map[string]DistrictInfo{}
	}
	d.CityDistricts[v.City][k] = v
}

// District finds a district by key, first among the districts of city.
func (d *Directory) District(city, k string) (DistrictInfo, bool) {
	if v, ok := d.CityDistricts[city][k]; ok {
		return v, true
	}
	v, ok := d.Districts[k]
	return v, ok
}

type cityEntry struct {
	r []rune
	p Place
}

// fuzzyLimit is the allowed number of typos for a word of n letters.
func fuzzyLimit(n int) int {
	switch {
	case n < 5:
		return 0
	case n < 8:
		return 1
	}
	return 2
}

// LookupCity finds a settlement by its exact key or, failing that, by a unique close spelling ("алматыы").
func (d *Directory) LookupCity(k string) (Place, bool) { return d.lookupCityWithin(k, 2) }

// NearCities looks for cities one letter from k in one pass: the city when it is the only one and k may be
// read as it (as by LookupCity with one typo), and whether any city, whichever, is that close.
func (d *Directory) NearCities(k string) (p Place, unique, near bool) {
	var buf [text.MaxFuzzyLen]rune
	r := text.ToRunes(k, buf[:0])
	if len(r) >= text.MaxFuzzyLen {
		return Place{}, false, false
	}
	bestD, ambiguous := 2, false
	for n := len(r) - 1; n <= len(r)+1; n++ {
		if n < 0 || n >= text.MaxFuzzyLen {
			continue
		}
		for _, c := range d.cityByLen[n] {
			dist := text.DistanceLimit(r, c.r, 1)
			switch {
			case dist < bestD:
				p, bestD, ambiguous = c.p, dist, false
			case dist == bestD && c.p.Name != p.Name:
				ambiguous = true
			}
		}
	}
	if bestD > 1 {
		return Place{}, false, false
	}
	if _, exact := d.Cities[k]; exact {
		return d.Cities[k], true, true
	}
	unique = !ambiguous && (len(r) >= 5 || len(r) == 4 && !d.knownName(k))
	if !unique {
		p = Place{}
	}
	return p, unique, true
}

// lookupCityWithin is LookupCity with at most maxTypos edits.
func (d *Directory) lookupCityWithin(k string, maxTypos int) (Place, bool) {
	if p, ok := d.Cities[k]; ok {
		return p, true
	}
	var buf [text.MaxFuzzyLen]rune
	r := text.ToRunes(k, buf[:0])
	limit := min(fuzzyLimit(len(r)), maxTypos)
	if len(r) == 4 && maxTypos > 0 && !d.knownName(k) {
		limit = 1 // "смей", "тарз", "акту": four letters one typo from a large city and no other name
	}
	if limit == 0 || len(r) >= text.MaxFuzzyLen {
		return Place{}, false
	}
	var best Place
	bestD, ambiguous := limit+1, false
	for n := len(r) - limit; n <= len(r)+limit; n++ {
		if n < 0 || n >= text.MaxFuzzyLen {
			continue
		}
		for _, c := range d.cityByLen[n] {
			dist := text.DistanceLimit(r, c.r, limit)
			switch {
			case dist < bestD:
				best, bestD, ambiguous = c.p, dist, false
			case dist == bestD && c.p.Name != best.Name:
				ambiguous = true
			}
		}
	}
	if bestD <= limit && !ambiguous {
		return best, true
	}
	return Place{}, false
}

// knownName reports whether k is a name of its own: a settlement of the classifier, a region, a district,
// a marker or a street the map knows in some city. Such a short word is not read as a mistyped city.
func (d *Directory) knownName(k string) bool {
	if len(d.Settlements[k]) > 0 || d.Regions[k] != "" || Markers[k] != nil || OSM().KnownAnywhere(k) {
		return true
	}
	if _, ok := d.Districts[k]; ok {
		return true
	}
	return FuzzyMarker(k) != nil
}

// AdminKnown reports whether k names a known region, district or (also by a close spelling) city.
func (d *Directory) AdminKnown(class Class, k string) bool {
	switch class {
	case Region:
		_, ok := d.Regions[k]
		return ok
	case District:
		_, ok := d.Districts[k]
		return ok
	}
	_, ok := d.LookupCity(k)
	return ok
}

// KnownWord reports whether a folded word is a marker, a region, a district or a city (one typo allowed).
func (d *Directory) KnownWord(f string) bool {
	if _, ok := Markers[f]; ok {
		return true
	}
	if _, ok := d.Regions[f]; ok {
		return true
	}
	if _, ok := d.Districts[f]; ok {
		return true
	}
	if _, ok := d.lookupCityWithin(f, 1); ok {
		return true
	}
	return FuzzyMarker(f) != nil
}
