package text

// MaxFuzzyLen bounds the words compared by edit distance; longer words are matched exactly only.
const MaxFuzzyLen = 48

// ToRunes decodes s into dst without allocating when s fits.
func ToRunes(s string, dst []rune) []rune {
	for _, c := range s {
		dst = append(dst, c)
	}
	return dst
}

// Distance is the optimal string alignment (Damerau-Levenshtein with adjacent transpositions).
func Distance(a, b []rune) int { return DistanceLimit(a, b, 2) }

// DistanceLimit returns the distance, or limit+1 as soon as it is known to exceed limit.
// It does not allocate for words shorter than MaxFuzzyLen.
func DistanceLimit(a, b []rune, limit int) int {
	if d := len(a) - len(b); d > limit || d < -limit {
		return limit + 1
	}
	var rows [3][MaxFuzzyLen + 1]int
	var prev2, prev, cur []int
	if len(b) < MaxFuzzyLen {
		prev2, prev, cur = rows[0][:len(b)+1], rows[1][:len(b)+1], rows[2][:len(b)+1]
	} else {
		prev2, prev, cur = make([]int, len(b)+1), make([]int, len(b)+1), make([]int, len(b)+1)
	}
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		rowMin := i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				cur[j] = min(cur[j], prev2[j-2]+1)
			}
			rowMin = min(rowMin, cur[j])
		}
		// a transposition reaches back two rows, so stop only when both rows are over the limit
		if rowMin > limit && i > 1 && minOf(prev) > limit {
			return limit + 1
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return min(prev[len(b)], limit+1)
}

func minOf(xs []int) int {
	m := xs[0]
	for _, x := range xs[1:] {
		m = min(m, x)
	}
	return m
}
