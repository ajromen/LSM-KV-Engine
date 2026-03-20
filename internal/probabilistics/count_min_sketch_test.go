package probabilistics

import (
	"log"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/ajromen/LSM-KV-Engine/internal/config"
)

// OVO VAM JE BRACO TEST ZA ACCURACY, GLAVNI TEST
// ON TESTIRA ADD I ESTIMATE FUNKCIJE
func TestAccuracy(t *testing.T) {
	log.SetOutput(os.Stdout)

	// PRAVIMO CMS KAO NA KODU SA GITHUBA
	cfg := config.CountMinSketchConfig{
		Accuracy:   0.0001,
		Confidence: 0.9999,
		Seeds: [][]byte{
			[]byte("seed-1"),
			[]byte("seed-2"),
			[]byte("seed-3"),
			[]byte("seed-4"),
			[]byte("seed-5"),
			[]byte("seed-6"),
			[]byte("seed-7"),
			[]byte("seed-8"),
			[]byte("seed-9"),
			[]byte("seed-10"),
		},
	}

	cms := NewCountMinSketch(cfg)

	// ITERIRAMO MNOGO VREDNOSTI
	iterations := 5500
	var diverged int // BROJI KOLIKO ESTIMATE PREMASHUJE REALNU VREDNOST
	for i := 1; i < iterations; i++ {
		v := uint(i % 50)
		key := []byte(strconv.Itoa(i))

		cms.Add(key, v)         // DODAJEMO VREDNOST U CMS
		vv := cms.Estimate(key) // PROCENJUJEMO VREDNOST

		if vv > v {
			diverged++ // AKO JE PROCENA VECA OD REALNE, POVECAJ BROJAC
		}
	}

	// PROVERA KOJI SU PROMASENI
	var miss int
	for i := 1; i < iterations; i++ {
		key := []byte(strconv.Itoa(i))
		vv := uint(i % 50)

		v := cms.Estimate(key)
		if v < vv {
			t.Errorf("estimate smaller than actual for key %s: got %d, want %d", key, v, vv)
		}
		if v != vv {
			log.Printf("real: %d, estimate: %d\n", vv, v)
			miss++
		}
	}

	log.Printf("missed %d of %d (%d diverged during adds)", miss, iterations, diverged)
}

// OVO JE BRACO BENCHMARK ZA ADD FUNKCIJU
func BenchmarkAdd(b *testing.B) {
	cfg := config.CountMinSketchConfig{
		Accuracy:   0.001,
		Confidence: 0.999,
		Seeds: [][]byte{
			[]byte("seed-1"),
			[]byte("seed-2"),
			[]byte("seed-3"),
			[]byte("seed-4"),
			[]byte("seed-5"),
			[]byte("seed-6"),
			[]byte("seed-7"),
			[]byte("seed-8"),
			[]byte("seed-9"),
			[]byte("seed-10"),
		},
	}

	cms := NewCountMinSketch(cfg)

	for i := 0; i < b.N; i++ {
		cms.Add([]byte(strconv.Itoa(rand.Int())), uint(rand.Int()%100)) // DODAJ RANDOM VREDNOST
	}
}

// OVO JE BRACO BENCHMARK ZA ESTIMATE FUNKCIJU
func BenchmarkEstimate(b *testing.B) {
	cfg := config.CountMinSketchConfig{
		Accuracy:   0.001,
		Confidence: 0.999,
		Seeds: [][]byte{
			[]byte("seed-1"),
			[]byte("seed-2"),
			[]byte("seed-3"),
			[]byte("seed-4"),
			[]byte("seed-5"),
			[]byte("seed-6"),
			[]byte("seed-7"),
			[]byte("seed-8"),
			[]byte("seed-9"),
			[]byte("seed-10"),
		},
	}

	cms := NewCountMinSketch(cfg)

	for i := 0; i < b.N; i++ {
		cms.Estimate([]byte(strconv.Itoa(rand.Int()))) // PROCENJUJ RANDOM KEY
	}
}
