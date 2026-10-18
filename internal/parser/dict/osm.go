package dict

import (
	"bufio"
	"compress/gzip"
	"embed"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// The street directory built from OpenStreetMap by cmd/osmref (manifest.json lists the source, the
// date, the areas and the checks): the city districts of Алматы, Астана, Шымкент and Тараз, and every other
// city with its own administration taken whole, with an empty district.
//
//go:embed osm/streets.tsv.gz osm/houses.tsv.gz osm/areas.tsv.gz osm/manifest.json
var osmFiles embed.FS

// StreetRef is where a street or a living area lies: the district of most of its segments and
// buildings ("" in a city taken whole), their share there, the number of votes and the only street
// type the map gives it.
type StreetRef struct {
	District string
	Share    float64
	Votes    int
	Type     string
}

// AnyType is the type column of the row that joins all streets of one name, for a line without a type.
const AnyType = "*"

// OSMRef holds the street directory, keyed by the canonical city name, StreetKey and the street type.
type OSMRef struct {
	Streets map[string]map[string]StreetRef  // key + "\x00" + type (or AnyType)
	Houses  map[string]map[string]string     // key + "\x00" + type + "\x00" + HouseKey -> district, streets across a border
	Areas   map[string]map[string]StreetRef  // area key
	Numbers map[string]map[string][]Numbered // key + "\x00" + type -> the same buildings by number

	anyOnce sync.Once
	any     map[string]struct{}            // street keys of all cities
	glued   map[string]map[string]struct{} // city ("" for all) -> street keys without spaces
}

// Street finds a street by its key and type ("улица", "проспект" or AnyType).
func (r *OSMRef) Street(city, key, typ string) (StreetRef, bool) {
	s, ok := r.Streets[city][key+"\x00"+typ]
	return s, ok
}

// House returns the district of one building of a street.
func (r *OSMRef) House(city, key, typ, house string) (string, bool) {
	d, ok := r.Houses[city][key+"\x00"+typ+"\x00"+HouseKey(house)]
	return d, ok
}

// Neighbors returns the district of a house the map does not know from the nearest numbers on its
// side of the street (NeighborDistrict).
func (r *OSMRef) Neighbors(city, key, typ, house string) (string, bool) {
	n, ok := HouseNumber(house)
	if !ok {
		return "", false
	}
	return NeighborDistrict(r.Numbers[city][key+"\x00"+typ], n)
}

// KnownAnywhere reports whether some city of the map has a street with the key.
func (r *OSMRef) KnownAnywhere(key string) bool {
	r.anyOnce.Do(r.buildAny)
	_, ok := r.any[key]
	return ok
}

// GluedStreet reports whether the city has a street written as the one word w, its key without spaces:
// "толеби" for Толе би. For a city the map does not cover, the streets of all cities count.
func (r *OSMRef) GluedStreet(city, w string) bool {
	r.anyOnce.Do(r.buildAny)
	w = strings.ReplaceAll(w, "-", "")
	if m, ok := r.glued[city]; ok {
		_, known := m[w]
		return known
	}
	_, known := r.glued[""][w]
	return known
}

func (r *OSMRef) buildAny() {
	r.any = map[string]struct{}{}
	r.glued = map[string]map[string]struct{}{"": {}}
	for city, streets := range r.Streets {
		g := map[string]struct{}{}
		for k := range streets {
			if name, typ, _ := strings.Cut(k, "\x00"); typ == AnyType {
				r.any[name] = struct{}{}
				sq := strings.ReplaceAll(name, " ", "")
				g[sq] = struct{}{}
				r.glued[""][sq] = struct{}{}
			}
		}
		r.glued[city] = g
	}
}

// OSM returns the embedded street directory; it is read once, on first use.
var OSM = sync.OnceValue(func() *OSMRef {
	r, err := loadOSM()
	if err != nil {
		panic("dict: embedded OSM directory: " + err.Error())
	}
	return r
})

func loadOSM() (*OSMRef, error) {
	r := &OSMRef{Streets: map[string]map[string]StreetRef{}, Houses: map[string]map[string]string{}, Areas: map[string]map[string]StreetRef{}}
	for _, t := range []struct {
		name string
		cols int
		add  func(f []string) error
	}{
		// city, key, type, district, share, votes, only type
		{"streets", 7, func(f []string) error {
			share, err := strconv.ParseFloat(f[4], 64)
			if err != nil {
				return err
			}
			n, err := strconv.Atoi(f[5])
			put(r.Streets, f[0], f[1]+"\x00"+f[2], StreetRef{District: f[3], Share: share, Votes: n, Type: f[6]})
			return err
		}},
		// city, key, type, house, district
		{"houses", 5, func(f []string) error {
			put(r.Houses, f[0], f[1]+"\x00"+f[2]+"\x00"+f[3], f[4])
			return nil
		}},
		// city, key, district, share
		{"areas", 4, func(f []string) error {
			share, err := strconv.ParseFloat(f[3], 64)
			put(r.Areas, f[0], f[1], StreetRef{District: f[2], Share: share})
			return err
		}},
	} {
		if err := readTSV("osm/"+t.name+".tsv.gz", t.cols, t.add); err != nil {
			return nil, err
		}
	}
	r.Numbers = map[string]map[string][]Numbered{}
	for city, hs := range r.Houses {
		for k, d := range hs {
			i := strings.LastIndexByte(k, 0)
			if n, ok := HouseNumber(k[i+1:]); ok {
				put(r.Numbers, city, k[:i], append(r.Numbers[city][k[:i]], Numbered{n, d}))
			}
		}
	}
	for _, streets := range r.Numbers {
		for _, list := range streets {
			SortNumbered(list)
		}
	}
	return r, nil
}

func put[V any](m map[string]map[string]V, city, k string, v V) {
	if m[city] == nil {
		m[city] = map[string]V{}
	}
	m[city][k] = v
}

func readTSV(path string, cols int, add func([]string) error) error {
	f, err := osmFiles.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		if strings.Contains(err.Error(), "EOF") {
			return nil // an empty snapshot
		}
		return fmt.Errorf("%s: %w", path, err)
	}
	sc := bufio.NewScanner(zr)
	for n := 1; sc.Scan(); n++ {
		fields := strings.Split(sc.Text(), "\t")
		if len(fields) != cols {
			return fmt.Errorf("%s:%d: %d columns, want %d", path, n, len(fields), cols)
		}
		if err := add(fields); err != nil {
			return fmt.Errorf("%s:%d: %w", path, n, err)
		}
	}
	return sc.Err()
}

// OSMManifest is the manifest of the embedded street directory, for the health endpoint and the docs.
func OSMManifest() []byte {
	b, _ := osmFiles.ReadFile("osm/manifest.json")
	return b
}
