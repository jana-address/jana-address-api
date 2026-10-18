package parser

import (
	"strings"
	"unicode"

	"github.com/jana-address/jana-address-api/internal/parser/dict"
	"github.com/jana-address/jana-address-api/internal/parser/text"
)

// tasZholy is "тасжолы", шоссе written as one word.
var tasZholy = text.Fold("тасжолы")

func (st *state) classify() {
	for i := range st.toks {
		t := &st.toks[i]
		if t.kind != tWord {
			continue
		}
		// "шағын аудан(ы)" = микрорайон (two words)
		if (t.fold == "шагын" || t.fold == "ыкшам") && i+1 < len(st.toks) && (st.toks[i+1].fold == "аудан" || st.toks[i+1].fold == "ауданы") {
			t.mk = dict.Markers["мкр"]
			st.toks[i+1].mk = t.mk
			st.toks[i+1].used = true // second word is part of the marker
			t.tail = 1
			continue
		}
		// "тас жолы" = шоссе (two words)
		if t.fold == "тас" && i+1 < len(st.toks) && st.toks[i+1].fold == text.Fold("жолы") {
			t.mk = dict.Markers["шоссе"]
			st.toks[i+1].mk = t.mk
			st.toks[i+1].used = true
			t.tail = 1
			continue
		}
		// ЖК written in several tokens: "ж.к.", "ж/к", "жилой комплекс", "тұрғын үй кешені"
		if n := st.livingTail(i); n > 0 {
			t.mk = dict.Markers["жк"]
			t.tail = n
			for k := i + 1; k <= i+n; k++ {
				st.toks[k].used = true
				st.toks[k].mk = nil
			}
			continue
		}
		if m, ok := dict.Markers[t.fold]; ok && t.mk == nil {
			t.mk = m
			if m.Class == dict.Noise {
				republic := t.fold == "республика"
				if republic && !(i+1 < len(st.toks) && st.toks[i+1].fold == "казахстан") {
					t.mk = nil
				} else if st.opensLine(i) || st.toks[i-1].kind == tPunct || (st.toks[i-1].mk != nil && st.toks[i-1].mk.Class == dict.Noise) {
					// also after a postcode: "040949 Казахстан Кошкарбаева 7"
					t.used = true
					continue
				}
				t.mk = nil
			}
		}
		// "тасжолы" is "тас жолы" written as one word. It is not in dict.Markers: the street keys of the map
		// keep it in the name ("темирлан тасжолы"), see osmStreet.
		if t.mk == nil && t.fold == tasZholy {
			t.mk = dict.Markers["шоссе"]
		}
		// typo in a long marker word: "подлъезд", "кваритра"; a typo right before a known city: "гролд актобе"
		if t.mk == nil && !st.d.AdminKnown(dict.City, t.fold) {
			m := dict.FuzzyMarker(t.fold)
			if signed := text.RuSpelling(strings.ToLower(t.text)); m == nil && strings.ContainsAny(signed, "ьъ") {
				m = dict.FuzzyMarker(signed) // a soft sign typed for a letter: "эьаж" is "этаж", not "эаж"
			}
			if m != nil {
				t.mk = m
			} else if i+1 < len(st.toks) && st.d.AdminKnown(dict.City, st.toks[i+1].fold) &&
				text.DistanceLimit([]rune(t.fold), []rune("город"), 2) <= 2 {
				t.mk = dict.Markers["город"]
			}
		}
		// an initial inside a street name: "ул. Д. Кунаева", "пр. С. Сейфуллина"; without the type only a capital
		// before a capitalised word that is not a place: "Алматы, С. Сейфуллина 45", "Г. Мурата 5".
		// A street type stays a type ("Ш. Коргалжынское" is шоссе), and so does қ. after a city ("АЛМАТЫ К.").
		if len([]rune(t.text)) == 1 && st.isPunct(i+1, ".") && i+2 < len(st.toks) && st.toks[i+2].kind == tWord {
			p := i - 1
			for st.isPunct(p, ".") {
				p--
			}
			afterType := p >= 0 && st.toks[p].mk != nil && st.toks[p].mk.Class == dict.Street
			afterCity := p == i-1 && p >= 0 && st.toks[p].kind == tWord && t.mk != nil && t.mk.Class == dict.City && st.d.AdminKnown(dict.City, st.toks[p].fold)
			initial := text.Capital(t.text) && text.Capital(st.toks[i+2].text) && !st.placeAt(i+2) && !afterCity && (t.mk == nil || t.mk.Class != dict.Street)
			if afterType || initial {
				t.mk = nil
				continue
			}
		}
		// "с." before a name is a village, otherwise a preposition; "с Отеген батыр" without a dot only before a known
		// place, or one of two words one letter off where the line opens: "040712 с Отеген баьыр"
		if t.fold == "с" && (st.isPunct(i+1, ".") || st.knownPlaceAt(i+1) || st.opensLine(i) && st.closePairAt(i+1)) {
			t.mk = &dict.Marker{Class: dict.Settlement}
		}
		if t.fold == "кала" && t.mk != nil && !(i > 0 && st.toks[i-1].kind == tWord && st.cityBeforeKala(st.toks[i-1], *t)) {
			t.mk = nil // "Жана кала" is a name
		}
		// Latin "of." or "of 12" is an office, unlike the "of" in "Republic of Kazakhstan"
		if t.fold == "of" && t.mk == nil && (st.isPunct(i+1, ".") || i+1 < len(st.toks) && st.toks[i+1].kind == tNum) {
			t.mk = dict.Markers["оф"]
		}
		if t.fold == "п" {
			j := i + 1
			for st.isPunct(j, ".") {
				j++
			}
			if j < len(st.toks) && st.toks[j].kind == tWord && dict.Markers[st.toks[j].fold] == nil {
				t.mk = dict.Markers["пос"]
			}
		}
	}
}

// knownPlaceAt reports whether a city or a classifier settlement of one or two words starts at i.
func (st *state) knownPlaceAt(i int) bool {
	if i >= len(st.toks) || st.toks[i].kind != tWord {
		return false
	}
	k := st.toks[i].fold
	if st.d.KnownCityWord(k) {
		return true
	}
	if i+1 < len(st.toks) && st.toks[i+1].kind == tWord {
		return st.d.KnownCityWord(k + " " + st.toks[i+1].fold)
	}
	return false
}

// closePairAt reports whether two words at i are a close spelling of a place of two words: "отеге батыр".
func (st *state) closePairAt(i int) bool {
	if i+1 >= len(st.toks) || st.toks[i].kind != tWord || st.toks[i+1].kind != tWord {
		return false
	}
	p, ok := st.d.LookupCity(st.toks[i].fold + " " + st.toks[i+1].fold)
	return ok && strings.Contains(p.Name, " ")
}

// placeAt reports whether a place name starts at i: a city, or a settlement or a close spelling of a city
// that is not followed by a number. "С. Сейфуллина 45" is a street with an initial although villages
// called Сейфуллина exist; "С. Кайнар, Новая 4" and "Г. Шымкнт, Тауке хана 20" are places.
func (st *state) placeAt(i int) bool {
	k := st.toks[i].fold
	if _, ok := st.d.Cities[k]; ok {
		return true
	}
	if i+1 < len(st.toks) && st.toks[i+1].kind == tWord && st.d.KnownCityWord(k+" "+st.toks[i+1].fold) {
		return true // "С. Отеген батыр"
	}
	return (len(st.d.Settlements[k]) > 0 || st.d.AdminKnown(dict.City, k)) && st.typoCityBefore(i)
}

// livingTail returns how many tokens after i complete a multi-token ЖК marker.
func (st *state) livingTail(i int) int {
	at := func(k int, w string) bool { return k < len(st.toks) && st.toks[k].fold == text.Fold(w) }
	switch {
	case at(i, "ж"):
		if (st.isPunct(i+1, ".") || st.isPunct(i+1, "/")) && at(i+2, "к") {
			if st.isPunct(i+3, ".") {
				return 3
			}
			return 2
		}
	case at(i, "жилой"):
		if i+1 < len(st.toks) && text.DistanceLimit([]rune(st.toks[i+1].fold), []rune("комплекс"), 2) <= 2 {
			return 1
		}
	case at(i, "тұрғын"):
		if at(i+1, "үй") && at(i+2, "кешені") {
			return 2
		}
	}
	return 0
}

// segment numbers tokens by ',', ';', brackets and a '/' between two words (bilingual pairs).
func (st *state) segment() {
	seg := 0
	st.segs = [][2]int{{-1, -1}}
	for i := range st.toks {
		t := &st.toks[i]
		if t.kind == tPunct {
			sep := strings.Contains(",;()", t.text)
			if t.text == "/" && !t.used && i > 0 && i+1 < len(st.toks) && st.toks[i-1].kind == tWord && st.toks[i+1].kind == tWord {
				sep = true
			}
			if sep {
				seg++
				st.segs = append(st.segs, [2]int{-1, -1})
				t.seg = -1
				continue
			}
		}
		t.seg = seg
		if st.segs[seg][0] < 0 {
			st.segs[seg][0] = i
		}
		st.segs[seg][1] = i
	}
}

// segRange returns the first and last token of a segment, -1, -1 for an empty one.
func (st *state) segRange(seg int) (int, int) {
	if seg < 0 || seg >= len(st.segs) {
		return -1, -1
	}
	return st.segs[seg][0], st.segs[seg][1]
}

// cityBeforeKala reports whether the word before "қала" is a city: a known one, or one with typos that was
// glued to it: "шымекнтқала", "караганыдқала".
func (st *state) cityBeforeKala(prev, t token) bool {
	if st.d.KnownCityWord(prev.fold) {
		return true
	}
	if !glued(prev, t) || !unicode.IsLower([]rune(t.text)[0]) {
		return false
	}
	_, ok := st.d.LookupCity(prev.fold)
	return ok
}
