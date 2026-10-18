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
