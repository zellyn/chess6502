package mirror

import (
	"path/filepath"
	"testing"
)

// TestKingBucketTune exercises the king-bucketed PSQT tuner end to end
// (task #30). It is also the regression guard for the AdamW decoupled-
// weight-decay fix: with L2 folded into the gradient instead, Adam's
// per-parameter normalization amplifies it into a restoring force that
// pins every delta to zero (train loss frozen). Here we assert the
// tuner actually descends and produces bounded, storable deltas.
//
// VERDICT (see docs/results.md): despite a large Texel-loss drop, the
// bucketed eval LOSES 44-102 Elo at depth 6 self-play (scaling with
// delta magnitude) — Texel loss and playing strength diverge; the
// 2560-param model overfits self-play result-correlation. King-file
// bucketed PSQT does not carry its 6502 storage weight.
func TestKingBucketTune(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	rows, err := ReadFenRows("testdata/fenrows-2026-07-19.gz")
	if err != nil {
		t.Fatalf("load FEN corpus: %v", err)
	}
	if pgns, _ := filepath.Glob("../../tools/pgn/pool_c96f604_*.pgn"); len(pgns) > 0 {
		for _, p := range pgns {
			r, _, err := PGNFenRows(p)
			if err != nil {
				t.Fatalf("pool %s: %v", p, err)
			}
			rows = append(rows, r...)
		}
	}
	t.Logf("king-bucket corpus: %d positions", len(rows))

	exs, err := BuildKBExamples(rows, TunedWeights, 7)
	if err != nil {
		t.Fatal(err)
	}
	k := FitKBK(exs)
	_, kb, before, after := TuneKB(exs, k, 0.05, 0.5, 300, nil)
	maxAbs, nz := KBStats(kb)
	t.Logf("K=%.2f  train loss %.6f -> %.6f  deltas: %d nonzero, max |%d|",
		k, before, after, nz, maxAbs)

	if after >= before {
		t.Errorf("tuner did not descend: %.6f -> %.6f (AdamW decay regression?)", before, after)
	}
	if before-after < 0.001 {
		t.Errorf("descent too small (%.6f); optimizer likely stalled", before-after)
	}
	if maxAbs == 0 {
		t.Errorf("all deltas zero — optimizer produced no signal")
	}
	if maxAbs > 127 {
		t.Errorf("max |delta| = %d exceeds signed-byte range", maxAbs)
	}
}
