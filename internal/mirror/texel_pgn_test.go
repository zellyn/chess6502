package mirror

import (
	"math/rand/v2"
	"path/filepath"
	"sort"
	"sync"
	"testing"
)

// weightNames label the 10 tunable parameters in weightsVec order.
var weightNames = [10]string{
	"doubled", "isolated", "passed2", "passed3", "passed4",
	"passed5", "passed6", "passed7", "shield", "openfile",
}

// tuneVec runs FitK + Tune on rows and returns the tuned parameter
// vector (weightsVec layout) and the fitted K.
func tuneVec(rows []Row) ([10]float64, float64) {
	k := FitK(rows, DefaultWeights)
	tuned, _, _ := Tune(rows, DefaultWeights, k, nil)
	return weightsVec(tuned), k
}

// TestPGNCorpusTune folds the rating-pool (non-self-play) PGN positions
// into the self-play Texel corpus, re-tunes, and bootstraps a 95% CI on
// every weight so we can tell which moves are real vs resampling noise.
// Self-play-only tune is the reference (reproduces TunedWeights).
func TestPGNCorpusTune(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	selfRows, err := LoadRows("testdata/texel-rows-2026-07-18.gz")
	if err != nil {
		t.Fatalf("load self-play corpus: %v", err)
	}

	pgns, _ := filepath.Glob("../../tools/pgn/pool_c96f604_*.pgn")
	if len(pgns) == 0 {
		t.Skip("no pool PGNs")
	}
	var poolSamples []Sample
	var poolStats PGNStats
	for _, p := range pgns {
		s, st, err := PGNSamples(p, 1)
		if err != nil {
			t.Fatalf("PGNSamples %s: %v", p, err)
		}
		poolSamples = append(poolSamples, s...)
		poolStats.Games += st.Games
		poolStats.Skipped += st.Skipped
		poolStats.Samples += st.Samples
	}
	poolRows := SampleRows(poolSamples)
	t.Logf("pool PGNs: %d files, %d games, %d skipped, %d quiet rows",
		len(pgns), poolStats.Games, poolStats.Skipped, poolStats.Samples)

	combined := make([]Row, 0, len(selfRows)+len(poolRows))
	combined = append(combined, selfRows...)
	combined = append(combined, poolRows...)
	t.Logf("corpus: self-play %d + pool %d = %d rows",
		len(selfRows), len(poolRows), len(combined))

	selfVec, selfK := tuneVec(selfRows)
	combVec, combK := tuneVec(combined)
	t.Logf("self-play tune:  K=%.2f  %s", selfK, fmtVec(selfVec))
	t.Logf("combined tune:   K=%.2f  %s", combK, fmtVec(combVec))

	// Bootstrap the combined corpus: resample rows with replacement,
	// re-tune, and take percentile CIs per weight.
	const B = 200
	samples := make([][10]float64, B)
	var wg sync.WaitGroup
	sem := make(chan struct{}, 7)
	for b := 0; b < B; b++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(b int) {
			defer wg.Done()
			defer func() { <-sem }()
			rnd := rand.New(rand.NewPCG(0xb007+uint64(b), 0x5eed))
			res := make([]Row, len(combined))
			for i := range res {
				res[i] = combined[rnd.IntN(len(combined))]
			}
			v, _ := tuneVec(res)
			samples[b] = v
		}(b)
	}
	wg.Wait()

	t.Logf("bootstrap B=%d, 95%% CI per weight (self -> combined [lo,hi]):", B)
	for j := 0; j < 10; j++ {
		col := make([]float64, B)
		for b := 0; b < B; b++ {
			col[b] = samples[b][j]
		}
		sort.Float64s(col)
		lo := col[int(0.025*float64(B))]
		hi := col[int(0.975*float64(B))]
		mean := 0.0
		for _, x := range col {
			mean += x
		}
		mean /= float64(B)
		// A weight "moves meaningfully" if the self-play value sits
		// outside the combined bootstrap 95% CI.
		flag := ""
		if selfVec[j] < lo || selfVec[j] > hi {
			flag = "  <-- MOVED (self outside CI)"
		}
		t.Logf("  %-9s %5.1f -> %5.1f  mean %5.1f  CI [%5.1f, %5.1f]%s",
			weightNames[j], selfVec[j], combVec[j], mean, lo, hi, flag)
	}
}

func fmtVec(v [10]float64) string {
	w := vecWeights(v)
	return "{" +
		sprintWeights(w) + "}"
}

func sprintWeights(w Weights) string {
	return "D:" + itoa(w.Doubled) + " I:" + itoa(w.Isolated) +
		" P:[" + itoa(w.Passed[1]) + "," + itoa(w.Passed[2]) + "," +
		itoa(w.Passed[3]) + "," + itoa(w.Passed[4]) + "," +
		itoa(w.Passed[5]) + "," + itoa(w.Passed[6]) + "]" +
		" S:" + itoa(w.Shield) + " O:" + itoa(w.OpenFile)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
