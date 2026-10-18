package parser

import (
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/jana-address/jana-address-api/internal/parser/dict"
	"github.com/jana-address/jana-address-api/internal/parser/text"
)

type cand struct {
	typ      string
	mk       int // marker token index
	a, b     int // name token range
	isLiving bool
}

func (st *state) laterNumberExists(i int) bool {
	if _, ok := st.vals[House]; ok {
		return true
	}
	for j := i + 1; j < len(st.toks); j++ {
		if st.availNum(j) {
			return true
		}
	}
	return false
}

// kazakhStreetPostfix are Kazakh street type words, written only after the street name: never a part of a ЖК name.
var kazakhStreetPostfix = map[string]bool{text.Fold("көшесі"): true, text.Fold("көш"): true, text.Fold("даңғылы"): true,
	text.Fold("даңғ"): true, text.Fold("даң"): true, text.Fold("тұйығы"): true, text.Fold("бульвары"): true, text.Fold("желекжолы"): true}

// collectForward reads a street name after a type marker: "ул. Курильская", "Мкр Аксай 3а".
func (st *state) collectForward(m int, typ string) (int, int) {
	j := m + 1 + st.toks[m].tail
	quoted := false
	for st.isPunct(j, ".\"") {
		quoted = quoted || st.isPunct(j, "\"")
		j++
	}
	a, b := -1, -1
	for k := j; k < len(st.toks); k++ {
		t := st.toks[k]
		if t.used || t.seg != st.toks[m].seg && t.seg >= 0 {
			break
		}
		switch t.kind {
		case tWord:
			if t.mk != nil && typ == "жилой комплекс" && a >= 0 && st.typeInName(k) && (!st.opensName(k) || st.typedLater(k)) {
				break // a type word inside the name: "ЖК Экспо Бульвар", "ЖК Экспо Бульвар Акжол ул. 5"; "ЖК Нурлы Тау ул. Толе би" ends before "ул."
			}
			if t.mk != nil {
				return a, b
			}
			if typ == "жилой комплекс" && a >= 0 && text.IsLatin(st.toks[b].text) != text.IsLatin(t.text) {
				return a, b
			}
			if !dict.RealStreetTypes[typ] && a >= 0 && st.postfixStreetAt(k+1) {
				return a, b
			}
		case tNum:
			if t.ignore {
				return a, b
			}
			if dict.NumberedAreaTypes[typ] {
				hyphened := a >= 0 && st.isPunct(k-1, "-") && glued(st.toks[k-1], t) // "Самал-2"
				if !(hyphened || st.laterNumberExists(k)) {
					return a, b
				}
				if a < 0 {
					a = k
				}
				return a, k
			}
			// "ул. 8 Марта": a number right after the type followed by a word
			if a < 0 && k+1 < len(st.toks) && st.toks[k+1].kind == tWord && st.toks[k+1].mk == nil && !st.toks[k+1].used &&
				!st.d.AdminKnown(dict.City, st.toks[k+1].fold) {
				break
			}
			return a, b
		case tPunct:
			if t.text == "\"" && quoted && a >= 0 && !dict.RealStreetTypes[typ] {
				return a, b // the closing quote ends an area name: "ЖК «Асыл Тау» Маметовой 18"
			}
			if (t.text == "-" || t.text == "\"") && a >= 0 && k+1 < len(st.toks) && st.toks[k+1].kind != tPunct {
				continue
			}
			if t.text == "." && a >= 0 && len([]rune(st.toks[k-1].text)) == 1 {
				continue
			}
			return a, b
		}
		if a < 0 {
			a = k
		}
		b = k
	}
	return a, b
}

// collectBackward reads a name before a postfix type: "Рыбинский переулок", "Нурсити 2 микрорайон", "66квартал".
func (st *state) collectBackward(m int, typ string) (int, int) {
	a, b := -1, -1
	for k := m - 1; k >= 0; k-- {
		t := st.toks[k]
		if t.used || t.seg != st.toks[m].seg {
			break
		}
		ok := false
		switch t.kind {
		case tWord:
			ok = t.mk == nil
		case tNum:
			ok = dict.NumberedAreaTypes[typ] && !t.ignore
		case tPunct:
			ok = t.text == "-" && a >= 0 || strings.Contains("\"«»", t.text)
		}
		if !ok {
			break
		}
		if t.kind == tPunct && t.text != "-" {
			continue // a quote around the name
		}
		if b < 0 {
			b = k
		}
		a = k
	}
	return a, b
}

// postfixAreaMarkers follow the name they type: "Самал шағын ауданы", "Хайвил ТҮК", "Мирас тұрғын үй кешені".
var postfixAreaMarkers = map[string]bool{text.Fold("шағын"): true, text.Fold("ықшам"): true, text.Fold("шағынаудан"): true,
	text.Fold("ықшамаудан"): true, text.Fold("түк"): true, text.Fold("тұрғын"): true}

// nameBefore reports whether the microdistrict or ЖК marker at i types the name before it (ba..bb) rather
// than the words after it (fa..fb, -1 when none): after a Kazakh postfix marker ("Самал шағын ауданы"), a
// number before the marker and after it ("6 шағын ауданы 10": 10 is the house), or when the words after the
// marker are the street: they run into a postfix street type ("32 мкр Абылай хан көшесі 48") or end at the
// house ("Баганашыл мкр. Сейфуллина 44", "Магнолия жк Бухар жырау д 33"). A word and a number before the
// marker are a street and its house ("Абая 10 мкр Самал"), and words after the marker that the city knows
// as its living area and not as a street, unlike the words before, stay the area.
func (st *state) nameBefore(i int, typ string, fa, fb, ba, bb int) bool {
	if ba < 0 {
		return false
	}
	if fa < 0 || postfixAreaMarkers[st.toks[i].fold] {
		return true
	}
	if st.vals[City] == "" && st.placeName(ba, bb) {
		return false // "Абай мкр Самал 5": with no city in the line, a town before the marker is the place
	}
	if st.toks[bb].kind == tNum {
		if ba < bb && !(st.isPunct(bb-1, "-") && glued(st.toks[bb-1], st.toks[bb])) {
			return false // "Абая 10 мкр Самал 5"; "Самал-2 мкр" is a name
		}
		if ba == bb && ba > 0 && st.toks[ba-1].kind == tWord && !st.toks[ba-1].used {
			return false // "ул Абая 10 мкр Самал": the number is the house of the street before
		}
	}
	if fa == fb && st.toks[fa].kind == tNum {
		return dict.NumberedAreaTypes[typ] && st.availNum(i-1)
	}
	next := fb + 1
	if next < len(st.toks) && st.toks[next].kind == tWord && st.toks[next].mk == nil && !st.toks[next].used {
		next++ // collectForward leaves the last word to a postfix street type: "мкр думан жибек жолы даң."
	}
	if st.postfixStreetAt(next) {
		return true
	}
	h := fb + 1
	for st.isPunct(h, ".,") {
		h++
	}
	if !(st.availNum(h) || st.houseAt(h)) {
		return false
	}
	if st.toks[ba].kind == tNum {
		return true // "5 мкр Айтеке би 43"
	}
	city := st.vals[City]
	return !st.knownArea(city, fa, fb) || st.knownArea(city, ba, bb) || st.streetVotes(city, fa, fb) >= 3
}

// placeName reports whether the words a..b name a city of the hand table or a town of the classifier.
func (st *state) placeName(a, b int) bool {
	for k := a; k <= b; k++ {
		if st.toks[k].kind != tWord {
			return false
		}
	}
	k := st.keyOf(a, b)
	if st.d.Cities[k] != (dict.Place{}) {
		return true
	}
	_, town := st.d.Town(k)
	return town
}

// knownArea reports whether the words a..b name a living area of the city in the map or the hand tables.
func (st *state) knownArea(city string, a, b int) bool {
	if city == "" {
		return false
	}
	k, _ := dict.StreetKey(st.text(a, b))
	if _, ok := dict.OSM().Areas[city][k]; ok {
		return true
	}
	for _, ar := range st.d.Areas[text.Key(st.text(a, b))] {
		if ar.City == city {
			return true
		}
	}
	if _, ok := dict.AlmatyAreas[st.toks[a].fold]; !ok || city != "Алматы" {
		return false
	}
	for k := a + 1; k <= b; k++ {
		if st.toks[k].kind == tWord {
			return false
		}
	}
	return true // "Самал", "Мамыр-5"
}

// areaThenStreet splits a microdistrict or ЖК name that runs into an untyped street, by the directory
// of the city: "мкр жагалау кабанбай батыра 37" is the area "жагалау" and the street "кабанбай батыра"
// when the city knows both, or knows the street well. It returns the first token of the street, or -1.
// A name the directory knows whole is never split.
func (st *state) areaThenStreet(c *cand) int {
	city := st.vals[City]
	ref := dict.OSM()
	if !areaType[c.typ] || c.b <= c.a || (ref.Streets[city] == nil && city != "Алматы") {
		return -1
	}
	key := func(a, b int) string { k, _ := dict.StreetKey(st.text(a, b)); return k }
	knownStreet := func(k string) bool { s, ok := ref.Street(city, k, dict.AnyType); return ok && s.Votes >= 3 }
	knownArea := func(a, b int) bool { return st.knownArea(city, a, b) }
	if knownStreet(key(c.a, c.b)) || knownArea(c.a, c.b) {
		return -1
	}
	var splits []int // word tokens that may start the street
	for j := c.a + 1; j <= c.b; j++ {
		if st.toks[j].kind == tWord && st.toks[j-1].kind == tWord {
			splits = append(splits, j)
		}
	}
	for _, j := range splits {
		// both halves known first: map names are noisy ("Алтын" is an area, "Тау" a street), and
		// "ЖК Алтын Булак" or "ЖК Асыл Тау" stay whole, as the map knows Булак and Тау too little
		if k := key(j, c.b); (st.strongStreet(city, k) || bySurname(city, k)) && knownArea(c.a, j-1) && !strings.ContainsAny(k, "0123456789") {
			return j
		}
	}
	for _, j := range splits { // a street mistyped by one letter: "мкр Асар Рыскулоова 10"
		if k := key(j, c.b); knownArea(c.a, j-1) && !strings.ContainsAny(k, "0123456789") && closeStreet(city, k) {
			return j
		}
	}
	for _, j := range splits { // an area mistyped by one letter: "мкр Сампл Шевченко 50"
		if k := key(j, c.b); st.strongStreet(city, k) && !strings.ContainsAny(k, "0123456789") && closeArea(city, key(c.a, j-1)) {
			return j
		}
	}
	// an area the map does not know, before a street it knows well: "мкр Мерей Айтеке би 31". A name one
	// letter from a known area is that area mistyped, even when its last word is also a street:
	// "ЖК Туа Самал" is Тау Самал, not the street Самал
	if closeArea(city, key(c.a, c.b)) {
		return -1
	}
	for _, j := range splits {
		if k := key(j, c.b); st.strongStreet(city, k) && !strings.ContainsAny(k, "0123456789") {
			return j
		}
	}
	return -1
}

// closeArea reports whether exactly one living area of the city in the map (a key of five letters or
// more) is one letter from the key: "сампл" is Самал.
func closeArea(city, k string) bool {
	var buf, kbuf [text.MaxFuzzyLen]rune
	r := text.ToRunes(k, kbuf[:0])
	if len(r) < 5 || len(r) >= text.MaxFuzzyLen-1 {
		return false
	}
	found := ""
	for name := range dict.OSM().Areas[city] {
		if n := utf8.RuneCountInString(name); n < len(r)-1 || n > len(r)+1 || n < 5 {
			continue
		}
		if text.DistanceLimit(r, text.ToRunes(name, buf[:0]), 1) <= 1 {
			if found != "" && found != name {
				return false
			}
			found = name
		}
	}
	return found != ""
}

// bySurname reports whether the key is the last word of exactly one street of the city that the map knows
// well: "ауэзова" is Мухтара Ауэзова in Астана, "чокина" Академика Чокина in Павлодар.
func bySurname(city, k string) bool {
	return len([]rune(k)) >= 5 && !strings.Contains(k, " ") && len(surnames()[city][k]) == 1
}

// surnames maps a city and the last word of its streets of two words or more, known well, to their keys.
var surnames = sync.OnceValue(func() map[string]map[string][]string {
	out := map[string]map[string][]string{}
	for city, streets := range dict.OSM().Streets {
		m := map[string][]string{}
		for key, s := range streets {
			name, typ, _ := strings.Cut(key, "\x00")
			i := strings.LastIndexByte(name, ' ')
			if typ != dict.AnyType || s.Votes < 3 || i < 0 {
				continue
			}
			m[name[i+1:]] = append(m[name[i+1:]], name)
		}
		out[city] = m
	}
	return out
})

// closeStreet reports whether exactly one street of the city that the map knows well (a key of six letters
// or more, three segments and buildings) is one letter from the key: "рыскулоова" is Рыскулова.
func closeStreet(city, k string) bool {
	var buf, kbuf [text.MaxFuzzyLen]rune
	r := text.ToRunes(k, kbuf[:0])
	if len(r) < 6 || len(r) >= text.MaxFuzzyLen-1 {
		return false
	}
	found := ""
	for key, s := range dict.OSM().Streets[city] {
		name, typ, _ := strings.Cut(key, "\x00")
		if typ != dict.AnyType || s.Votes < 3 {
			continue
		}
		if n := utf8.RuneCountInString(name); n < len(r)-1 || n > len(r)+1 || n < 6 {
			continue
		}
		if text.DistanceLimit(r, text.ToRunes(name, buf[:0]), 1) <= 1 {
			if found != "" && found != name {
				return false // two streets as close: no guess
			}
			found = name
		}
	}
	return found != ""
}

// strongStreet reports whether the city's map knows the street key well enough to cut it off a living
// area name: a key of five letters or more with at least three segments and buildings, a long key such as
// "саттарханова" with any, a short one such as "абая" only with many.
func (st *state) strongStreet(city, k string) bool {
	s, ok := dict.OSM().Street(city, k, dict.AnyType)
	n := len([]rune(k))
	return ok && (n >= 8 || n >= 5 && s.Votes >= 3 || n == 4 && s.Votes >= 20)
}

// splitShared sets the border between a microdistrict or ЖК and a street that both read the same words:
// the area forward and the street backward from its postfix type ("мкр Коктем Кабанбай батыра улица 53"),
// or the street forward and the area backward ("ул. Абая Самал мкр 5", "ул Жастар 4 мкрн").
func (st *state) splitShared(cands []cand) {
	for k := 1; k < len(cands); k++ {
		p, s := &cands[k-1], &cands[k]
		if !(p.a > p.mk && s.b < s.mk && s.a <= p.b) {
			continue // no shared words, or not the first read forward and the second backward
		}
		areaFirst := !dict.RealStreetTypes[p.typ] && dict.RealStreetTypes[s.typ]
		if !areaFirst && !(dict.RealStreetTypes[p.typ] && !dict.RealStreetTypes[s.typ]) {
			continue
		}
		if !areaFirst && s.b > p.b {
			s.a = p.b + 1 // "ул Жастар 4 мкрн": the street ends before the number, the rest is the area
			continue
		}
		a, b, living := min(p.a, s.a), s.b, s.isLiving
		if areaFirst {
			a, b, living = p.a, max(p.b, s.b), p.isLiving
		}
		if j := st.areaStreetBorder(a, b, areaFirst, living); j > a {
			p.b, s.a = st.beforeQuotes(j-1), j
		}
	}
}

// areaStreetBorder returns the first token of the second name in the words a..b that hold an area and a
// street (the area first when areaFirst; living: the area is a ЖК), or -1. A quoted name is never cut. The
// longest half the city knows as a street sets the border, then the longest one it knows as an area.
// Otherwise a microdistrict is one word ("мкр Строитель Темира Масина ул."), and the street is one word
// before a ЖК ("ЖК Arai City Жансугурова ул."), the first word before an area ("ул. Кирова Шмелев Лог
// мкр"), or two with a title ("Кабанбай батыра", "Толе би").
func (st *state) areaStreetBorder(a, b int, areaFirst, living bool) int {
	quoted := false
	for j := a; j <= b; j++ {
		quoted = quoted || st.isPunct(j, "\"«»")
	}
	// plain words after a word or, in a quoted name, after its quote: "ЖК «Мирас» Капал батыра ул."; a type
	// word inside the name does not start the street: "ЖК Самал Бульвар Тауке хана ул."
	var splits []int
	for j := a + 1; j <= b; j++ {
		if st.toks[j].kind == tWord && st.toks[j].mk == nil && st.toks[st.beforeQuotes(j-1)].kind == tWord && (!quoted || st.isPunct(j-1, "\"«»")) {
			splits = append(splits, j)
		}
	}
	if len(splits) == 0 {
		return -1
	}
	city := st.vals[City]
	streetOf := func(j int) (int, int) {
		if areaFirst {
			return j, b
		}
		return a, st.beforeQuotes(j - 1)
	}
	areaOf := func(j int) (int, int) {
		if areaFirst {
			return a, st.beforeQuotes(j - 1)
		}
		return j, b
	}
	streetAt := func(j, votes int) bool { sa, sb := streetOf(j); return st.streetVotes(city, sa, sb) >= votes }
	areaAt := func(j int) bool { aa, ab := areaOf(j); return st.knownArea(city, aa, ab) }
	byStreet := make([]int, len(splits)) // from the longest street half to the shortest
	for n := range splits {
		byStreet[n] = splits[n]
		if !areaFirst {
			byStreet[n] = splits[len(splits)-1-n]
		}
	}
	for _, j := range byStreet {
		if streetAt(j, 1) && areaAt(j) {
			return j
		}
	}
	for _, j := range byStreet {
		if streetAt(j, 3) {
			return j
		}
	}
	for n := len(byStreet) - 1; n >= 0; n-- {
		if areaAt(byStreet[n]) {
			return byStreet[n]
		}
	}
	for _, j := range byStreet {
		if streetAt(j, 1) {
			return j
		}
	}
	last := len(splits) - 1
	switch {
	case areaFirst && !living:
		return splits[0]
	case areaFirst:
		if last > 0 && streetTitles[st.toks[splits[last]].fold] {
			return splits[last-1]
		}
		return splits[last]
	case last > 0 && streetTitles[st.toks[splits[0]].fold]:
		return splits[1]
	}
	return splits[0]
}

// streetTitles follow a name in a street name: "Кабанбай батыра", "Абылай хана", "Толе би", "Жибек жолы".
var streetTitles = map[string]bool{}

func init() {
	for _, w := range []string{"батыр", "батыра", "би", "бия", "хан", "хана", "жырау", "сері", "ақын", "датқа", "жолы"} {
		streetTitles[text.Fold(w)] = true
	}
}

// beforeQuotes returns the last token at or before i that is not a quote.
func (st *state) beforeQuotes(i int) int {
	for i > 0 && st.isPunct(i, "\"«»") {
		i--
	}
	return i
}

// streetVotes returns the number of map segments and buildings of the street a..b in the city, 0 when unknown.
func (st *state) streetVotes(city string, a, b int) int {
	k, _ := dict.StreetKey(st.text(a, b))
	if s, ok := dict.OSM().Street(city, k, dict.AnyType); ok && k != "" {
		return s.Votes
	}
	return 0
}

// typeInName reports whether the street type word at k may be a part of a ЖК name ("Экспо Бульвар"):
// not улица and not a Kazakh type word, which always type the street before them.
func (st *state) typeInName(k int) bool {
	m := st.toks[k].mk
	return m != nil && m.Class == dict.Street && m.Canon != "улица" && !kazakhStreetPostfix[st.toks[k].fold]
}

// typedLater reports whether the plain words after the type word at k end at another, postfix, street
// type: in "ЖК Экспо Бульвар Сейфуллина ул. 44" the street is Сейфуллина, and Бульвар is in the ЖК name.
func (st *state) typedLater(k int) bool {
	j := k + 1 + st.toks[k].tail
	for st.isPunct(j, ".") {
		j++
	}
	n := 0
	for ; j < len(st.toks) && st.toks[j].kind == tWord && st.toks[j].mk == nil && !st.toks[j].used; j++ {
		n++
	}
	return n > 0 && st.postfixStreetAt(j)
}

// typesInLivingNames drops the street type of a word that ends a ЖК name written before its marker:
// "Экспо Бульвар ЖК, Мәңгілік Ел даң.", "«Экспо Бульвар» ЖК пр. Тауелсиздик", "Экспо Бульвар тұрғын үй
// кешені Габдуллин 33". The name must be quoted, or the marker Kazakh (it follows its name), or the marker
// not followed by a name of its own: in "Абая пр. ЖК Хайвил 5" пр. types Абая.
func (st *state) typesInLivingNames() {
	for k := 1; k+1 < len(st.toks); k++ {
		if !st.typeInName(k) || st.toks[k].used || st.toks[k-1].kind != tWord || st.toks[k-1].mk != nil {
			continue
		}
		j := k + 1 + st.toks[k].tail
		for st.isPunct(j, ".") {
			j++
		}
		quoted := st.isPunct(j, "\"")
		for st.isPunct(j, "\"") {
			j++
		}
		if j >= len(st.toks) || st.toks[j].mk == nil || st.toks[j].mk.Class != dict.Living || st.toks[j].used {
			continue
		}
		if quoted || postfixAreaMarkers[st.toks[j].fold] || !st.opensName(j) {
			st.toks[k].mk = nil
		}
	}
}

// opensName reports whether the marker word at i, after its dots, is followed by a plain word that it
// names: "ул. Толе би", "ул.садовая".
func (st *state) opensName(i int) bool {
	j := i + 1 + st.toks[i].tail
	for st.isPunct(j, ".") {
		j++
	}
	return j < len(st.toks) && st.toks[j].kind == tWord && st.toks[j].mk == nil && !st.toks[j].used
}

// typeAfterName reports whether the street type word at i surely types the name before it: a Kazakh one
// ("Сәтбаев көшесі"), or one followed by the house ("Сатпаева ул. д. 12", "Сатпаева ул 12"), unlike "ул. 1 Мая".
func (st *state) typeAfterName(i int) bool {
	if !st.postfixStreetAt(i) {
		return false
	}
	if kazakhStreetPostfix[st.toks[i].fold] {
		return true
	}
	j := i + 1 + st.toks[i].tail
	for st.isPunct(j, ".,") {
		j++
	}
	if st.houseAt(j) {
		return true
	}
	return j < len(st.toks) && st.toks[j].kind == tNum && !(j+1 < len(st.toks) && st.toks[j+1].kind == tWord && st.toks[j+1].mk == nil)
}

// postfixStreetAt reports whether a real street type word starts at i: "көшесі", "көш.", "ул.".
func (st *state) postfixStreetAt(i int) bool {
	if !(i < len(st.toks) && st.toks[i].kind == tWord && !st.toks[i].used && st.toks[i].mk != nil &&
		st.toks[i].mk.Class == dict.Street && dict.RealStreetTypes[st.toks[i].mk.Canon]) {
		return false
	}
	return !st.opensName(i) // "ул.садовая" opens a new name, so it is a prefix, not a postfix
}

func (st *state) street() {
	st.typesInLivingNames()
	var cands []cand
	covered := -1 // a type word inside a collected name is not a new street: "жк Экспо Бульвар"
	for i, t := range st.toks {
		if i <= covered || t.kind != tWord || t.used || t.mk == nil || (t.mk.Class != dict.Street && t.mk.Class != dict.Living) {
			continue
		}
		typ := t.mk.Canon
		c := cand{typ: typ, mk: i, isLiving: t.mk.Class == dict.Living}
		c.a, c.b = st.collectForward(i, typ)
		if !dict.RealStreetTypes[typ] {
			if ba, bb := st.collectBackward(i, typ); ba > covered && st.nameBefore(i, typ, c.a, c.b, ba, bb) {
				c.a, c.b = ba, bb
			}
		}
		if c.a < 0 {
			c.a, c.b = st.collectBackward(i, typ)
			if c.a > 0 && dict.RealStreetTypes[typ] && st.numberStartsName(c.a, c.b) {
				c.a-- // "1 Мая көшесі", "50 лет Октября улица"
			}
		}
		if c.a < 0 {
			continue
		}
		covered = max(covered, c.b)
		cands = append(cands, c)
	}
	st.splitShared(cands)

	var street, living *cand
	for i := range cands {
		c := &cands[i]
		if dict.RealStreetTypes[c.typ] && street == nil {
			street = c
		}
	}
	for i := range cands {
		c := &cands[i]
		if c == street {
			continue
		}
		if street == nil && !c.isLiving {
			street = c
		} else if living == nil {
			living = c
		}
	}

	untyped := false
	if living != nil {
		st.markUsed(min(living.mk, living.a), max(living.mk, living.b))
		if street == nil {
			untyped = st.untypedStreet()
		}
		if street == nil && !untyped && living.isLiving {
			street = living // ЖК without a street is the street: "Астана жк Северное сияние дом 7"
		} else {
			st.set(LivingArea, st.text(living.a, living.b), SourceString, living.a, living.b)
		}
	}
	if street != nil && living == nil && !untyped && !dict.RealStreetTypes[street.typ] {
		st.markUsed(min(street.mk, street.a), max(street.mk, street.b))
		if st.untypedStreet() { // "мкр Самал-2, Жолдасбекова 97"
			st.set(LivingArea, st.text(street.a, street.b), SourceString, street.a, street.b)
			return
		}
		for i := min(street.mk, street.a); i <= max(street.mk, street.b); i++ {
			st.toks[i].used = false
		}
	}
	if street != nil {
		if j := st.areaThenStreet(street); j > 0 { // "мкрн Баганашыл Муканова д 43"
			st.set(LivingArea, st.text(street.a, j-1), SourceString, street.a, j-1)
			st.set(StreetName, st.text(j, street.b), SourceString, j, street.b)
			st.markUsed(min(street.mk, street.a), max(street.mk, street.b))
			st.streetE = street.b + 1
			return
		}
		st.set(StreetType, street.typ, SourceString, street.mk, street.mk)
		st.set(StreetName, st.text(street.a, street.b), SourceString, street.a, street.b)
		st.markUsed(min(street.mk, street.a), max(street.mk, street.b))
		st.streetE = max(street.mk, street.b) + 1
		return
	}
	if !untyped {
		st.untypedStreet()
	}
}

// numberStartsName reports whether the number right before the street name a..b, read backward from its
// type, is the start of the name: a date or an anniversary ("1 Мая", "8 Марта", "50 лет Октября"), or a
// numbered street of the city's map, also one letter off ("1 Маяя көшесі"). A number after a unit word is
// the house or the flat: "үй 29 Кривенко көшесі".
func (st *state) numberStartsName(a, b int) bool {
	n := a - 1
	if !st.availNum(n) || st.toks[n].seg != st.toks[a].seg || st.toks[a].kind != tWord {
		return false
	}
	p := n - 1
	for st.isPunct(p, ".") {
		p--
	}
	if p >= 0 && st.toks[p].mk.IsUnit() {
		return false
	}
	if st.toks[a].fold == "лет" || a == b && monthsOf[st.toks[a].fold] {
		return true
	}
	city := st.vals[City]
	if st.streetVotes(city, n, b) > 0 {
		return true
	}
	k, _ := dict.StreetKey(st.text(a, b))
	var buf, kbuf [text.MaxFuzzyLen]rune
	r := text.ToRunes(k, kbuf[:0])
	if len(r) < 2 || len(r) >= text.MaxFuzzyLen-1 {
		return false
	}
	prefix := st.toks[n].fold + " "
	for key := range dict.OSM().Streets[city] {
		name, typ, _ := strings.Cut(key, "\x00")
		if typ == dict.AnyType && strings.HasPrefix(name, prefix) && text.DistanceLimit(r, text.ToRunes(name[len(prefix):], buf[:0]), 1) <= 1 {
			return true
		}
	}
	return false
}

// monthsOf are month names after a day: "1 Мая", "8 Марта".
var monthsOf = map[string]bool{}

func init() {
	for _, w := range []string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"} {
		monthsOf[text.Fold(w)] = true
	}
}

// untypedStreet finds words followed by a house number ("Кулманова 133", "Назарбаева8", "майлин д 10", "саина, 84").
func (st *state) untypedStreet() bool {
	for i := 0; i < len(st.toks); i++ {
		t := st.toks[i]
		if t.kind != tWord || t.used || t.mk != nil {
			continue
		}
		j := i
		for j+1 < len(st.toks) {
			n := st.toks[j+1]
			if n.kind == tWord && !n.used && n.mk == nil {
				j++
				continue
			}
			if n.kind == tPunct && (n.text == "-" || n.text == ".") && j+2 < len(st.toks) && st.toks[j+2].kind == tWord && !st.toks[j+2].used {
				j++
				continue
			}
			break
		}
		// "самал-2, 59": a number after a hyphen belongs to the name when the house number comes later
		if st.isPunct(j+1, "-") && glued(st.toks[j], st.toks[j+1]) && st.availNum(j+2) && glued(st.toks[j+1], st.toks[j+2]) &&
			st.laterNumberExists(j+2) {
			j += 2
		}
		h := j + 1
		for st.isPunct(h, ".") {
			h++
		}
		// a whole part of the line may be the street: "алматы, саина, 84"
		if st.isPunct(h, ",") && (i == 0 || st.toks[i-1].kind != tWord || st.toks[i-1].used) {
			h++
		}
		if st.availNum(h) || st.houseAt(h) {
			st.set(StreetName, st.text(i, j), SourceString, i, j)
			st.markUsed(i, j)
			st.streetE = j + 1
			return true
		}
		i = j
	}
	return false
}

// livingPart takes a separate part of plain words between the city and the street as the living area:
// "Караганда, Пришахтинск, ул. Ленина 5", "Астана, Нурсая, Сыганак 10". Without a city such a part may be
// the town itself, so it is left alone.
func (st *state) livingPart() {
	if _, ok := st.vals[LivingArea]; ok || st.vals[City] == "" || st.vals[StreetName] == "" || st.streetE <= 0 {
		return
	}
	streetSeg := st.toks[st.streetE-1].seg
	cityEnd := -1
	for _, sp := range st.spans {
		if sp.Field == City {
			cityEnd = max(cityEnd, sp.End)
		}
	}
	best := -1
	for seg := 0; seg < streetSeg; seg++ {
		a, b := st.segRange(seg)
		if a < 0 || b-a > 4 || st.c.Start[st.toks[a].s] < cityEnd {
			continue
		}
		words := 0
		for i := a; i <= b; i++ {
			t := st.toks[i]
			switch {
			case t.kind == tWord && !t.used && t.mk == nil:
				words++
			case t.kind == tNum && !t.used && i > a+1 && st.isPunct(i-1, "-") && glued(st.toks[i-1], t): // "Самал-2"
			case st.isPunct(i, "-\"«»"):
			default:
				words = -100
			}
		}
		if words > 0 && words <= 3 {
			best = seg
		}
	}
	if best < 0 {
		return
	}
	a, b := st.segRange(best)
	for st.isPunct(a, "\"«»") {
		a++
	}
	for st.isPunct(b, "\"«»") {
		b--
	}
	st.set(LivingArea, st.text(a, b), SourceString, a, b)
	st.markUsed(a, b)
}
