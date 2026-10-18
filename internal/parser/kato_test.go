package parser

import (
	"testing"

	"github.com/jana-address/jana-address-api/internal/kato"
	"github.com/jana-address/jana-address-api/internal/parser/dict"
)

func TestKATOSettlements(t *testing.T) {
	recs, _, err := kato.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	old := dict.Current()
	defer dict.SetCurrent(old)
	if st := UseKATO(recs); st.Settlements < 5000 || st.Districts < 100 {
		t.Fatalf("too little from the classifier: %+v", st)
	}
	cases := []struct {
		raw  string
		want map[Field]string // "-" means must be absent
	}{
		// a rural rayon gives the region
		{"Сайрамский район с. Аксукент ул. Ташкентская 33", map[Field]string{
			Province: "Туркестанская область", District: "Сайрамский", StreetName: "Ташкентская", House: "33"}},
		// a written rayon picks one of two villages called Узынагаш
		{"Жамбылский район, с. Узынагаш, ул. Абая 5", map[Field]string{
			City: "Узынагаш", Province: "Алматинская область", StreetName: "Абая"}},
		// nothing written: the large one from the hand table
		{"с. Узынагаш, ул. Абая 5", map[Field]string{City: "Узынагаш", Province: "Алматинская область"}},
		// a village known only to the classifier, after "с."
		{"с. Кайнар, Новая 4", map[Field]string{City: "Кайнар", House: "4"}},
		// a rayon written "Район Магжана Жумабаева" in the classifier picks one of several villages called Полтавка
		{"Магжана Жумабаева ауданы, с. Полтавка, ул. Абая 5", map[Field]string{
			City: "Полтавка", District: "Магжана Жумабаева", Province: "Северо-Казахстанская область"}},
		// rural rayons named like city districts of Шымкент and Астана keep the village of the written region
		{"область Абай, Абайский район, с. Карааул, ул. Абая 5", map[Field]string{
			Province: "область Абай", District: "Абайский", City: "Карааул"}},
		{"Акмолинская обл., Есильский р-н, с. Бузулук, ул. Мира 2", map[Field]string{
			Province: "Акмолинская область", District: "Есильский", City: "Бузулук"}},
		{"Шымкент, Абайский район, ул. Абая 5", map[Field]string{City: "Шымкент", District: "Абайский", Province: "Шымкент"}},
		// villages called Сейфуллина exist, but a capital initial before a house number is a street
		{"Алматы, С. Сейфуллина 45", map[Field]string{City: "Алматы", StreetName: "С. Сейфуллина", House: "45"}},
		{"С. Кайнар, Новая 4", map[Field]string{City: "Кайнар", StreetName: "Новая", House: "4"}},
		// an old name of a town in the hand table is also a village in the written rayon of another region
		{"Мамлютский район, Щучье, ул. Абая 5", map[Field]string{
			City: "Щучье", Province: "Северо-Казахстанская область", District: "Мамлютский"}},
		// a Kazakh rayon name: written as is, and it still picks the village
		{"Іле ауданы, Өтеген батыр ауылы, Нұрлы көшесі 17", map[Field]string{City: "Отеген батыр", District: "Іле", Province: "Алматинская область"}},
		// a written rayon replaces the one the village gives
		{"Алматинская обл, село Боралдай, Іле ауданы, ул. Школьная 33", map[Field]string{City: "Боралдай", District: "Іле"}},
		// city districts of Караганда and Актобе from the classifier
		{"Караганда, район им. Казыбек би, ул. Ерубаева 20", map[Field]string{City: "Караганда", District: "Казыбек би", StreetName: "Ерубаева"}},
		{"Караганда, Казыбек би ауданы, Ерубаева 20", map[Field]string{City: "Караганда", District: "Казыбек би"}},
		{"Караганда, район Әлихан Бөкейхан, ул. Гоголя 5", map[Field]string{City: "Караганда", District: "Әлихан Бөкейхан"}},
		{"район им. Казыбек би, ул. Ерубаева 20", map[Field]string{City: "Караганда", Province: "Карагандинская область", District: "Казыбек би"}},
		{"город Актобе/Ақтөбе қаласы, район Алматы, 12 мкр, 5, кв 10", map[Field]string{
			City: "Актобе", Province: "Актюбинская область", District: "Алматы", StreetName: "12", House: "5"}},
		{"Актобе, район Астана, ул. Абая 5", map[Field]string{City: "Актобе", Province: "Актюбинская область", District: "Астана"}},
		// a town of the classifier before "мкр", with no other city, is not a living area
		{"Жаркент мкр Самал 3", map[Field]string{LivingArea: "-", StreetType: "микрорайон", StreetName: "Самал", House: "3"}},
		{"Астана, район Алматы, ул. Абая 5", map[Field]string{City: "Астана", Province: "Астана", District: "Алматы"}},
		// a new region keeps its written Russian form
		{"Абайская область, Семей, ул. Шакарима 20", map[Field]string{Province: "Абайская область", City: "Семей", StreetName: "Шакарима"}},
		{"область Абай, Семей, ул. Шакарима 20", map[Field]string{Province: "область Абай", City: "Семей"}},
		{"Абай облысы, Семей қаласы, Шәкәрім көшесі 20", map[Field]string{Province: "область Абай", City: "Семей"}},
		{"Жетысуская обл, Талдыкорган, Абая 5", map[Field]string{Province: "Жетысуская область", City: "Талдыкорган"}},
		{"область Жетысу, Талдыкорган, Абая 5", map[Field]string{Province: "область Жетысу", City: "Талдыкорган"}},
		{"обл. Улытау, Жезказган, Абая 5", map[Field]string{Province: "область Улытау", City: "Жезказган"}},
		{"Ұлытау облысы, Жезқазған қаласы, Абай көшесі 5", map[Field]string{Province: "область Ұлытау", City: "Жезказган"}},
		{"Карагандинская обл, Темиртау, Мира 4", map[Field]string{Province: "Карагандинская область"}},
		// a village next to a written rayon, without "с."
		{"Карасайский район, Иргели, ул Центральная 5", map[Field]string{
			City: "Иргели", Province: "Алматинская область", District: "Карасайский"}},
		// villages called like streets never take a bare street name
		{"Абай 10", map[Field]string{StreetName: "Абай", House: "10", City: "-"}},
		{"Алматы, Достык 5", map[Field]string{City: "Алматы", StreetName: "Достык", House: "5"}},
		{"Береке 12 кв 3", map[Field]string{StreetName: "Береке", City: "-"}},
		// the old cases still hold
		{"город Алматы/Алматы қаласы, Мкр Аксай 3а, 59, кв 70", map[Field]string{
			Province: "Алматы", City: "Алматы", District: "Ауэзовский", StreetName: "Аксай 3а", House: "59", Apartment: "70"}},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			o := Parse(tc.raw)
			for f, want := range tc.want {
				got, ok := o.Values[f]
				if want == "-" {
					if ok {
						t.Errorf("%s = %q, want empty", f, got)
					}
					continue
				}
				if got != want {
					t.Errorf("%s = %q, want %q (all: %v)", f, got, want, o.Values)
				}
			}
		})
	}
}

// TestKATOTowns: a town of the classifier written alone before the street is the city, with its region.
func TestKATOTowns(t *testing.T) {
	recs, _, err := kato.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	old := dict.Current()
	defer dict.SetCurrent(old)
	UseKATO(recs)
	cases := []struct {
		raw  string
		want map[Field]string // "-" means must be absent
	}{
		{"Жаркент, ул. Абая 5", map[Field]string{City: "Жаркент", Province: "область Жетісу", StreetName: "Абая", House: "5"}},
		{"Кульсары, ул Абая 12, кв 3", map[Field]string{City: "Кульсары", Province: "Атырауская область", Apartment: "3"}},
		{"Сарыагаш Абая 5", map[Field]string{City: "Сарыагаш", Province: "Туркестанская область", StreetName: "Абая"}},
		{"Аягоз, Абая 5", map[Field]string{City: "Аягоз", Province: "область Абай", StreetName: "Абая"}},
		// a town and villages of the same name: "г." and a bare name mean the town
		{"г. Шу, ул. Абая 5", map[Field]string{City: "Шу", Province: "Жамбылская область"}},
		{"г. Шалкар, ул. Абая 5", map[Field]string{City: "Шалкар", Province: "Актюбинская область"}},
		{"г. Кандыагаш, ул. Абая 5", map[Field]string{City: "Кандыагаш", Province: "Актюбинская область"}},
		{"Шалкар, Абая 5", map[Field]string{City: "Шалкар", Province: "Актюбинская область"}},
		// "с." still picks a village, and a written region its own one
		{"Целиноградский район, с. Шалкар, ул. Абая 5", map[Field]string{City: "Шалкар", Province: "Акмолинская область"}},
		// streets named like towns stay streets
		{"Абай, 12", map[Field]string{StreetName: "Абай", House: "12", City: "-"}},
		{"Абай даңғылы 10", map[Field]string{StreetName: "Абай", House: "10", City: "-"}},
		{"Абай д 44 кв. 19 конаев", map[Field]string{StreetName: "Абай", House: "44", City: "Конаев"}},
		{"Абай, дом 126Д", map[Field]string{StreetName: "Абай", House: "126Д", City: "-"}},
		{"алатау сити жк ул.пушкина 111, 79 кв", map[Field]string{City: "-", StreetName: "пушкина"}},
		{"лисаков сейфуллин кошеси 283", map[Field]string{City: "Лисаковск", StreetName: "сейфуллин"}},
		{"Алматы, Есиль 5", map[Field]string{City: "Алматы", StreetName: "Есиль", House: "5"}},
		{"Алтай 2 мкр 5", map[Field]string{City: "-"}},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			o := Parse(tc.raw)
			for f, want := range tc.want {
				got, ok := o.Values[f]
				if want == "-" {
					if ok {
						t.Errorf("%s = %q, want empty (all: %v)", f, got, o.Values)
					}
					continue
				}
				if got != want {
					t.Errorf("%s = %q, want %q (all: %v)", f, got, want, o.Values)
				}
			}
		})
	}

	// every town of the classifier: after "г." always, written alone unless it is named like a city
	// district ("Есиль" is also a district of Астана, "Абай" of Шымкент)
	likeDistrict := map[string]bool{"Есиль": true, "Алатау": true, "Каратау": true, "Абай": true, "Байконыр": true}
	for _, r := range recs {
		if r.Kind != kato.City {
			continue
		}
		if o := Parse("г. " + r.NameRUClean + ", ул. Абая 5"); o.Values[City] == "" || o.Values[Province] == "" {
			t.Errorf("г. %s: %v", r.NameRUClean, o.Values)
		}
		o := Parse(r.NameRUClean + ", ул. Абая 5")
		if got := o.Values[City] != "" && o.Values[Province] != ""; got == likeDistrict[r.NameRUClean] {
			t.Errorf("%s alone: %v", r.NameRUClean, o.Values)
		}
	}
}

// TestAreaBeforeStreet: an area no directory knows, run into a street the map knows in some city, is cut
// before the street; a quoted or known name, or a ЖК with a short last word, stays whole.
func TestAreaBeforeStreet(t *testing.T) {
	recs, _, err := kato.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	old := dict.Current()
	defer dict.SetCurrent(old)
	UseKATO(recs)
	cases := []struct {
		raw  string
		want map[Field]string // "-" means must be absent
	}{
		{"Астана мкр Шугыла Валиханова 12 кв 3", map[Field]string{LivingArea: "Шугыла", StreetName: "Валиханова", House: "12"}},
		{"Усть-Каменогорск мкр Береке Независимости 7", map[Field]string{LivingArea: "Береке", StreetName: "Независимости"}},
		{"мкр Ынтымак Горького 15", map[Field]string{LivingArea: "Ынтымак", StreetName: "Горького", House: "15"}},
		{"ЖК Жасыл Ел Байтурсынова 9 кв 4", map[Field]string{LivingArea: "Жасыл Ел", StreetName: "Байтурсынова"}},
		{`ЖК "Жасыл Ел" 9`, map[Field]string{LivingArea: "-", StreetName: "Жасыл Ел", House: "9"}},
		{"Алматы ЖК Алтын Булак 2", map[Field]string{LivingArea: "-", StreetName: "Алтын Булак"}},
		{"Алматы мкр Жас Канат 5", map[Field]string{LivingArea: "-", StreetName: "Жас Канат", House: "5"}},
		{"павлодар мкр достык академика маргулана 47 кв 9", map[Field]string{LivingArea: "достык", StreetName: "академика маргулана"}},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			o := Parse(tc.raw)
			for f, want := range tc.want {
				got, ok := o.Values[f]
				if want == "-" {
					if ok {
						t.Errorf("%s = %q, want empty", f, got)
					}
					continue
				}
				if got != want {
					t.Errorf("%s = %q, want %q (all: %v)", f, got, want, o.Values)
				}
			}
		})
	}
}

// TestTypeGluedAfterCity: a street type glued to a street in a word of its own after the city is split off
// when the city's map knows the street; other words keep their letters.
func TestTypeGluedAfterCity(t *testing.T) {
	recs, _, err := kato.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	old := dict.Current()
	defer dict.SetCurrent(old)
	UseKATO(recs)
	cases := []struct {
		raw  string
		want map[Field]string
	}{
		{"караганда улбукетова 12 кв 4", map[Field]string{StreetType: "улица", StreetName: "букетова", House: "12"}},
		{"павлодар пр-тназарбаева 15", map[Field]string{StreetType: "проспект", StreetName: "назарбаева"}},
		{"алматы улжибек жолы 20", map[Field]string{StreetName: "жибек жолы", House: "20"}},
		{"астана улытау 5", map[Field]string{StreetName: "улытау"}},
		{"Алматы пригородный 5", map[Field]string{StreetName: "пригородный"}},
		// a street known by its surname keeps a first syllable that reads as a type: "кош" in Кошкарбаева
		{"астанакошкарбаева 40", map[Field]string{StreetName: "кошкарбаева", House: "40"}},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			o := Parse(tc.raw)
			for f, want := range tc.want {
				if got := o.Values[f]; got != want {
					t.Errorf("%s = %q, want %q (all: %v)", f, got, want, o.Values)
				}
			}
		})
	}
}

// TestTypoCityOpening: a city one letter off before a number opens the line when a typed street follows;
// a street name stays a street.
func TestTypoCityOpening(t *testing.T) {
	recs, _, err := kato.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	old := dict.Current()
	defer dict.SetCurrent(old)
	UseKATO(recs)
	cases := []struct {
		raw  string
		want map[Field]string // "-" means must be absent
	}{
		{"Кокшетауу 15 кв 3, ул. Абая", map[Field]string{City: "Кокшетау", StreetName: "Абая"}},
		{"Павлодаар, 5 мкр, 12", map[Field]string{City: "Павлодар", House: "12"}},
		{"Сатпаева 22, мкр Самал 5", map[Field]string{City: "-", StreetName: "Сатпаева", House: "22"}},
		{"Абая 10 кв 5 мкр Самал", map[Field]string{City: "-", StreetName: "Абая", House: "10"}},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			o := Parse(tc.raw)
			for f, want := range tc.want {
				got, ok := o.Values[f]
				if want == "-" {
					if ok {
						t.Errorf("%s = %q, want empty", f, got)
					}
					continue
				}
				if got != want {
					t.Errorf("%s = %q, want %q (all: %v)", f, got, want, o.Values)
				}
			}
		})
	}
}
