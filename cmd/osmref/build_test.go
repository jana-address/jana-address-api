package main

import (
	"reflect"
	"testing"
)

func TestAddresses(t *testing.T) {
	for _, tc := range []struct {
		tags map[string]string
		want []address
	}{
		// one street in two languages: one address, the spellings are joined
		{map[string]string{"addr:street": "проспект Абая", "addr:street:kk": "Абай даңғылы", "addr:housenumber": "10"},
			[]address{{[]string{"проспект Абая", "Абай даңғылы"}, "10"}}},
		// a corner building: two streets, each with its number
		{map[string]string{"addr:street": "улица Комиссарова;проспект Бухар-жырау", "addr:housenumber": "37;66"},
			[]address{{[]string{"улица Комиссарова"}, "37"}, {[]string{"проспект Бухар-жырау"}, "66"}}},
		// the numbers do not pair up: the streets stay apart
		{map[string]string{"addr:street": "Балпык би;улица Жансугурова", "addr:housenumber": "84;84а;40"},
			[]address{{[]string{"Балпык би"}, "84;84а;40"}, {[]string{"улица Жансугурова"}, "84;84а;40"}}},
	} {
		if got := addresses(tc.tags); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("addresses(%v) = %v, want %v", tc.tags, got, tc.want)
		}
	}
}
