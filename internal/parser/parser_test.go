package parser

import (
	"testing"
	"time"
)

func TestParseExamples(t *testing.T) {
	cases := []struct {
		raw  string
		want map[Field]string // fields that must match; "-" means must be absent
	}{
		{"город Алматы/Алматы қаласы, Мкр Аксай 3а, 59, кв 70", map[Field]string{
			Province: "Алматы", City: "Алматы", District: "Ауэзовский", LivingArea: "-", StreetType: "микрорайон",
			StreetName: "Аксай 3а", House: "59", Apartment: "70", Entrance: "-", Floor: "-", Landmark: "-"}},
		{"66квартал 12дом 69квартира", map[Field]string{
			StreetType: "квартал", StreetName: "66", House: "12", Apartment: "69"}},
		{"Ул .Курильская 9 кв 1", map[Field]string{
			StreetType: "улица", StreetName: "Курильская", House: "9", Apartment: "1"}},
		{"Туран 55/6 магазин береке 4подьезд 9этаж", map[Field]string{
			StreetName: "Туран", House: "55/6", Landmark: "магазин береке", Entrance: "4", Floor: "9", Apartment: "-"}},
		{"Нурсити 2 микрорайон 11дом 5 подьезд 239 кв", map[Field]string{
			StreetType: "микрорайон", StreetName: "Нурсити 2", House: "11", Entrance: "5", Apartment: "239"}},
		{"Турксибский район, Рыбинский переулок, дом 4", map[Field]string{
			District: "Турксибский", City: "Алматы", Province: "Алматы", StreetType: "переулок", StreetName: "Рыбинский", House: "4"}},
		{"Кулманова 133", map[Field]string{StreetName: "Кулманова", House: "133", StreetType: "-"}},
		{"жуалы 19", map[Field]string{StreetName: "жуалы", House: "19"}},
		{"город Актобе/Ақтөбе қаласы, Назарбаева8, кв 12", map[Field]string{
			City: "Актобе", Province: "Актюбинская область", StreetName: "Назарбаева", House: "8", Apartment: "12"}},
		{"Кулманова 133 flat 191", map[Field]string{StreetName: "Кулманова", House: "133", Apartment: "191"}},
		{"ул Абая 10 kv 12", map[Field]string{StreetType: "улица", StreetName: "Абая", House: "10", Apartment: "12"}},
		{"Абай көшесі 5 үй 7 пәтер", map[Field]string{StreetType: "улица", StreetName: "Абай", House: "5", Apartment: "7"}},
		{"г.Алматы, пр. Достык 1/2, кв 5", map[Field]string{
			City: "Алматы", StreetType: "проспект", StreetName: "Достык", House: "1/2", Apartment: "5"}},
		{"Боралдай, ул Ленина 12а", map[Field]string{
			City: "Боралдай", Province: "Алматинская область", District: "Илийский", StreetName: "Ленина", House: "12а"}},
		{"Qazpost Солнечный, Мира 7", map[Field]string{Landmark: "Qazpost Солнечный", StreetName: "Мира", House: "7"}},
		{"​ул. Сатпаева​ 22, кв­ 3", map[Field]string{StreetName: "Сатпаева", House: "22", Apartment: "3"}},
		{"Карагандинская обл, г. Темиртау, ул. Мира 4, кв. 15", map[Field]string{
			Province: "Карагандинская область", City: "Темиртау", StreetName: "Мира", House: "4", Apartment: "15"}},
		// review fixes: admin markers without commas, initials, postcodes, hyphenated microdistricts
		{"г. Алматы ул. Абая 10 кв 5", map[Field]string{City: "Алматы", StreetType: "улица", StreetName: "Абая", House: "10", Apartment: "5"}},
		{"г Алматы Абая 10", map[Field]string{City: "Алматы", StreetName: "Абая", House: "10"}},
		{"г.Шымкент ул.Байтурсынова 12 кв.4", map[Field]string{City: "Шымкент", StreetName: "Байтурсынова", House: "12", Apartment: "4"}},
		{"с.Иргели ул.Абая 5", map[Field]string{City: "Иргели", StreetName: "Абая", House: "5"}},
		{"Тұран ауданы Байтұрсынов көшесі 12", map[Field]string{District: "Туранский", City: "Шымкент", StreetName: "Байтұрсынов", House: "12"}},
		{"Карагандинская обл г. Темиртау ул. Мира 4", map[Field]string{Province: "Карагандинская область", City: "Темиртау", StreetName: "Мира", House: "4"}},
		{"Алматы, ул. Г. Потанина 3", map[Field]string{City: "Алматы", StreetType: "улица", StreetName: "Г. Потанина", House: "3"}},
		{"ул. Д. Кунаева 12, кв 5", map[Field]string{StreetType: "улица", StreetName: "Д. Кунаева", House: "12", Apartment: "5"}},
		{"160000, г. Шымкент, ул. Байтурсынова 12", map[Field]string{City: "Шымкент", House: "12", Apartment: "-"}},
		{"050000 Алматы ул Абая 10", map[Field]string{City: "Алматы", Province: "Алматы", StreetName: "Абая", House: "10"}},
		{"Алматы, мкр Самал-2, д. 33, кв. 7", map[Field]string{StreetName: "Самал-2", House: "33", Apartment: "7", District: "Медеуский"}},
		{"Алматы, мкр Самал-2", map[Field]string{StreetName: "Самал-2", House: "-"}},
		// fast typing: typos in markers, no commas
		{"гролд актобе дом5 6 кв 55 подлъезд 5", map[Field]string{
			City: "Актобе", Province: "Актюбинская область", House: "5", Apartment: "55", Entrance: "5"}},
		{"алматы абая 15 кваритра 3", map[Field]string{City: "Алматы", StreetName: "абая", House: "15", Apartment: "3"}},
		{"Астана, проспкт Республики 24, этж 3", map[Field]string{City: "Астана", StreetType: "проспект", StreetName: "Республики", House: "24", Floor: "3", Apartment: "-"}},
		// typos in city names: canonical city + province
		{"г. алматыы, ул. Абая 10", map[Field]string{City: "Алматы", Province: "Алматы", StreetName: "Абая", House: "10"}},
		{"алматыы абая 10 кв 5", map[Field]string{City: "Алматы", Province: "Алматы", StreetName: "абая", House: "10", Apartment: "5"}},
		{"город Караганды, ул. Ерубаева 5", map[Field]string{City: "Караганда", Province: "Карагандинская область"}},
		{"г. Шымкнт, Тауке хана 20", map[Field]string{City: "Шымкент", Province: "Шымкент", StreetName: "Тауке хана", House: "20"}},
		// real street names close to marker words must stay names
		{"Райымбек 160", map[Field]string{StreetName: "Райымбек", House: "160"}},
		{"Гоголя 5", map[Field]string{StreetName: "Гоголя", House: "5"}},
		{"Магжана 7", map[Field]string{StreetName: "Магжана", House: "7"}},
		{"Сатпаева 22", map[Field]string{StreetName: "Сатпаева", House: "22", City: "-"}},
		{"Алматы, Сатпаева 22", map[Field]string{City: "Алматы", StreetName: "Сатпаева", House: "22"}},
		{"Алматы, Сатпаева, 22", map[Field]string{City: "Алматы", House: "22"}},
		{"алматыы, абая 10", map[Field]string{City: "Алматы", Province: "Алматы", StreetName: "абая", House: "10"}},
		{"Ауэзова 12", map[Field]string{StreetName: "Ауэзова", House: "12"}},
		{"ул. Сейфуллина 498", map[Field]string{StreetType: "улица", StreetName: "Сейфуллина", House: "498"}},
		// review: blocks, letter after a space, country prefix
		{"Абая 10 корпус 2 кв 5", map[Field]string{StreetName: "Абая", House: "10", Apartment: "5"}},
		{"ул Абая 10 а, кв 5", map[Field]string{StreetName: "Абая", House: "10а", Apartment: "5"}},
		{"ул Абая 10 в подъезде 2", map[Field]string{StreetName: "Абая", House: "10"}},
		{"Казахстан, г. Алматы, ул. Абая 10", map[Field]string{City: "Алматы", StreetName: "Абая", House: "10", Landmark: "-"}},
		{"РК, Алматы, Абая 10", map[Field]string{City: "Алматы", StreetName: "Абая", House: "10"}},
		// fast-typing set: Latin cities, duplicates, direction of "кв", ЖК, spaced fractions, regions
		{"almaty, abaya 52 kv 10", map[Field]string{City: "Алматы", Province: "Алматы", StreetName: "abaya", House: "52", Apartment: "10"}},
		{"алматы алматы жандосова 58 кв 4", map[Field]string{City: "Алматы", StreetName: "жандосова", House: "58", Apartment: "4"}},
		{"ул Сыганак 25 кв 300 4подъезд 12этаж", map[Field]string{StreetName: "Сыганак", House: "25", Apartment: "300", Entrance: "4", Floor: "12"}},
		{"Муканова 241 кв 34 5 этаж", map[Field]string{StreetName: "Муканова", House: "241", Apartment: "34", Floor: "5"}},
		{"Астана жк Северное сияние дом 7 кв 88", map[Field]string{StreetType: "жилой комплекс", StreetName: "Северное сияние", House: "7", LivingArea: "-"}},
		{"алматы жк Esentai City аль-фараби 77/7 кв 112", map[Field]string{LivingArea: "Esentai City", StreetName: "аль-фараби", House: "77/7"}},
		{"навои 208 / 2 кв 17", map[Field]string{StreetName: "навои", House: "208/2", Apartment: "17"}},
		{"шымкент микрорайн нурсат 112 кв 45", map[Field]string{StreetType: "микрорайон", StreetName: "нурсат", House: "112"}},
		{"усть-каменогорск пр казахстан 66 кв 20", map[Field]string{StreetType: "проспект", StreetName: "казахстан", House: "66"}},
		{"ВКО, Усть-Каман, ул. Протозанова 83 кв 5", map[Field]string{Province: "Восточно-Казахстанская область", City: "Усть-Каменогорск"}},
		{"Илийский район п Боралдай ул Аэродромная 14", map[Field]string{City: "Боралдай", District: "Илийский", StreetName: "Аэродромная"}},
		{"Астана Алматинский р-н Момышулы 23 кв 56", map[Field]string{District: "Алматинский", StreetName: "Момышулы", House: "23"}},
		{"Алматы мкр Орбита-1 дом 21 кв 30 напротив школы 12", map[Field]string{Landmark: "напротив школы 12", House: "21"}},
		{"Алматы улца Сейфуллина 498 кв 61", map[Field]string{StreetType: "улица", StreetName: "Сейфуллина"}},
		{"Астана р-н Нура ул Мухамедханова 8 кв 3", map[Field]string{City: "Астана", District: "Нура", StreetName: "Мухамедханова"}},
		{"Алматы мкр Самал-2 ул Жолдасбекова 97 кв 14", map[Field]string{LivingArea: "Самал-2", StreetType: "улица", StreetName: "Жолдасбекова", House: "97"}},
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

func TestSourcesAndSpans(t *testing.T) {
	raw := "город Алматы/Алматы қаласы, Мкр Аксай 3а, 59, кв 70"
	o := Parse(raw)
	if o.SourceOf(District) != SourceDirectory || o.SourceOf(StreetName) != SourceString || o.SourceOf(Entrance) != SourceAbsent {
		t.Fatalf("sources = %v", o.Sources)
	}
	u := []rune(raw) // raw is BMP-only, so UTF-16 offsets equal rune offsets
	found := false
	for i, s := range o.Spans {
		if i > 0 && s.Start < o.Spans[i-1].End {
			t.Fatalf("overlapping spans: %v", o.Spans)
		}
		if s.Field == StreetName && string(u[s.Start:s.End]) == "Аксай 3а" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no street_name span for «Аксай 3а»: %v", o.Spans)
	}
	if o.Status() != "confident" {
		t.Fatalf("status = %s (%.2f)", o.Status(), o.Confidence)
	}
}

// Repeated markers used to loop forever in units().
func TestRepeatedMarkersTerminate(t *testing.T) {
	for _, raw := range []string{"кв 5 кв 6", "ул Абая 10, оф 5, кв 3", "дом 1 дом 2", "Абая 10 2 этаж 3 эт", "ул Абая 10 кв 5 кв.5"} {
		done := make(chan Output, 1)
		go func() { done <- Parse(raw) }()
		select {
		case o := <-done:
			if raw == "кв 5 кв 6" && o.Values[Apartment] != "5" {
				t.Errorf("%q: apartment = %q", raw, o.Values[Apartment])
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%q: Parse did not return", raw)
		}
	}
}

func TestGarbageIsLowConfidence(t *testing.T) {
	for _, raw := range []string{"", "   ", "???", "привет как дела"} {
		o := Parse(raw)
		if o.Confidence >= 0.5 {
			t.Errorf("%q: confidence %.2f", raw, o.Confidence)
		}
	}
}

func BenchmarkParse(b *testing.B) {
	for b.Loop() {
		Parse("город Алматы/Алматы қаласы, Мкр Аксай 3а, 59, кв 70")
	}
}
