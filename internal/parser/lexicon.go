package parser

import "strings"

type mclass uint8

const (
	mHouse mclass = iota + 1
	mApartment
	mEntrance
	mFloor
	mStreet     // street type; canon is the output word (улица, проспект, ...)
	mLiving     // residential complex (ЖК)
	mCity       // город, қаласы
	mSettlement // пос., село, ауыл
	mDistrict   // район, ауданы
	mRegion     // область, облысы
	mLandmark   // магазин, ТЦ, напротив, ...
	mBlock      // корпус, блок, строение: consumes its number, not an output field
	mNoise      // Казахстан, РК: ignored words
)

type marker struct {
	class mclass
	canon string
}

func (m *marker) isUnit() bool {
	return m != nil && (m.class == mHouse || m.class == mApartment || m.class == mEntrance || m.class == mFloor || m.class == mBlock)
}

func (m *marker) isAdmin() bool {
	return m != nil && (m.class == mCity || m.class == mSettlement || m.class == mDistrict || m.class == mRegion)
}

// markers maps folded word forms to their meaning (Russian, Kazakh, Latin, common typos).
var markers = map[string]*marker{}

func addMarkers(class mclass, canon string, forms ...string) {
	m := &marker{class: class, canon: canon}
	for _, f := range forms {
		markers[fold(f)] = m
	}
}

func init() {
	addMarkers(mHouse, "", "д", "дом", "домм", "дм", "үй", "уй", "house", "dom", "d")
	addMarkers(mApartment, "", "кв", "квартира", "кварт", "квартра", "кв-ра", "пәтер", "flat", "kv", "квю", "apt", "apartment", "оф", "офис", "office", "ofis")
	addMarkers(mEntrance, "", "подъезд", "подьезд", "подезд", "падезд", "под", "пд", "кіреберіс", "entrance", "ent", "pod", "podezd", "podyezd")
	addMarkers(mFloor, "", "этаж", "етаж", "этж", "етж", "эт", "қабат", "floor", "fl", "etazh", "etaj", "et")

	addMarkers(mStreet, "улица", "ул", "улица", "улца", "улиц", "уица", "ulitsa", "ulica", "көшесі", "көш", "street", "st", "ul", "ulica")
	addMarkers(mStreet, "проспект", "пр", "пр-т", "просп", "проспект", "даңғылы", "даңғ", "avenue", "ave", "pr", "prospekt", "prosp", "pr-t", "пр-кт", "dangyly", "даң")
	addMarkers(mStreet, "микрорайон", "мкр", "мкп", "мкрн", "мкр-н", "м-н", "мик-н", "микрорайон", "mkr", "mkrn", "mikrorayon", "mikroraion", "microdistrict")
	addMarkers(mStreet, "квартал", "квартал", "кв-л", "квл", "kvartal")
	addMarkers(mStreet, "переулок", "пер", "переулок", "пер-к", "тұйық", "pereulok", "per")
	addMarkers(mStreet, "бульвар", "б-р", "бульвар", "бул", "bulvar", "bul")
	addMarkers(mStreet, "шоссе", "шоссе", "ш", "shosse")
	addMarkers(mStreet, "трасса", "трасса")
	addMarkers(mLiving, "жилой комплекс", "жк", "zhk", "jk")

	addMarkers(mCity, "", "г", "город", "гор", "қаласы", "қ", "kalasy")
	addMarkers(mSettlement, "", "пос", "поселок", "пгт", "село", "ауыл", "аул", "ауылы")
	addMarkers(mDistrict, "", "район", "р-н", "рн", "ауданы", "аудан", "ауд")
	addMarkers(mRegion, "", "область", "обл", "облысы", "обл-ть")

	addMarkers(mBlock, "", "корпус", "корп", "блок", "строение", "стр", "block", "korp", "korpus", "blok")
	addMarkers(mNoise, "", "казахстан", "қазақстан", "рк", "республика", "kz", "kazakhstan")
	addMarkers(mLandmark, "", "магазин", "маг", "тц", "трц", "тд", "рынок", "базар", "школа", "мектеп", "садик",
		"напротив", "возле", "около", "рядом", "остановка", "аялдама", "qazpost", "казпочта", "аптека", "кафе",
		"азс", "бц", "супермаркет", "маркет")

	initPlaces()
}

// fuzzyForms are long marker words that are often mistyped ("подлъезд", "кваритра", "микрорайно").
var fuzzyForms = []string{"подъезд", "квартира", "бульвар", "корпус", "микрорайон", "проспект", "переулок", "улица", "квартал",
	"город", "район", "область", "облысы", "қаласы", "ауданы", "көшесі", "даңғылы", "этаж", "магазин", "поселок"}

// fuzzyMarker finds a marker within a small edit distance: 1 for words of 5–7 letters, 2 for longer ones.
func fuzzyMarker(f string) *marker {
	r := []rune(f)
	if len(r) < 5 {
		return nil
	}
	limit := 1
	if len(r) >= 8 {
		limit = 2
	}
	var best *marker
	bestD := limit + 1
	for _, form := range fuzzyForms {
		ff := fold(form)
		if d := osaDistance(r, []rune(ff)); d < bestD {
			best, bestD = markers[ff], d
		} else if d == bestD && best != nil && markers[ff] != best {
			best = nil // ambiguous between two markers
		}
	}
	return best
}

var kkToRu = strings.NewReplacer("Ә", "А", "ә", "а", "Ғ", "Г", "ғ", "г", "Қ", "К", "қ", "к", "Ң", "Н", "ң", "н",
	"Ө", "О", "ө", "о", "Ұ", "У", "ұ", "у", "Ү", "У", "ү", "у", "Һ", "Х", "һ", "х", "І", "И", "і", "и")

// ruSpelling replaces Kazakh letters with Russian ones, keeping the case.
func ruSpelling(s string) string { return kkToRu.Replace(s) }

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

// lookupCity finds a settlement by its exact key or, failing that, by a unique close spelling ("алматыы").
func lookupCity(k string) (place, bool) {
	if p, ok := cities[k]; ok {
		return p, true
	}
	r := []rune(k)
	limit := fuzzyLimit(len(r))
	if limit == 0 {
		return place{}, false
	}
	var best place
	bestD, ambiguous := limit+1, false
	for ck, p := range cities {
		d := osaDistance(r, []rune(ck))
		switch {
		case d < bestD:
			best, bestD, ambiguous = p, d, false
		case d == bestD && p.name != best.name:
			ambiguous = true
		}
	}
	if bestD <= limit && !ambiguous {
		return best, true
	}
	return place{}, false
}

// osaDistance is the optimal string alignment (Damerau-Levenshtein with adjacent transpositions).
func osaDistance(a, b []rune) int {
	if d := len(a) - len(b); d > 2 || d < -2 {
		return 3
	}
	prev2 := make([]int, len(b)+1)
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				cur[j] = min(cur[j], prev2[j-2]+1)
			}
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(b)]
}

// realStreetTypes are street types that can coexist with a microdistrict (which then becomes living_area).
var realStreetTypes = map[string]bool{"улица": true, "проспект": true, "переулок": true, "бульвар": true, "шоссе": true, "трасса": true}

// numberedAreaTypes may carry a number inside the name: "Аксай 3а", "Нурсити 2", "66 квартал".
var numberedAreaTypes = map[string]bool{"микрорайон": true, "квартал": true, "жилой комплекс": true}

// ---------- small built-in directory (demo; the full one is built from KATO + OSM) ----------

type place struct {
	name     string
	province string
	district string // known rayon for small settlements
}

type districtInfo struct {
	name     string
	city     string // empty for rural rayons
	province string
}

var (
	cities    = map[string]place{}
	regions   = map[string]string{}
	districts = map[string]districtInfo{}
	// microdistrict (first word, folded) -> district of Almaty
	almatyAreas = map[string]string{}
)

func key(s string) string { return strings.Join(strings.Fields(fold(s)), " ") }

func addCity(name, province string, aliases ...string) {
	p := place{name: name, province: province}
	cities[key(name)] = p
	for _, a := range aliases {
		cities[key(a)] = p
	}
}

func addRegion(name string, keys ...string) {
	for _, k := range keys {
		regions[key(k)] = name
	}
}

func addDistrict(name, city, province string, keys ...string) {
	d := districtInfo{name: name, city: city, province: province}
	districts[key(name)] = d
	for _, k := range keys {
		districts[key(k)] = d
	}
}

func initPlaces() {
	const (
		akm = "Акмолинская область"
		akt = "Актюбинская область"
		alm = "Алматинская область"
		atr = "Атырауская область"
		vko = "Восточно-Казахстанская область"
		zhm = "Жамбылская область"
		zko = "Западно-Казахстанская область"
		krg = "Карагандинская область"
		kst = "Костанайская область"
		kzo = "Кызылординская область"
		mng = "Мангистауская область"
		pvl = "Павлодарская область"
		sko = "Северо-Казахстанская область"
		trk = "Туркестанская область"
		aba = "область Абай"
		zht = "область Жетісу"
		ult = "область Ұлытау"
	)
	addRegion(akm, "акмолинская", "ақмола")
	addRegion(akt, "актюбинская", "ақтөбе")
	addRegion(alm, "алматинская", "алматы")
	addRegion(atr, "атырауская", "атырау")
	addRegion(vko, "восточно-казахстанская", "шығыс қазақстан", "вко")
	addRegion(zhm, "жамбылская", "жамбыл")
	addRegion(zko, "западно-казахстанская", "батыс қазақстан", "зко")
	addRegion(krg, "карагандинская", "қарағанды")
	addRegion(kst, "костанайская", "қостанай")
	addRegion(kzo, "кызылординская", "қызылорда")
	addRegion(mng, "мангистауская", "мангыстауская", "маңғыстау")
	addRegion(pvl, "павлодарская", "павлодар")
	addRegion(sko, "северо-казахстанская", "солтүстік қазақстан", "ско")
	addRegion(trk, "туркестанская", "түркістан", "юко", "южно-казахстанская")
	addRegion(aba, "абай", "абайская")
	addRegion(zht, "жетісу", "жетысу", "жетысуская")
	addRegion(ult, "ұлытау", "улытауская")

	addCity("Алматы", "Алматы", "алма-ата", "алмата")
	addCity("Астана", "Астана", "нур-султан", "нұр-сұлтан", "нурсултан", "акмола", "целиноград")
	addCity("Шымкент", "Шымкент", "чимкент")
	addCity("Кокшетау", akm, "көкшетау", "кокчетав")
	addCity("Степногорск", akm)
	addCity("Актобе", akt, "ақтөбе", "актюбинск")
	addCity("Конаев", alm, "қонаев", "капшагай", "қапшағай")
	addCity("Талгар", alm)
	addCity("Каскелен", alm, "қаскелең")
	addCity("Есик", alm, "есік", "esik", "issyk", "yesik", "иссык")
	addCity("Щучинск", akm, "щучье", "shchuchinsk")
	addCity("Косшы", akm, "қосшы", "kosshy")
	addCity("Бурабай", akm, "boravoe", "боровое", "burabay")
	addCity("Шортанды", akm, "shortandy")
	addCity("Аксу", pvl, "ақсу", "aksu")
	addCity("Лисаковск", kst, "lisakovsk")
	addCity("Шамалган", alm, "шамалған", "shamalgan")
	addCity("Иргели", alm, "іргелі", "irgeli")
	addCity("Узынагаш", alm, "ұзынағаш", "uzynagash")
	addCity("Отеген батыр", alm, "өтеген батыр", "otegen batyr")
	addCity("Кордай", zhm, "қордай", "korday")
	addCity("Аксукент", trk, "ақсукент", "aksukent")
	cities[key("Боралдай")] = place{name: "Боралдай", province: alm, district: "Илийский"}
	cities[key("Бурундай")] = cities[key("Боралдай")]
	addCity("Атырау", atr, "гурьев")
	addCity("Усть-Каменогорск", vko, "өскемен")
	addCity("Риддер", vko)
	addCity("Тараз", zhm, "джамбул", "аулие-ата")
	addCity("Уральск", zko, "орал")
	addCity("Караганда", krg, "қарағанды")
	addCity("Темиртау", krg, "теміртау")
	addCity("Балхаш", krg, "балқаш")
	addCity("Шахтинск", krg)
	addCity("Сарань", krg)
	addCity("Костанай", kst, "қостанай", "кустанай")
	addCity("Рудный", kst)
	addCity("Кызылорда", kzo, "қызылорда")
	addCity("Актау", mng, "ақтау", "шевченко")
	addCity("Жанаозен", mng, "жаңаөзен")
	addCity("Павлодар", pvl)
	addCity("Экибастуз", pvl, "екібастұз")
	addCity("Петропавловск", sko, "петропавл")
	addCity("Туркестан", trk, "түркістан")
	addCity("Кентау", trk)
	addCity("Семей", aba, "семипалатинск")
	addCity("Талдыкорган", zht, "талдықорған", "талды-курган")
	addCity("Жезказган", ult, "жезқазған")
	addCity("Сатпаев", ult, "сәтбаев")

	// Latin and colloquial spellings
	for name, aliases := range map[string][]string{
		"Алматы": {"almaty", "alma-ata"}, "Астана": {"astana", "nur-sultan"}, "Шымкент": {"shymkent", "chimkent"},
		"Актобе": {"aktobe", "aqtobe"}, "Караганда": {"karaganda", "karagandy", "qaragandy"}, "Атырау": {"atyrau"},
		"Актау": {"aktau", "aqtau"}, "Павлодар": {"pavlodar"}, "Костанай": {"kostanay", "kostanai", "qostanay"},
		"Тараз": {"taraz"}, "Семей": {"semey", "semei"}, "Уральск": {"uralsk", "oral"}, "Кокшетау": {"kokshetau"},
		"Туркестан": {"turkestan", "turkistan"}, "Кызылорда": {"kyzylorda", "qyzylorda"},
		"Петропавловск": {"petropavlovsk", "petropavl"}, "Талдыкорган": {"taldykorgan", "taldyqorgan"},
		"Усть-Каменогорск": {"ust-kamenogorsk", "oskemen", "усть-каман", "устькаман", "уска"},
		"Экибастуз":        {"ekibastuz"}, "Темиртау": {"temirtau"}, "Конаев": {"konaev", "qonaev"}, "Жезказган": {"zhezkazgan"},
	} {
		for _, a := range aliases {
			cities[key(a)] = cities[key(name)]
		}
	}

	for name, aliases := range map[string][]string{
		"Тараз": {"dzhambul", "zhambyl"}, "Атырау": {"guryev"}, "Семей": {"semipalatinsk"}, "Конаев": {"kapshagay"},
		"Жанаозен": {"zhanaozen"}, "Сатпаев": {"satpaev", "satbayev"}, "Степногорск": {"stepnogorsk"}, "Риддер": {"ridder"},
		"Кентау": {"kentau"}, "Рудный": {"rudny", "rudnyi", "rudnyy"}, "Балхаш": {"balkhash", "balqash"}, "Талгар": {"talgar"},
		"Каскелен": {"kaskelen", "qaskelen"},
	} {
		for _, a := range aliases {
			cities[key(a)] = cities[key(name)]
		}
	}

	// city districts
	for _, d := range [][]string{
		{"Алатауский", "алатау"}, {"Алмалинский", "алмалы"}, {"Ауэзовский", "әуезов", "ауэзов"},
		{"Бостандыкский", "бостандық"}, {"Жетысуский", "жетісу"}, {"Медеуский", "медеу"},
		{"Наурызбайский", "наурызбай"}, {"Турксибский", "түрксіб", "турксиб"},
	} {
		addDistrict(d[0], "Алматы", "Алматы", d[1:]...)
	}
	for _, d := range [][]string{
		{"Алматинский", "алматы"}, {"Байконурский", "байқоңыр"}, {"Есильский", "есіл"},
		{"Сарыаркинский", "сарыарқа"}, {"Нура", "нұра"}, {"Сарайшык", "сарайшық"},
	} {
		addDistrict(d[0], "Астана", "Астана", d[1:]...)
	}
	for _, d := range [][]string{
		{"Абайский", "абай"}, {"Аль-Фарабийский", "әл-фараби", "аль-фараби"}, {"Енбекшинский", "еңбекші"},
		{"Каратауский", "қаратау"}, {"Туранский", "тұран"},
	} {
		addDistrict(d[0], "Шымкент", "Шымкент", d[1:]...)
	}
	// rural rayons (province only)
	addDistrict("Илийский", "", alm, "іле")
	addDistrict("Карасайский", "", alm, "қарасай")
	addDistrict("Талгарский", "", alm)
	addDistrict("Енбекшиказахский", "", alm, "еңбекшіқазақ")
	addDistrict("Целиноградский", "", akm)

	for name, d := range map[string]string{
		"аксай": "Ауэзовский", "мамыр": "Ауэзовский", "таугуль": "Ауэзовский", "жетысу": "Ауэзовский", "сайран": "Ауэзовский",
		"самал": "Медеуский", "орбита": "Бостандыкский", "коктем": "Бостандыкский", "алмагуль": "Бостандыкский",
		"казахфильм": "Бостандыкский", "баганашыл": "Бостандыкский", "шанырак": "Алатауский", "айгерим": "Алатауский",
		"калкаман": "Наурызбайский", "жулдыз": "Турксибский", "кулагер": "Жетысуский", "айнабулак": "Жетысуский",
	} {
		almatyAreas[key(name)] = d
	}
}
