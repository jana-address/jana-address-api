package parser

import "testing"

// checkFields compares the parsed fields with the expected ones; "-" means the field must be absent.
func checkFields(t *testing.T, raw string, want map[Field]string) {
	t.Helper()
	o := Parse(raw)
	for f, w := range want {
		got, ok := o.Values[f]
		if w == "-" {
			if ok {
				t.Errorf("%s: %s = %q, want empty", raw, f, got)
			}
			continue
		}
		if got != w {
			t.Errorf("%s: %s = %q, want %q (all: %v)", raw, f, got, w, o.Values)
		}
	}
}

func TestNumbersAfterArea(t *testing.T) {
	cases := []struct {
		raw  string
		want map[Field]string
	}{
		// the number of a microdistrict before a comma is not a house: the next number is
		{"Актау мкр.7, 41 кв 112, 3 эт.", map[Field]string{StreetName: "7", House: "41", Apartment: "112", Floor: "3"}},
		{"Караганда мкр Степной-3, 18 пәтер 205, 2 под", map[Field]string{House: "18", Apartment: "205", Entrance: "2"}},
		{"Актобе мкрн 11, 27 под 3 эт 4, 9 кв", map[Field]string{House: "27", Entrance: "3", Floor: "4", Apartment: "9"}},
		{"aktau mkrn 14, 21 apt 75, 2 etazh", map[Field]string{House: "21", Apartment: "75", Floor: "2"}},
		// a ЖК name before its marker or in quotes, then the house
		{"Алматы Grand Park жк 12 кв. 40, 5 этаж", map[Field]string{House: "12", Apartment: "40", Floor: "5"}},
		{"Астана жк \"Нурсая\" 9 кв 73, 4 под", map[Field]string{House: "9", Apartment: "73", Entrance: "4"}},
		// a postfix microdistrict type keeps the house after it
		{"Тараз Мангилик мкр. 14, 2 блок кв 30, 1 под", map[Field]string{StreetName: "Мангилик", House: "14", Apartment: "30"}},
	}
	for _, tc := range cases {
		checkFields(t, tc.raw, tc.want)
	}
}

func TestNumbersAroundBlock(t *testing.T) {
	cases := []struct {
		raw  string
		want map[Field]string
	}{
		// the number after a block is taken by its own marker, so the block takes the one before it
		{"Шымкент, Байтурсынова 14, 3 блок 2 подъезд, кв. 51", map[Field]string{House: "14", Entrance: "2", Apartment: "51"}},
		{"Астана, Кенесары көшесі 40, 2 блок 7 қабат", map[Field]string{House: "40", Floor: "7"}},
		{"Тараз Абая үй 21а 4 корпус 1 под. кв17", map[Field]string{House: "21а", Entrance: "1", Apartment: "17"}},
		{"Кокшетау ул. Абая 12, 2 блок 45 кв подъезд3", map[Field]string{House: "12", Apartment: "45", Entrance: "3"}},
		{"Актобе 11 мкр. 7 үй 2 корпус 15 офис", map[Field]string{House: "7", Apartment: "15"}},
		{"Актобе 4 мкр 31, 2 блок 9 эт 140 кв.", map[Field]string{House: "31", Floor: "9", Apartment: "140"}},
		// a block followed by its own number stays as it was
		{"Абая 10 блок 2 кв 7", map[Field]string{House: "10", Apartment: "7"}},
		{"Абая 10 корпус", map[Field]string{House: "10"}},
	}
	for _, tc := range cases {
		checkFields(t, tc.raw, tc.want)
	}
}

func TestLandmarkNumbers(t *testing.T) {
	cases := []struct {
		raw  string
		want map[Field]string
	}{
		// the number of a school before a Kazakh postfix landmark is not a unit number
		{"Астана, Сарайшык көшесі 9 үй 15 мектептің жанында", map[Field]string{House: "9", Landmark: "15 мектептің жанында"}},
		{"Шымкент Абая 30, 12 пәтер 7 балабақшаның жанында", map[Field]string{House: "30", Apartment: "12", Landmark: "7 балабақшаның жанында"}},
		{"Тараз мкр Жайлау 14, 3 мектептің жанында", map[Field]string{StreetName: "Жайлау", House: "14", Landmark: "3 мектептің жанында"}},
		// with no other house number, or taken by a marker before it, the number stays a unit number
		{"Абай көшесі 64 мектептің жанында", map[Field]string{House: "64", Landmark: "мектептің жанында"}},
		{"Абая 5 кв 12 мектептің жанында", map[Field]string{House: "5", Apartment: "12", Landmark: "мектептің жанында"}},
		{"Сатпаева д 30 эт 2 кв 88 емхананың қасында", map[Field]string{House: "30", Floor: "2", Apartment: "88"}},
		{"Конаев мкрн. 2 үй 19 мектептің қасында", map[Field]string{StreetName: "2", House: "19"}},
		// a school number after "№" or before a bracket belongs to the landmark
		{"Караганда мкр. Голубые пруды 17 кв 4 напротив школы №35", map[Field]string{
			StreetName: "Голубые пруды", House: "17", Apartment: "4", Landmark: "напротив школы №35"}},
		{"Тараз мкрн. Карасу 21 кв 6 (возле школы 14)", map[Field]string{StreetName: "Карасу", House: "21", Landmark: "возле школы 14"}},
	}
	for _, tc := range cases {
		checkFields(t, tc.raw, tc.want)
	}
}

func TestSpacedHouseLetter(t *testing.T) {
	cases := []struct {
		raw  string
		want map[Field]string
	}{
		// a house letter written apart before a mistyped apartment word, a ЖК or a bracket
		{"Шымкент Байтурсынова 52 г кваритра 9", map[Field]string{House: "52г", Apartment: "9"}},
		{"Тараз мкрн Жайлау, 8 в кварира 14 подъезд 1", map[Field]string{House: "8в", Apartment: "14", Entrance: "1"}},
		{"Алматы пәтер 3, д14 г ж.к. Самал", map[Field]string{House: "14г", Apartment: "3"}},
		{"Астана мкр Самал 12 г (возле рынка)", map[Field]string{House: "12г", Landmark: "возле рынка"}},
		// an inflected word after "в" keeps the preposition
		{"Абая 10 в подъезде 3", map[Field]string{House: "10", Entrance: "3"}},
		{"Абая 12 в квартире 5", map[Field]string{House: "12", Apartment: "5"}},
		// Latin "of." is an office
		{"Almaty zhk Koktem 7 of. 120", map[Field]string{House: "7", Apartment: "120"}},
		{"Republic of Kazakhstan, Almaty, Abay 10", map[Field]string{House: "10", Apartment: "-"}},
	}
	for _, tc := range cases {
		checkFields(t, tc.raw, tc.want)
	}
}

func TestTypeWords(t *testing.T) {
	cases := []struct {
		raw  string
		want map[Field]string
	}{
		{"Astana b-r Kenesary 12, kv 40", map[Field]string{StreetType: "бульвар", StreetName: "Kenesary", House: "12", Apartment: "40"}},
		// "тасжолы" written as one word is шоссе; the map still finds the road by its full name
		{"Астана, Қорғалжын тасжолы 9", map[Field]string{StreetType: "шоссе", StreetName: "Қорғалжын", House: "9"}},
		{"Шымкент Темірлан тасжолы 100", map[Field]string{StreetType: "шоссе", StreetName: "Темірлан", House: "100", District: "Абайский"}},
	}
	for _, tc := range cases {
		checkFields(t, tc.raw, tc.want)
	}
}

func TestPostfixLandmarkBounds(t *testing.T) {
	cases := []struct {
		raw  string
		want map[Field]string
	}{
		// the name right after a street type and a city name do not join a Kazakh postfix landmark
		{"Атырау кв. 12 дом 7 ул Махамбета балабақшаның жанында", map[Field]string{
			StreetName: "Махамбета", House: "7", Apartment: "12", Landmark: "балабақшаның жанында"}},
		{"ул.Байтурсынова 12 дом абайский р-н шымкент шымкент балабақшаның жанында", map[Field]string{StreetName: "Байтурсынова", House: "12", Landmark: "балабақшаның жанында"}},
		// after a Kazakh type, which follows its name, the words before the landmark noun stay in it
		{"Семей Абай көшесі 5 Small дүкенінің жанында", map[Field]string{StreetName: "Абай", House: "5", Landmark: "Small дүкенінің жанында"}},
	}
	for _, tc := range cases {
		checkFields(t, tc.raw, tc.want)
	}
}
