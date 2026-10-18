package parser

import (
	"strings"

	"github.com/jana-address/jana-address-api/internal/parser/dict"
	"github.com/jana-address/jana-address-api/internal/parser/text"
)

// kazakhPostfix markers always follow the name, unlike Russian "район" that can stand on either side.
var kazakhPostfix = map[string]bool{text.Fold("ауданы"): true, text.Fold("аудан"): true, text.Fold("ауд"): true, text.Fold("қаласы"): true, text.Fold("облысы"): true}

// postfixAdmin markers usually follow the name: "Алматы қаласы", "Тұран ауданы", "Турксибский район".
var postfixAdmin = map[string]bool{}

func init() {
	for _, w := range []string{"қаласы", "қалас", "ауданы", "аудан", "облысы", "район", "р-н", "рн", "область", "обл", "обл-ть", "ауылы", "ауыл", "ауд", "қ", "қала"} {
		postfixAdmin[text.Fold(w)] = true
	}
}

func (st *state) keyOf(a, b int) string {
	var parts []string
	for i := a; i <= b; i++ {
		if st.toks[i].kind == tWord {
			parts = append(parts, st.toks[i].fold)
		}
	}
	return strings.Join(parts, " ")
}

// wordRun returns consecutive unmarked words next to the marker m in direction dir (+1/-1).
func (st *state) wordRun(m, dir int) (int, int) {
	j := m + dir
	for dir > 0 && st.isPunct(j, ".") {
		j++
	}
	a, b := -1, -1
	for k := j; k >= 0 && k < len(st.toks); k += dir {
		t := st.toks[k]
		if dir > 0 && a >= 0 && st.isPunct(k, ".") && st.toks[k-1].fold == "им" {
			continue // "район им. Казыбек би"
		}
		if t.kind != tWord || t.used || t.mk != nil || t.seg != st.toks[m].seg {
			break
		}
		if dir > 0 && a >= 0 && st.postfixStreetAt(k+1) {
			break
		}
		if a < 0 {
			a, b = k, k
		}
		if dir > 0 {
			b = k
		} else {
			a = k
		}
	}
	return a, b
}

// adminName picks the name of an admin marker: the longest directory match next to it,
// otherwise a single word when more text follows ("г. Арысь Абая 10" -> "Арысь").
func (st *state) adminName(m int) (int, int) {
	class := st.toks[m].mk.Class
	fa, fb := st.wordRun(m, +1)
	ba, bb := st.wordRun(m, -1)
	gluedAfter := m > 0 && st.toks[m-1].kind == tWord && glued(st.toks[m-1], st.toks[m])
	backward := ba >= 0 && (postfixAdmin[st.toks[m].fold] || fa < 0 || gluedAfter)
	fwdKnown := func() bool {
		for b := fb; fa >= 0 && b >= fa; b-- {
			if st.d.AdminKnown(class, st.keyOf(fa, b)) {
				return true
			}
		}
		return false
	}
	if backward {
		for a := ba; a <= bb; a++ {
			k := st.keyOf(a, bb)
			if _, city := st.d.Cities[k]; city && class == dict.District && !kazakhPostfix[st.toks[m].fold] && fwdKnown() {
				break
			}
			if st.d.AdminKnown(class, k) {
				return a, bb
			}
		}
	}
	if fa >= 0 { // a known name after the marker wins over an unknown one before it: "Астана р-н Нура"
		for b := fb; b >= fa; b-- {
			if st.d.AdminKnown(class, st.keyOf(fa, b)) {
				return fa, b
			}
		}
	}
	if backward {
		if ba == 0 || st.toks[ba-1].kind == tPunct {
			return ba, bb
		}
		return bb, bb
	}
	if fa < 0 {
		return -1, -1
	}
	if fb > fa && st.availNum(fb+1) {
		return fa, fa
	}
	return fa, fb
}

func (st *state) admin() {
	// 1. explicit markers: "г. Алматы", "Алматы қаласы", "Тұран ауданы", "Карагандинская обл"
	for i := range st.toks {
		t := st.toks[i]
		if t.kind != tWord || t.used || !t.mk.IsAdmin() {
			continue
		}
		a, b := st.adminName(i)
		if a < 0 {
			continue
		}
		k, name := st.keyOf(a, b), st.text(a, b)
		switch t.mk.Class {
		case dict.Region:
			if r, ok := st.d.Regions[k]; ok {
				st.set(Province, r, SourceString, a, b)
				st.writtenRegion(r, k, name, t.fold)
			} else {
				st.set(Province, name, SourceString, a, b)
			}
		case dict.District:
			st.setDistrict(k, name, a, b)
		default:
			st.setCity(k, name, a, b, t.mk.Class == dict.Settlement)
		}
		st.markUsed(min(i, a), max(i, b))
	}

	maxSeg := 0
	for _, t := range st.toks {
		maxSeg = max(maxSeg, t.seg)
	}
	for seg := 0; seg <= maxSeg; seg++ {
		a, b := st.segRange(seg)
		if a < 0 {
			continue
		}
		// skip leading postcodes: "050000 Алматы ул Абая 10"
		for a <= b && (st.toks[a].used || (st.toks[a].kind == tNum && st.toks[a].ignore) || st.isPunct(a, ".")) {
			a++
		}
		if a > b {
			continue
		}
		clean := st.toks[a].kind == tWord // only unmarked words, no numbers
		for i := a; i <= b; i++ {
			t := st.toks[i]
			if t.used || t.kind == tNum || (t.kind == tWord && t.mk != nil) {
				clean = false
			}
		}
		k := st.keyOf(a, b)
		switch {
		// 2. a segment that is just a known name: "Алматы", "Турксибский"
		case clean && (st.d.Cities[k] != dict.Place{} || ((seg == 0 || st.vals[City] == "" && st.opensLine(a)) && st.d.AdminKnown(dict.City, k) && (st.typoCityBefore(b) || a == b && st.typoCityOpening(a)))): // close spelling only where the line opens, maybe after the region
			st.setCity(k, st.text(a, b), a, b, false)
			st.markUsed(a, b)
		// 2b. a town of the classifier as a part of its own before more words: "Жаркент, ул. Абая 5".
		// Only towns: thousands of villages are called like streets ("Абай", "Достык").
		case clean && st.vals[City] == "" && st.townBefore(k, b):
			st.setCity(k, st.text(a, b), a, b, false)
			st.markUsed(a, b)
		// 2c. a living area of one city: "Пришахтинск, ул. Ленина 5" is in Караганда
		case clean && st.vals[LivingArea] == "" && st.livingArea(k, a, b):
			st.markUsed(a, b)
		case clean && st.d.Regions[k] != "" && (len([]rune(k)) == 3 || strings.HasSuffix(k, "ская")):
			st.set(Province, st.d.Regions[k], SourceString, a, b)
			st.writtenRegion(st.d.Regions[k], k, st.text(a, b), "")
			st.markUsed(a, b)
		case clean && st.vals[City] == "" && len(st.d.Settlements[k]) > 0 &&
			(st.writtenValue(Province) != "" || st.writtenValue(District) != "") && !st.houseNext(b):
			st.setCity(k, st.text(a, b), a, b, true)
			st.markUsed(a, b)
		// a rayon adjective after a city with rayons of its own is one of them; another one is a street:
		// "г. Шымкент, Сайрамский, д.117" is the street, "Шымкент, Енбекшинский район" a rayon of the city
		case clean && strings.HasSuffix(k, "ий") && st.d.AdminKnown(dict.District, k) && st.cityRayon(k):
			st.setDistrict(k, st.text(a, b), a, b)
			st.markUsed(a, b)
		default:
			// a region by its three letters opens the part: "ВКО ул. Абая 6", "зко мкр Кунаева"
			if t := st.toks[a]; t.kind == tWord && t.mk == nil && a < b && len([]rune(t.fold)) == 3 && st.d.Regions[t.fold] != "" && st.vals[Province] == "" {
				st.set(Province, st.d.Regions[t.fold], SourceString, a, a)
				st.writtenRegion(st.d.Regions[t.fold], t.fold, t.text, "")
				st.markUsed(a, a)
				a++
			}
			// 3. a known city inside a street segment: "Алматы Сатпаева 5", "050000 Алматы ул Абая 10"
			st.cityPair(a, b)
			for i := a; i <= b; i++ {
				t := st.toks[i]
				if t.kind != tWord || t.used || t.mk != nil || (!st.d.AdminKnown(dict.City, t.fold) && i != 0) {
					continue
				}
				areaNext := i+2 <= b && st.toks[i+1].kind == tNum && st.toks[i+2].mk != nil && dict.NumberedAreaTypes[st.toks[i+2].mk.Canon]
				_, exact := st.d.Cities[t.fold]
				if !exact && !areaNext && !st.typoCityBefore(i) && !st.typoCityOpening(i) {
					continue
				}
				if !exact && st.houseNext(i) && !st.typoCityOpening(i) {
					continue // a town or village before the house is the street: "Сауран д 104 кв 12"
				}
				if st.shortTypo(i) && (i > a && st.toks[i-1].kind == tWord && !st.toks[i-1].used && st.toks[i-1].mk == nil || st.exactCityElsewhere(i)) {
					continue // "ул. Акын Сара" is a street, "Esim khana 72, Turkestan" names its city
				}
				if !st.d.AdminKnown(dict.City, t.fold) && (i != 0 || !st.townBefore(t.fold, i)) {
					continue // a town of the classifier only opens the line: "Жаркент ул. Абая 5"
				}
				p := i - 1
				for st.isPunct(p, ".") {
					p--
				}
				if p >= 0 && st.toks[p].mk != nil && st.toks[p].mk.Class == dict.Street || st.typeAfterName(i+1) {
					continue // "ул. Астана", "Сәтбаев көшесі" and "Сатпаева ул. д. 12" are streets, not the city
				}
				if cur, set := st.vals[City]; set {
					// the city written twice: "алматы алматы жандосова 58", "Актобе Актобе 5 мкр", "Шымкент Шымкент «Бахыт» жк"
					// and three times: "туркестан / түркістан туркестан кожанова 180"
					if pl, _ := st.d.LookupCity(t.fold); pl.Name == cur && i < b && (st.toks[i+1].kind == tWord || areaNext || st.isPunct(i+1, "\"«»")) {
						st.toks[i].used = true
						continue
					}
					break
				}
				st.setCity(t.fold, t.text, i, i, false)
				st.toks[i].used = true
			}
		}
	}
	st.cityPairAtEnd()
}

// houseNext reports whether the house follows token i, maybe after a comma: a house number that does not
// start a numbered area, or a house marker. A town or village so placed is the street named after it:
// "Сауран д 104", "Торайгырова, 29/11".
func (st *state) houseNext(i int) bool {
	j := i + 1
	for st.isPunct(j, ",.") {
		j++
	}
	if j >= len(st.toks) {
		return false
	}
	t := st.toks[j]
	if t.kind == tWord {
		return t.mk != nil && t.mk.Class == dict.House
	}
	if t.kind != tNum || t.ignore {
		return false
	}
	return j+1 >= len(st.toks) || st.toks[j+1].mk == nil || !dict.NumberedAreaTypes[st.toks[j+1].mk.Canon]
}

// shortTypo reports whether token i is a word of four letters read as a large city only by its close
// spelling: "тарз", "смей".
func (st *state) shortTypo(i int) bool {
	t := st.toks[i]
	if _, exact := st.d.Cities[t.fold]; exact || len([]rune(t.fold)) != 4 {
		return false
	}
	_, ok := st.d.LookupCity(t.fold)
	return ok
}

// exactCityElsewhere reports whether a word other than token i names a city of the hand table exactly.
func (st *state) exactCityElsewhere(i int) bool {
	for j, t := range st.toks {
		if j == i || t.kind != tWord {
			continue
		}
		if _, ok := st.d.Cities[t.fold]; ok {
			return true
		}
		if j+1 < len(st.toks) && st.toks[j+1].kind == tWord && j+1 != i {
			if _, ok := st.d.Cities[t.fold+" "+st.toks[j+1].fold]; ok {
				return true
			}
		}
	}
	return false
}

// cityPairAtEnd reads a city of two words, also one letter off, that closes the line after the street and
// the numbers: "ул. Айтеке би 9 Отеген батыр", "Школьная д. 47 өтеген батыр", "Абая 5 отеген батырр".
func (st *state) cityPairAtEnd() {
	b := len(st.toks) - 1
	for b >= 0 && (st.toks[b].kind == tPunct || st.toks[b].used && st.vals[Province] != "") {
		b-- // and before the region read already: "Наурыз 62, 3 эт Өтеген батыр алматы облысы"
	}
	a := b - 1
	if st.vals[City] != "" || a < 1 {
		return
	}
	for i := a; i <= b; i++ {
		if t := st.toks[i]; t.kind != tWord || t.used || t.mk != nil {
			return
		}
	}
	if p := st.toks[a-1]; p.kind != tNum && p.kind != tPunct && !p.mk.IsUnit() {
		return // only after the numbers, a comma or a unit word: "ул. Отеген батыр" stays a street
	}
	p := a - 1
	for p > 0 && st.isPunct(p, ".") {
		p--
	}
	if m := st.toks[p].mk; m != nil && (m.Class == dict.Street || m.Class == dict.Living) {
		return
	}
	k := st.keyOf(a, b)
	if _, ok := st.d.LookupCity(k); !ok {
		return
	}
	st.setCity(k, st.text(a, b), a, b, false)
	st.markUsed(a, b)
}

// cityPair reads a city of two words that opens part a..b of the line before more words:
// "Отеген батыр ул. Центральная 105", "новый узень абая 5", but not before a postfix street type.
// A close spelling counts only where the line opens, after a postcode or a region ("отеген баатыр",
// "Алматинская обл Отеген баатыр Абая 5"), and the name written twice
// is read once ("Отеген батыр Отеген батыр …").
func (st *state) cityPair(a, b int) {
	if cur := st.vals[City]; cur != "" {
		// the city of two words written again: "с. Отеген батыр, Отеген батыр Жибек жолы 51"
		if a+2 <= b && st.plainWords(a, a+1) && st.toks[a+2].kind == tWord && st.d.Cities[st.keyOf(a, a+1)].Name == cur {
			st.markUsed(a, a+1)
		}
		return
	}
	if a+2 > b || !st.typoCityBefore(a+1) || st.postfixStreetAt(a+2) || !st.plainWords(a, a+1) {
		return // "Өтеген батыр көшесі 25" is a street
	}
	k := st.keyOf(a, a+1)
	if st.d.Cities[k] == (dict.Place{}) {
		if _, ok := st.d.LookupCity(k); !ok || !st.opensLine(a) {
			return
		}
	}
	st.setCity(k, st.text(a, a+1), a, a+1, false)
	st.markUsed(a, a+1)
	if a+3 <= b && st.toks[a+2].kind == tWord && st.toks[a+3].kind == tWord && st.keyOf(a+2, a+3) == k {
		st.markUsed(a+2, a+3)
	}
}

// plainWords reports whether the tokens a..b are unused words without a marker.
func (st *state) plainWords(a, b int) bool {
	for i := a; i <= b; i++ {
		if t := st.toks[i]; t.kind != tWord || t.used || t.mk != nil {
			return false
		}
	}
	return true
}

// livingArea stores a part of the line that is a known living area of one city consistent with the
// written city and region; the city and the region come from the directory. A street must follow:
// in "Караганда, Майкудук, д.30" the area is the street name, as the case defines living_area.
func (st *state) livingArea(k string, a, b int) bool {
	j := b + 1
	for st.isPunct(j, ",;()/") {
		j++
	}
	if j >= len(st.toks) {
		return false
	}
	switch t := st.toks[j]; {
	case t.kind == tWord && !t.mk.IsUnit():
	case t.kind == tNum && j+1 < len(st.toks) && st.toks[j+1].mk != nil && st.toks[j+1].mk.Class == dict.Street: // "21 мкр"
	default:
		return false
	}
	var found dict.Area
	n := 0
	for _, ar := range st.d.Areas[k] {
		if city := st.vals[City]; city != "" && city != ar.City {
			continue
		}
		if prov := st.writtenValue(Province); prov != "" && prov != st.d.Cities[text.Key(ar.City)].Province {
			continue
		}
		found = ar
		n++
	}
	if n != 1 {
		return false
	}
	st.set(LivingArea, st.text(a, b), SourceString, a, b)
	if p, ok := st.d.Cities[text.Key(found.City)]; ok {
		st.setDir(City, p.Name, LevelArea)
		st.setDir(Province, p.Province, LevelArea)
	}
	st.setDir(District, found.District, LevelArea)
	return true
}

// townBefore reports whether k, ending at token i, is a town of the classifier followed by a word, not by
// a house number, a street type or a house marker: "Жаркент, ул. Абая 5" is the town, "Абай, 12",
// "Абай даңғылы 10" and "Абай д 44" stay streets.
// A town named like a city district ("Алатау") is left to the district and living-area rules.
func (st *state) townBefore(k string, i int) bool {
	if _, ok := st.d.Town(k); !ok || i+1 >= len(st.toks) || !st.typoCityBefore(i) || st.postfixStreetAt(i+1) {
		return false
	}
	if d, ok := st.d.Districts[k]; ok && d.City != "" {
		return false
	}
	j := i + 1
	for st.isPunct(j, ",;()/") {
		j++
	}
	return !st.toks[j].mk.IsUnit()
}

// typoCityBefore reports whether a close spelling of a city may end at token i: only before another word
// or at the end ("алматыы абая 10", "алматыы, абая 10", "шымкнет квартира 120" are the city), never before
// a number ("Сатпаева 22" and "Сатпаева, 22" are a street one letter from the town Сатпаев).
func (st *state) typoCityBefore(i int) bool {
	j := i + 1
	for st.isPunct(j, ",;()/\"«»") {
		j++ // a comma between two parts, a quoted name: "алматыы «Нурлы Тау» жк"
	}
	return j == len(st.toks) || st.toks[j].kind == tWord
}

// cityRayon reports whether the rayon name k may be read with the city of the line: no city yet, a city
// without rayons in the tables, or one of its own rayons.
func (st *state) cityRayon(k string) bool {
	ds, ok := st.d.CityDistricts[st.vals[City]]
	if st.vals[City] == "" || !ok {
		return true
	}
	_, own := ds[k]
	return own
}

// opensLine reports whether token i opens the line but for a postcode and the parts already read, such as
// the region: "Алматинская обл, Отеген баатыр, Абая 5".
func (st *state) opensLine(i int) bool {
	for j := 0; j < i; j++ {
		if p := st.toks[j]; p.kind == tWord && !p.used || p.kind == tNum && !p.ignore {
			return false
		}
	}
	return true
}

// typoCityOpening reports whether a close spelling of a city at token i, before a number, is the city: the
// word opens the line, no city of the map has a street so written, and a street or area type comes later:
// "Асьана 167 кв, ул. Калдаякова", "Влматы, 8 мкр, 17". "Абая 10 кв 5" and "Сатпаева 22, мкр Самал 5"
// stay streets.
func (st *state) typoCityOpening(i int) bool {
	t := st.toks[i]
	if t.kind != tWord || t.mk != nil || dict.OSM().KnownAnywhere(lookupKey(t.text)) || dict.OSM().KnownAnywhere(t.fold) {
		return false
	}
	if !st.opensLine(i) {
		return false
	}
	for j := i + 1; j < len(st.toks); j++ {
		if m := st.toks[j].mk; m != nil && (m.Class == dict.Street || m.Class == dict.Living) {
			return true
		}
	}
	return false
}

// setCity stores a city or village. The hand table of large places goes first unless the line names
// another region or rayon; then the classifier's settlements, with the written rayon or region choosing
// among namesakes; a typo match comes last and, after "с.", "п.", "ауылы", only for names the
// classifier does not know ("с. Алмалы" must not become Алматы).
func (st *state) setCity(k, name string, a, b int, village bool) {
	use := func(p dict.Place) {
		st.set(City, p.Name, SourceString, a, b)
		st.setDir(Province, p.Province, LevelSettlement)
		st.setDir(District, p.District, LevelSettlement)
	}
	consistent := func(p dict.Place) bool {
		prov, dist := st.writtenValue(Province), st.writtenValue(District)
		return (prov == "" || p.Province == prov) && (dist == "" || p.District == "" || p.District == dist)
	}
	if hand, ok := st.d.Cities[k]; ok && consistent(hand) {
		// a written rayon picks its own village over a town of another region: "Мамлютский район, Щучье"
		// is a village there, not Щучинск; in the same region the hand table keeps its spelling
		if p, own := st.settlement(k); own && hand.District == "" && st.writtenValue(District) != "" && p.Province != hand.Province {
			use(p)
		} else {
			use(hand)
		}
		return
	}
	ps := st.d.Settlements[k]
	if p, ok := st.settlement(k); ok {
		use(p)
		return
	}
	if p, ok := st.d.Town(k); ok && !village && consistent(p) { // "г. Шу": the town, not the village namesake
		use(p)
		return
	}
	if len(ps) == 0 || !village {
		if p, ok := st.d.LookupCity(k); ok && consistent(p) { // a close spelling must fit the written region
			use(p)
			return
		}
	}
	if len(ps) > 0 && sameName(ps) { // namesakes in several places: the name is certain, the region only if they share it
		st.set(City, ps[0].Name, SourceString, a, b)
		if prov := shared(ps, func(p dict.Place) string { return p.Province }); prov != "" {
			st.setDir(Province, prov, LevelSettlement) // "с. Баскудук": two villages, both in Мангистауская область
			st.setDir(District, shared(ps, func(p dict.Place) string { return p.District }), LevelSettlement)
		}
		return
	}
	st.set(City, text.RuSpelling(name), SourceString, a, b)
}

// shared returns the value every place has, or "".
func shared(ps []dict.Place, value func(dict.Place) string) string {
	v := value(ps[0])
	for _, p := range ps[1:] {
		if value(p) != v {
			return ""
		}
	}
	return v
}

func sameName(ps []dict.Place) bool {
	for _, p := range ps[1:] {
		if p.Name != ps[0].Name {
			return false
		}
	}
	return true
}

// setDistrict stores a district written in the line. The output keeps it as written, without the
// word "район" ("Сарыарка ауданы" -> "Сарыарка"); the directory name is used while parsing, so a
// written rayon still picks its own village and a city.
func (st *state) setDistrict(k, name string, a, b int) {
	for a < b && (st.toks[a].fold == "им" || st.toks[a].fold == "имени") { // "им. Казыбек би" -> "Казыбек би"
		a++
		for a < b && st.isPunct(a, ".") {
			a++
		}
		k, name = st.keyOf(a, b), st.text(a, b)
	}
	d, ok := st.d.District(st.vals[City], k)
	if !ok {
		st.set(District, name, SourceString, a, b)
		return
	}
	if st.src[District] != SourceString {
		st.written[District] = name
	}
	st.set(District, d.Name, SourceString, a, b)
	// "Абайский", "Есильский" are city districts of Шымкент and Астана and also rural rayons in other
	// regions: the city comes from the district only when the line names no other region
	if prov := st.writtenValue(Province); prov == "" || prov == d.Province {
		st.setDir(City, d.City, LevelDistrict)
		st.setDir(Province, d.Province, LevelDistrict)
	}
}

// settlement resolves a town or village name; province and district written in the line pick one
// of several namesakes. It returns false when the name is unknown or still ambiguous.
func (st *state) settlement(k string) (dict.Place, bool) {
	ps := st.d.Settlements[k]
	if len(ps) == 0 {
		return dict.Place{}, false
	}
	prov, dist := st.writtenValue(Province), st.writtenValue(District)
	var match []dict.Place
	for _, p := range ps {
		if (prov == "" || p.Province == prov) && (dist == "" || p.District == dist) {
			match = append(match, p)
		}
	}
	if len(match) == 1 {
		return match[0], true
	}
	return dict.Place{}, false
}

// writtenRegion keeps the Russian form of a new region as written: "Абайская область", "область Абай"
// and "область Жетысу" stay so; the Kazakh "Абай облысы" gives the official "область Абай". The old
// regions have one Russian form, the canonical one.
func (st *state) writtenRegion(canon, k, name, marker string) {
	if !strings.HasPrefix(canon, "область ") || st.vals[Province] != canon || kazakhPostfix[marker] {
		return
	}
	if strings.HasSuffix(k, "ская") {
		st.written[Province] = name + " область"
	} else if marker != "" {
		st.written[Province] = "область " + name
	}
}

// writtenForms puts the written forms of province and district into the output once parsing is done.
func (st *state) writtenForms() {
	for f, v := range st.written {
		if st.src[f] == SourceString {
			st.vals[f] = v
		}
	}
}

// writtenValue returns a field value only when it came from the line itself.
func (st *state) writtenValue(f Field) string {
	if st.src[f] != SourceString {
		return ""
	}
	return st.vals[f]
}
