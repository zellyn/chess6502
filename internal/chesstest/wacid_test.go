package chesstest

import "testing"

// TestWACDeepening runs the WAC subset in iterative-deepening mode with
// the same depth cap as the fixed-depth baseline: ID + TT-move ordering
// should complete depth 4 in far fewer cycles than a cold fixed-depth
// search, despite doing depths 1-3 first.
func TestWACDeepening(t *testing.T) {
	if testing.Short() {
		t.Skip("slow: tactical searches")
	}
	solved := 0
	var total uint64
	for _, tc := range wacCases {
		pos, err := ParseFEN(tc.fen)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		// Huge budget: every iteration up to the cap completes.
		res, err := SearchBudget(loadEngine(t), defs, pos, 4, 100_000_000_000, 200_000_000_000)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		ok := res.Move == tc.bm
		if ok {
			solved++
		}
		total += res.Cycles
		t.Logf("%s: got %s (score %d, %dM cycles) want %s %v",
			tc.name, res.Move, res.Score, res.Cycles/1_000_000, tc.bm, ok)
	}
	t.Logf("WAC ID-to-depth-4: %d/%d solved, %dM cycles total", solved, len(wacCases), total/1_000_000)
}
