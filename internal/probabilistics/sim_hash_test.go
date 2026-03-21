package probabilistic

func seedOf(b byte, n int) []byte {
	if n <= 0 {
		n = 32
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}
