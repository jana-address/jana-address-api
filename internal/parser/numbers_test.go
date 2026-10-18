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
