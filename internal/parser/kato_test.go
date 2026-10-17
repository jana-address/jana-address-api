package parser

import (
	"testing"

	"github.com/jana-address/jana-address-api/internal/kato"
)

func TestKATOSettlements(t *testing.T) {
	recs, _, err := kato.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	old := current.Load()
	defer current.Store(old)
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
		// a Kazakh rayon name
		{"Іле ауданы, Өтеген батыр ауылы, Нұрлы көшесі 17", map[Field]string{City: "Отеген батыр", District: "Илийский"}},
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
