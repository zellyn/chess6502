package chesstest

import "testing"

// TestIterativeDeepening runs budget mode: ID from depth 1 under a
// cycle budget, depth-capped, falling back safely on abort.
func TestIterativeDeepening(t *testing.T) {
	// Mate-in-2 under a generous budget: must still find the mate.
	pos, err := ParseFEN("k7/8/2K5/8/8/8/8/6R1 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	res, err := SearchBudget(loadEngine(t), defs, pos, 10, 20_000_000, 200_000_000)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("kr-mate-in-2 budget: move=%s score=%d cycles=%d", res.Move, res.Score, res.Cycles)
	if res.Move != "c6b6" || res.Score != mate-3 {
		t.Errorf("got %s/%d, want c6b6/%d", res.Move, res.Score, mate-3)
	}

	// Startpos with a ~10-emulated-second budget: any sane move; the
	// run must respect the hard abort (cycles < ~2.2x budget).
	pos, err = ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	budget := uint64(10_000_000)
	res, err = SearchBudget(loadEngine(t), defs, pos, 24, budget, 60_000_000)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("startpos budget=%dM: move=%s score=%d cycles=%dM", budget/1e6, res.Move, res.Score, res.Cycles/1e6)
	if res.Move == "" || res.Score < -200 || res.Score > 200 {
		t.Errorf("implausible: %q %d", res.Move, res.Score)
	}
	if res.Cycles > budget*5/2 {
		t.Errorf("hard abort failed: %d cycles for budget %d", res.Cycles, budget)
	}
}
