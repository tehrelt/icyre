package dsp

import "math"

// fft is an in-place iterative radix-2 FFT of a fixed power-of-two size.
type fft struct {
	n        int
	rev      []int
	cos, sin []float64
}

func newFFT(n int) *fft {
	bits := 0
	for 1<<bits < n {
		bits++
	}
	f := &fft{n: n, rev: make([]int, n), cos: make([]float64, n/2), sin: make([]float64, n/2)}
	for i := range f.rev {
		r := 0
		for b := 0; b < bits; b++ {
			r |= (i >> b & 1) << (bits - 1 - b)
		}
		f.rev[i] = r
	}
	for i := range f.cos {
		a := -2 * math.Pi * float64(i) / float64(n)
		f.cos[i], f.sin[i] = math.Cos(a), math.Sin(a)
	}
	return f
}

func (f *fft) transform(re, im []float64) {
	for i, j := range f.rev {
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}
	for size := 2; size <= f.n; size <<= 1 {
		half, step := size/2, f.n/size
		for start := 0; start < f.n; start += size {
			for k := 0; k < half; k++ {
				wr, wi := f.cos[k*step], f.sin[k*step]
				a, b := start+k, start+k+half
				tr := re[b]*wr - im[b]*wi
				ti := re[b]*wi + im[b]*wr
				re[b], im[b] = re[a]-tr, im[a]-ti
				re[a], im[a] = re[a]+tr, im[a]+ti
			}
		}
	}
}
