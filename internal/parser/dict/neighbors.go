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
// number: most buildings of the nearest number below and of the nearest number above it on the same
// side of the street (same parity) must lie in one district. A street is often itself the border, so
// the other side does not count, and a house past the last known number gets nothing. A building deep
// in the block may lie across the border ("Розыбакиева 37а" of three 37s), hence the majority.
func NeighborDistrict(list []Numbered, n int) (string, bool) {
	var below, above []string // districts of the buildings of the nearest number on each side
	belowN, aboveN := -1, -1
	for _, x := range list {
		if x.N%2 != n%2 {
			continue
		}
		if x.N <= n { // the list is sorted: the last number at or below n is the nearest
			if x.N != belowN {
				belowN, below = x.N, nil
			}
			below = append(below, x.District)
		}
		if x.N >= n && (aboveN < 0 || x.N == aboveN) {
			aboveN = x.N
			above = append(above, x.District)
		}
	}
	b, okB := majority(below)
	a, okA := majority(above)
	if !okB || !okA || a != b {
		return "", false
	}
	return a, true
}

// majority is the district of more than half of the buildings.
func majority(ds []string) (string, bool) {
	n := map[string]int{}
	for _, d := range ds {
		n[d]++
		if n[d]*2 > len(ds) {
			return d, true
		}
	}
	for d, k := range n {
		if k*2 > len(ds) {
			return d, true
		}
	}
	return "", false
}
