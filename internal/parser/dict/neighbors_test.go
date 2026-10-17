package dict

import "testing"

func TestNeighborDistrict(t *testing.T) {
	list := []Numbered{{233, "Б"}, {237, "Б"}, {240, "Б"}, {242, "А"}, {247, "Б"}, {251, "А"}, {260, "А"}}
	SortNumbered(list)
	for _, tc := range []struct {
		n    int
		want string
	}{
		{241, "Б"}, // 237 and 247 on the odd side
		{249, ""},  // 247 and 251 disagree
		{244, "А"}, // 242 and 260 on the even side; 247 across the street does not count
		{300, ""},  // past the last number
		{235, "Б"},
	} {
		if got, _ := NeighborDistrict(list, tc.n); got != tc.want {
			t.Errorf("house %d: %q, want %q", tc.n, got, tc.want)
		}
	}

	// buildings of the nearest number in two districts: no answer
	mixed := []Numbered{{17, "Б"}, {17, "Н"}, {17, "Б"}, {23, "Н"}}
	SortNumbered(mixed)
	if d, ok := NeighborDistrict(mixed, 19); ok {
		t.Errorf("house 19 between mixed 17 and 23: %q", d)
	}

	for h, want := range map[string]int{"241": 241, "17/13": 17, "5а": 5} {
		if n, ok := HouseNumber(h); !ok || n != want {
			t.Errorf("HouseNumber(%q) = %d", h, n)
		}
	}
	if _, ok := HouseNumber("а5"); ok {
		t.Error("HouseNumber(а5)")
	}
}
