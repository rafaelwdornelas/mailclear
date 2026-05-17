// Package typo implementa detecção de typos via Damerau-Levenshtein com bucketing.
package typo

// DamerauLevenshtein devolve a distância entre a e b com até 1 transposição
// adjacente (Optimal String Alignment). Iterativo, O(len(a)*len(b)) em tempo
// e O(len(b)) em espaço.
func DamerauLevenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev2 := make([]int, lb+1)
	prev1 := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev1[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			del := prev1[j] + 1
			ins := curr[j-1] + 1
			sub := prev1[j-1] + cost
			curr[j] = min3(del, ins, sub)

			// Transposição (Optimal String Alignment)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				if t := prev2[j-2] + 1; t < curr[j] {
					curr[j] = t
				}
			}
		}
		prev2, prev1, curr = prev1, curr, prev2
	}
	return prev1[lb]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
