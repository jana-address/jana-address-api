package parser

import (
	"testing"

	"github.com/jana-address/jana-address-api/internal/kato"
	"github.com/jana-address/jana-address-api/internal/parser/dict"
)

// TestGluedCity: a city glued to the street, the city or the street mistyped.
func TestGluedCity(t *testing.T) {
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
		// a street one letter off that starts with the city's last letter: Есик and Кунаева
		{"есиккнаева 12", map[Field]string{City: "Есик", StreetName: "кнаева", House: "12"}},
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
