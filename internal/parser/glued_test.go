package parser

import (
	"testing"

	"github.com/jana-address/jana-address-api/internal/kato"
	"github.com/jana-address/jana-address-api/internal/parser/dict"
)

// TestGluedBorders: the borders of words typed without spaces between a city, its type word and the street.
func TestGluedBorders(t *testing.T) {
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
		// "қала" glued after a mistyped city is the city's type word, not a part of the street
		{"павлдарқалалермонтова 15", map[Field]string{City: "Павлодар", StreetName: "лермонтова", House: "15"}},
		{"кокшетуқала абылайхана 3", map[Field]string{City: "Кокшетау", StreetName: "абылайхана", House: "3"}},
		// a stray letter between a city and its type word stays with the city
		{"кокшетауфқала абая 3", map[Field]string{City: "Кокшетау", StreetName: "абая", House: "3"}},
		{"павлодарркаласыкутузова 12", map[Field]string{City: "Павлодар", StreetName: "кутузова", House: "12"}},
		// the longest type word goes, a short street stays; "қаласатпаева" is қала and Сатпаева
		{"павлодарулицаабй 5", map[Field]string{City: "Павлодар", StreetType: "улица", StreetName: "абй", House: "5"}},
		{"алматықаласатпаева 10", map[Field]string{City: "Алматы", StreetName: "сатпаева", House: "10"}},
		{"алматықаласатпаевакөш 10", map[Field]string{City: "Алматы", StreetName: "сатпаева", House: "10"}},
		// a known street before a glued type word is not a mistyped city: Конаев
		{"алматы конаевакөш 12", map[Field]string{City: "Алматы", StreetName: "конаева", House: "12"}},
		// a street of the city that starts with a type word keeps it, also mistyped
		{"астанаулыдалла 3", map[Field]string{City: "Астана", StreetName: "улыдалла", House: "3"}},
		{"астанақаласыкошкарбайева 9", map[Field]string{City: "Астана", StreetName: "кошкарбайева", House: "9"}},
		// a city in capitals keeps its last letter before a lowercase street
		{"ПАВЛОДАРкутузова 4", map[Field]string{City: "Павлодар", StreetName: "кутузова", House: "4"}},
		{"СЕМЕЙАбая 2", map[Field]string{City: "Семей", StreetName: "Абая", House: "2"}},
		// a short type glued before a mistyped street that no city knows, in a word of its own after the city
		{"павлодар улкутузвоа 3", map[Field]string{City: "Павлодар", StreetType: "улица", StreetName: "кутузвоа", House: "3"}},
		{"кокшетау пркенесарыы 4", map[Field]string{City: "Кокшетау", StreetType: "проспект", StreetName: "кенесарыы", House: "4"}},
		{"павлодар б-рмирра 6", map[Field]string{City: "Павлодар", StreetType: "бульвар", StreetName: "мирра", House: "6"}},
		{"алматы пр-тсейфуллина 10", map[Field]string{City: "Алматы", StreetType: "проспект", StreetName: "сейфуллина", House: "10"}},
		// a mistyped type word and a street one letter from a known one stay whole
		{"алматы прспект абая 5", map[Field]string{City: "Алматы", StreetName: "абая", House: "5"}},
		{"актобе улиханова 7", map[Field]string{City: "Актобе", StreetName: "улиханова", House: "7"}},
		// a street glued to the second word of a city of two words
		{"отеген батырсадовая 5", map[Field]string{City: "Отеген батыр", StreetName: "садовая", House: "5"}},
		{"отеген батыравезова 8", map[Field]string{City: "Отеген батыр", StreetName: "авезова", House: "8"}},
		{"отегин батырсадовая 5", map[Field]string{City: "Отеген батыр", StreetName: "садовая", House: "5"}},
		// a city glued to a street of it one letter off: a short city name, an adjective-like street
		{"оралдостыкк 5", map[Field]string{City: "Уральск", StreetName: "достыкк", House: "5"}},
		{"петропавлинтернационалная 3", map[Field]string{City: "Петропавловск", StreetName: "интернационалная", House: "3"}},
		{"кокшетуабылайхна 9", map[Field]string{City: "Кокшетау", StreetName: "абылайхна", House: "9"}},
		{"павлдараба 5", map[Field]string{City: "Павлодар", StreetName: "аба", House: "5"}},
		{"тарзтолебии 5", map[Field]string{City: "Тараз", StreetName: "толебии", House: "5"}},
		// a short city before қала
		{"семеқала абая 4", map[Field]string{City: "Семей", StreetName: "абая", House: "4"}},
		// a short street after "ауыл", a street that reads as an adjective after "ул", a mistyped street
		// that starts with "пр"
		{"Талгарауылабй 3", map[Field]string{City: "Талгар", StreetName: "абй", House: "3"}},
		{"петропавловскулинтернациональная 7", map[Field]string{City: "Петропавловск", StreetType: "улица", StreetName: "интернациональная", House: "7"}},
		{"усть-каменогорскпрмитина 5", map[Field]string{City: "Усть-Каменогорск", StreetName: "прмитина", House: "5"}},
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
