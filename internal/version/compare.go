package version

import "strings"

// Compare returns -1 if a < b, 0 if equal, 1 if a > b.
// "v" prefix is ignored; missing parts are treated as zero (0.1 == 0.1.0).
func Compare(a, b string) int {
	pa := versionParts(a)
	pb := versionParts(b)
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	}
	return 0
}

func versionParts(v string) []int {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(strings.ToLower(v), "v")
	if v == "" {
		return []int{0}
	}
	chunks := strings.Split(v, ".")
	out := make([]int, 0, len(chunks))
	for _, chunk := range chunks {
		n := 0
		for _, c := range chunk {
			if c < '0' || c > '9' {
				break
			}
			n = n*10 + int(c-'0')
		}
		out = append(out, n)
	}
	return out
}
