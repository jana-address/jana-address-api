// Package dict is what the parser knows about words and places: marker words and their meaning,
// the hand-written place tables, the KATO classifier added on top of them and well-known streets.
package dict

import (
	"sync"

	"github.com/jana-address/jana-address-api/internal/parser/text"
)

// Class is the meaning of a marker word.
type Class uint8

const (
	House Class = iota + 1
	Apartment
	Entrance
	Floor
	Street     // street type; canon is the output word (улица, проспект, ...)
	Living     // residential complex (ЖК)
	City       // город, қаласы
	Settlement // пос., село, ауыл
	District   // район, ауданы
	Region     // область, облысы
	Landmark   // магазин, ТЦ, напротив, ...
	Block      // корпус, блок, строение: consumes its number, not an output field
	Noise      // Казахстан, РК: ignored words
)

// Marker is a word that says what the next or the previous words are.
type Marker struct {
	Class Class
	Canon string
}

// IsUnit reports whether the marker takes a number: house, apartment, entrance, floor or block.
func (m *Marker) IsUnit() bool {
	return m != nil && (m.Class == House || m.Class == Apartment || m.Class == Entrance || m.Class == Floor || m.Class == Block)
}

// IsAdmin reports whether the marker names a city, settlement, district or region.
func (m *Marker) IsAdmin() bool {
	return m != nil && (m.Class == City || m.Class == Settlement || m.Class == District || m.Class == Region)
}

// Markers maps folded word forms to their meaning (Russian, Kazakh, Latin, common typos).
var Markers = map[string]*Marker{}

func addMarkers(class Class, canon string, forms ...string) {
	m := &Marker{Class: class, Canon: canon}
	for _, f := range forms {
		Markers[text.Fold(f)] = m
	}
}

func init() {
	addMarkers(House, "", "д", "дом", "домм", "дм", "үй", "уй", "house", "dom", "d")
	addMarkers(Apartment, "", "кв", "квартира", "кварт", "квартра", "кв-ра", "пәтер", "flat", "kv", "квю", "apt", "apartment", "оф", "офис", "office", "ofis")
	addMarkers(Entrance, "", "подъезд", "подьезд", "подезд", "падезд", "под", "пд", "кіреберіс", "entrance", "ent", "pod", "podezd", "podyezd")
	addMarkers(Floor, "", "этаж", "етаж", "этж", "етж", "эт", "қабат", "floor", "fl", "etazh", "etaj", "et")

	addMarkers(Street, "улица", "ул", "улица", "улца", "улиц", "уица", "ulitsa", "ulica", "көшесі", "көш", "street", "st", "ul", "ulica")
	addMarkers(Street, "проспект", "пр", "пр-т", "просп", "проспект", "даңғылы", "даңғ", "avenue", "ave", "pr", "prospekt", "prosp", "pr-t", "пр-кт", "dangyly", "даң")
	addMarkers(Street, "микрорайон", "мкр", "мкп", "мкрн", "шағынаудан", "ықшамаудан", "мкр-н", "м-н", "мик-н", "микрорайон", "mkr", "mkrn", "mikrorayon", "mikroraion", "microdistrict")
	addMarkers(Street, "квартал", "квартал", "кв-л", "квл", "kvartal")
	addMarkers(Street, "переулок", "пер", "переулок", "пер-к", "тұйық", "pereulok", "per")
	addMarkers(Street, "бульвар", "б-р", "бульвар", "бул", "bulvar", "bul")
	addMarkers(Street, "шоссе", "шоссе", "ш", "shosse", "трасса") // "трасса" is not an output type
	addMarkers(Living, "жилой комплекс", "жк", "zhk", "jk")

	addMarkers(City, "", "г", "город", "гор", "горд", "қаласы", "қалас", "қала", "қ", "kalasy")
	addMarkers(Settlement, "", "пос", "поселок", "пгт", "село", "ауыл", "аул", "ауылы")
	addMarkers(District, "", "район", "р-н", "рн", "ауданы", "аудан", "ауд")
	addMarkers(Region, "", "область", "обл", "облысы", "обл-ть")

	addMarkers(Block, "", "корпус", "корп", "блок", "строение", "стр", "block", "korp", "korpus", "blok")
	addMarkers(Noise, "", "казахстан", "қазақстан", "рк", "республика", "kz", "kazakhstan")
	addMarkers(Landmark, "", "у", "во", "за", "между", "магазин", "маг", "тц", "трц", "тд", "рынок", "базар", "школа", "мектеп", "садик",
		"напротив", "возле", "около", "рядом", "остановка", "аялдама", "qazpost", "казпочта", "аптека", "кафе",
		"азс", "бц", "супермаркет", "маркет")
}

// fuzzyForms are long marker words that are often mistyped ("подлъезд", "кваритра", "микрорайно").
var fuzzyForms = []string{"подъезд", "квартира", "бульвар", "корпус", "микрорайон", "проспект", "переулок", "улица", "квартал",
	"город", "район", "область", "облысы", "қаласы", "ауданы", "көшесі", "даңғылы", "этаж", "магазин", "поселок"}

// FuzzyMarker finds a marker within a small edit distance: 1 for words of 5–7 letters, 2 for longer ones.
func FuzzyMarker(f string) *Marker {
	var buf [text.MaxFuzzyLen]rune
	r := text.ToRunes(f, buf[:0])
	if len(r) < 5 || len(r) >= text.MaxFuzzyLen {
		return nil
	}
	limit := 1
	if len(r) >= 8 {
		limit = 2
	}
	fuzzyIndex.Do(buildFuzzyIndex)
	var best *Marker
	bestD := limit + 1
	for _, form := range fuzzyMarkerForms {
		if d := text.DistanceLimit(r, form.r, limit); d < bestD {
			best, bestD = form.m, d
		} else if d == bestD && best != nil && form.m != best {
			best = nil // ambiguous between two markers
		}
	}
	return best
}

type markerForm struct {
	r []rune
	m *Marker
}

var (
	fuzzyIndex       sync.Once
	fuzzyMarkerForms []markerForm
)

// buildFuzzyIndex prepares the marker forms for edit-distance search once.
func buildFuzzyIndex() {
	for _, f := range fuzzyForms {
		ff := text.Fold(f)
		fuzzyMarkerForms = append(fuzzyMarkerForms, markerForm{[]rune(ff), Markers[ff]})
	}
}

// RealStreetTypes are street types that can coexist with a microdistrict (which then becomes living_area).
var RealStreetTypes = map[string]bool{"улица": true, "проспект": true, "переулок": true, "бульвар": true, "шоссе": true}

// NumberedAreaTypes may carry a number inside the name: "Аксай 3а", "Нурсити 2", "66 квартал".
var NumberedAreaTypes = map[string]bool{"микрорайон": true, "квартал": true, "жилой комплекс": true}

// GlueMarkers can start or end a glued word.
var GlueMarkers = map[string]bool{}

func init() {
	for _, w := range []string{"қаласы", "қалас", "город", "мкр", "мкрн", "микрорайон", "улица", "көшесі", "даңғылы",
		"проспект", "ауданы", "район", "ықшамаудан", "шағынаудан"} {
		GlueMarkers[text.Fold(w)] = true
	}
}

// AfterCityMarkers are short type words that are split off only right after a city: "алматыул…", "астанагородпр-т…".
var AfterCityMarkers = map[string]bool{}

func init() {
	for _, w := range []string{"ул", "пр", "пр-т", "пер", "мкр", "мкрн", "горд", "кала", "қала", "қалас",
		"ауылы", "ауыл", "село", "көш", "даңғ"} {
		AfterCityMarkers[text.Fold(w)] = true
	}
}
