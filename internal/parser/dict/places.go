package dict

import "github.com/jana-address/jana-address-api/internal/parser/text"

// ---------- small built-in directory (demo; the full one is built from KATO + OSM) ----------

// Place is a city, town or village.
type Place struct {
	Name     string
	Province string
	District string // known rayon for small settlements
	Town     bool   // a town ("г."), not a village
}

// DistrictInfo is a city district or a rural rayon.
type DistrictInfo struct {
	Name     string
	City     string // empty for rural rayons
	Province string
}

// Hand-written base tables: aliases, Latin and old names, city districts. The directory used by
// the parser starts from them and adds the official classifier on top (directory.go).
var (
	baseCities    = map[string]Place{}
	baseRegions   = map[string]string{}
	baseDistricts = map[string]DistrictInfo{}
	// AlmatyAreas maps a microdistrict (first word, folded) to its district of Almaty.
	AlmatyAreas = map[string]string{}
	baseAreas   = map[string][]Area{}
)

// Area is a living area (жилой массив, микрорайон) of a city: "Пришахтинск" is a part of Караганда, not a town.
type Area struct {
	City     string
	District string // empty when not known
}

func addArea(city, district string, names ...string) {
	for _, n := range names {
		baseAreas[text.Key(n)] = append(baseAreas[text.Key(n)], Area{City: city, District: district})
	}
}

func addCity(name, province string, aliases ...string) {
	p := Place{Name: name, Province: province, Town: true}
	baseCities[text.Key(name)] = p
	for _, a := range aliases {
		baseCities[text.Key(a)] = p
	}
}

func addRegion(name string, keys ...string) {
	for _, k := range keys {
		baseRegions[text.Key(k)] = name
	}
}

func addDistrict(name, city, province string, keys ...string) {
	d := DistrictInfo{Name: name, City: city, Province: province}
	baseDistricts[text.Key(name)] = d
	for _, k := range keys {
		baseDistricts[text.Key(k)] = d
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
	addCity("Аксу", pvl, "ақсу", "aksu", "aqsu", "ермак") // Ермак until 1993
	addCity("Лисаковск", kst, "лисаков", "lisakovsk")     // Лисаков in Kazakh
	addCity("Шамалган", alm, "шамалған", "shamalgan")
	addCity("Иргели", alm, "іргелі", "irgeli")
	addCity("Узынагаш", alm, "ұзынағаш", "uzynagash")
	addCity("Отеген батыр", alm, "өтеген батыр", "otegen batyr")
	addCity("Кордай", zhm, "қордай", "korday")
	addCity("Аксукент", trk, "ақсукент", "aksukent")
	baseCities[text.Key("Боралдай")] = Place{Name: "Боралдай", Province: alm, District: "Илийский"}
	baseCities[text.Key("Бурундай")] = baseCities[text.Key("Боралдай")]
	for _, a := range []string{"boralday", "boraldai", "burunday"} {
		baseCities[text.Key(a)] = baseCities[text.Key("Боралдай")]
	}
	addCity("Атырау", atr, "гурьев")
	addCity("Усть-Каменогорск", vko, "өскемен")
	addCity("Риддер", vko, "лениногорск")
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
	addCity("Актау", mng, "ақтау") // not the old name "Шевченко": one of the most common street names
	addCity("Жанаозен", mng, "жаңаөзен", "новый узень")
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
		"Актобе": {"aktobe", "aqtobe", "aktyubinsk", "aktiubinsk"}, "Караганда": {"karaganda", "karagandy", "qaragandy"}, "Атырау": {"atyrau"},
		"Актау": {"aktau", "aqtau"}, "Павлодар": {"pavlodar"}, "Костанай": {"kostanay", "kostanai", "qostanay"},
		"Тараз": {"taraz"}, "Семей": {"semey", "semei"}, "Уральск": {"uralsk", "oral"}, "Кокшетау": {"kokshetau"},
		"Туркестан": {"turkestan", "turkistan"}, "Кызылорда": {"kyzylorda", "qyzylorda"},
		"Петропавловск": {"petropavlovsk", "petropavl"}, "Талдыкорган": {"taldykorgan", "taldyqorgan"},
		"Усть-Каменогорск": {"ust-kamenogorsk", "oskemen", "усть-каман", "устькаман", "уска"},
		"Экибастуз":        {"ekibastuz"}, "Темиртау": {"temirtau"}, "Конаев": {"konaev", "qonaev"}, "Жезказган": {"zhezkazgan"},
	} {
		for _, a := range aliases {
			baseCities[text.Key(a)] = baseCities[text.Key(name)]
		}
	}

	for name, aliases := range map[string][]string{
		"Тараз": {"dzhambul", "zhambyl"}, "Атырау": {"guryev"}, "Семей": {"semipalatinsk"}, "Конаев": {"kapshagay"},
		"Жанаозен": {"zhanaozen"}, "Сатпаев": {"satpaev", "satbayev"}, "Степногорск": {"stepnogorsk"}, "Риддер": {"ridder"},
		"Кентау": {"kentau"}, "Рудный": {"rudny", "rudnyi", "rudnyy"}, "Балхаш": {"balkhash", "balqash"}, "Талгар": {"talgar"},
		"Каскелен": {"kaskelen", "qaskelen"},
	} {
		for _, a := range aliases {
			baseCities[text.Key(a)] = baseCities[text.Key(name)]
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
	// current official names ("район Есиль"), the old adjectives stay as aliases
	for _, d := range [][]string{
		{"Алматы", "алматинский"}, {"Байконур", "байқоңыр", "байконыр", "байконурский"}, {"Есиль", "есіл", "есильский"},
		{"Сарыарка", "сарыарқа", "сарыаркинский"}, {"Нура", "нұра", "нуринский"}, {"Сарайшык", "сарайшық"},
	} {
		addDistrict(d[0], "Астана", "Астана", d[1:]...)
	}
	for _, d := range [][]string{
		{"Абайский", "абай"}, {"Аль-Фарабийский", "әл-фараби", "аль-фараби"}, {"Енбекшинский", "еңбекші"},
		{"Каратауский", "қаратау"}, {"Туран", "тұран", "туранский"},
	} {
		addDistrict(d[0], "Шымкент", "Шымкент", d[1:]...)
	}
	// rural rayons (province only)
	addDistrict("Илийский", "", alm, "іле")
	addDistrict("Карасайский", "", alm, "қарасай")
	addDistrict("Талгарский", "", alm)
	addDistrict("Енбекшиказахский", "", alm, "еңбекшіқазақ")
	addDistrict("Целиноградский", "", akm)

	// living areas that are not settlements of the classifier and are not called like villages elsewhere
	addArea("Караганда", "", "Пришахтинск", "Майкудук", "Майқұдық", "Новый Майкудук", "Сортировка")

	for name, d := range map[string]string{
		"аксай": "Ауэзовский", "мамыр": "Ауэзовский", "таугуль": "Ауэзовский", "жетысу": "Ауэзовский", "сайран": "Ауэзовский",
		"самал": "Медеуский", "орбита": "Бостандыкский", "коктем": "Бостандыкский", "алмагуль": "Бостандыкский",
		"казахфильм": "Бостандыкский", "баганашыл": "Бостандыкский", "шанырак": "Алатауский", "айгерим": "Алатауский",
		"калкаман": "Наурызбайский", "жулдыз": "Турксибский", "кулагер": "Жетысуский", "айнабулак": "Жетысуский",
	} {
		AlmatyAreas[text.Key(name)] = d
	}
}
