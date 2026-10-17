package parser

import "testing"

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
