package dict

import "sort"

// Numbered is a building of a street with its leading house number: "247/3" and "247а" are 247.
type Numbered struct {
	N        int
	District string
}

// HouseNumber is the leading number of a house: "241" -> 241, "17/13" -> 17, "5а" -> 5.
func HouseNumber(h string) (int, bool) {
	n, digits := 0, 0
	for _, r := range h {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
		digits++
		if digits > 6 {
			return 0, false
		}
	}
	return n, digits > 0
}

// SortNumbered orders the buildings of one street by number.
func SortNumbered(list []Numbered) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].N != list[j].N {
			return list[i].N < list[j].N
		}
		return list[i].District < list[j].District
	})
}

// NeighborDistrict is the district of house n from the known buildings of its street, sorted by
// number: every building of the nearest number below and of the nearest number above it on the same
// side of the street (same parity) must lie in one district. A street is often itself the border, so
// the other side does not count; a house past the last known number, or between numbers that two
// streets of one name share ("Жандосова" of Ауэзовский and of Наурызбайский), gets nothing.
func NeighborDistrict(list []Numbered, n int) (string, bool) {
	belowN, aboveN := -1, -1
	below, above := "", ""
	belowOK, aboveOK := true, true
	for _, x := range list {
		if x.N%2 != n%2 {
			continue
		}
		if x.N <= n { // the list is sorted: the last number at or below n is the nearest
			if x.N != belowN {
				belowN, below, belowOK = x.N, x.District, true
			} else if x.District != below {
				belowOK = false
			}
		}
		if x.N >= n {
			if aboveN < 0 {
				aboveN, above = x.N, x.District
			} else if x.N == aboveN && x.District != above {
				aboveOK = false
			}
		}
	}
	if belowN < 0 || aboveN < 0 || !belowOK || !aboveOK || below != above {
		return "", false
	}
	return below, true
}
