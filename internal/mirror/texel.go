package mirror

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sync"
)

// GenerateData plays fixed-depth self-play games (all features on,
// dither on, both sides using weights w) and collects quiet labeled
// positions until at least minSamples are gathered.
func GenerateData(openings [][]string, w Weights, depth, minSamples, workers int, seed uint64, progress func(games, samples int)) ([]Sample, error) {
	cfg := PlayerCfg{Features: FtNull | FtKiller | FtFutil | FtPstruct, Weights: w, Depth: depth}
	var mu sync.Mutex
	var samples []Sample
	games := 0
	var firstErr error
	stop := false

	var wg sync.WaitGroup
	if workers <= 0 {
		workers = 1
	}
	for wk := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rnd := rand.New(rand.NewPCG(seed^0xda7a6e2, uint64(wk)*0x9e3779b97f4a7c15+1))
			for {
				mu.Lock()
				if stop || firstErr != nil {
					mu.Unlock()
					return
				}
				g := games
				games++
				mu.Unlock()

				opening := openings[g%len(openings)]
				rec, err := PlayGame(cfg, cfg, opening, rnd, true)

				mu.Lock()
				if err != nil {
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					return
				}
				samples = append(samples, rec.Samples...)
				if progress != nil && games%50 == 0 {
					progress(games, len(samples))
				}
				if len(samples) >= minSamples {
					stop = true
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return samples, firstErr
}

// weightsVec flattens the tunable parameters: doubled, isolated,
// passed[1..6], shield, openfile (10 values).
func weightsVec(w Weights) [10]float64 {
	return [10]float64{
		float64(w.Doubled), float64(w.Isolated),
		float64(w.Passed[1]), float64(w.Passed[2]), float64(w.Passed[3]),
		float64(w.Passed[4]), float64(w.Passed[5]), float64(w.Passed[6]),
		float64(w.Shield), float64(w.OpenFile),
	}
}

// vecWeights rounds a parameter vector back to integer Weights,
// clamping to the asm's unsigned-byte range.
func vecWeights(v [10]float64) Weights {
	clamp := func(x float64) int {
		n := int(math.Round(x))
		if n < 0 {
			n = 0
		}
		if n > 255 {
			n = 255
		}
		return n
	}
	return Weights{
		Doubled:  clamp(v[0]),
		Isolated: clamp(v[1]),
		Passed:   [8]int{0, clamp(v[2]), clamp(v[3]), clamp(v[4]), clamp(v[5]), clamp(v[6]), clamp(v[7]), 0},
		Shield:   clamp(v[8]),
		OpenFile: clamp(v[9]),
	}
}

// featVec extracts the per-sample feature multipliers matching
// weightsVec's layout (white minus black, with the signs pawnFeatures.
// dot applies).
func featVec(f *pawnFeatures) [10]float64 {
	return [10]float64{
		-float64(f.doubledW - f.doubledB),
		-float64(f.isolatedW - f.isolatedB),
		float64(f.passedW[1] - f.passedB[1]),
		float64(f.passedW[2] - f.passedB[2]),
		float64(f.passedW[3] - f.passedB[3]),
		float64(f.passedW[4] - f.passedB[4]),
		float64(f.passedW[5] - f.passedB[5]),
		float64(f.passedW[6] - f.passedB[6]),
		float64(f.shieldW - f.shieldB),
		-float64(f.openW - f.openB),
	}
}

func sigmoid(evalCp, k float64) float64 {
	return 1 / (1 + math.Pow(10, -k*evalCp/400))
}

// texelLoss is the mean squared error of the logistic prediction over
// the samples for parameter vector v.
func texelLoss(samples []Sample, feats [][10]float64, v [10]float64, k float64) float64 {
	sum := 0.0
	for i := range samples {
		eval := float64(samples[i].Base)
		for j, fj := range feats[i] {
			eval += fj * v[j]
		}
		d := samples[i].R - sigmoid(eval, k)
		sum += d * d
	}
	return sum / float64(len(samples))
}

// FitK finds the sigmoid scale minimizing the loss with the given
// weights fixed (coarse grid + refinement).
func FitK(samples []Sample, w Weights) float64 {
	feats := make([][10]float64, len(samples))
	for i := range samples {
		feats[i] = featVec(&samples[i].F)
	}
	v := weightsVec(w)
	bestK, bestL := 1.0, math.Inf(1)
	for k := 0.4; k <= 2.4; k += 0.05 {
		if l := texelLoss(samples, feats, v, k); l < bestL {
			bestK, bestL = k, l
		}
	}
	return bestK
}

// Tune runs multi-scale integer coordinate descent on the pawn-
// structure weights, PeSTO fixed. Returns the tuned weights and the
// loss before/after.
func Tune(samples []Sample, start Weights, k float64, progress func(step string, loss float64)) (Weights, float64, float64) {
	feats := make([][10]float64, len(samples))
	for i := range samples {
		feats[i] = featVec(&samples[i].F)
	}
	v := weightsVec(start)
	lossBefore := texelLoss(samples, feats, v, k)
	best := lossBefore
	for _, step := range []float64{32, 16, 8, 4, 2, 1} {
		improved := true
		for improved {
			improved = false
			for j := range v {
				for _, dir := range []float64{step, -step} {
					cand := v
					cand[j] += dir
					if cand[j] < 0 {
						cand[j] = 0
					}
					if cand[j] > 255 {
						cand[j] = 255
					}
					if cand[j] == v[j] {
						continue
					}
					if l := texelLoss(samples, feats, cand, k); l < best {
						v, best = cand, l
						improved = true
					}
				}
			}
		}
		if progress != nil {
			progress(fmt.Sprintf("step %g", step), best)
		}
	}
	return vecWeights(v), lossBefore, best
}
