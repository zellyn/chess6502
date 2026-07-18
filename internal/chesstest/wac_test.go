package chesstest

import "testing"

// A small Win-At-Chess subset as a tactical baseline. This is a
// RECORDED BASELINE, not a gate: the test logs the solve rate at fixed
// depth so search changes show their tactical effect over time. Best
// moves should be re-verified against the canonical WAC epd before ever
// being promoted to a hard gate.
var wacCases = []struct {
	name string
	fen  string
	bm   string
}{
	{"WAC.001", "2rr3k/pp3pp1/1nnqbN1p/3pN3/2pP4/2P3Q1/PPB4P/R4RK1 w - - 0 1", "g3g6"},
	{"WAC.002", "8/7p/5k2/5p2/p1p2P2/Pr1pPK2/1P1R3P/8 b - - 0 1", "b3b2"},
	{"WAC.004", "r1bq2rk/pp3pbp/2p1p1pQ/7P/3P4/2PB1N2/PP3PPR/2KR4 w - - 0 1", "h6h7"},
	{"WAC.005", "5k2/6pp/p1qN4/1p1p4/3P4/2PKP2Q/PP3r2/3R4 b - - 0 1", "c6c4"},
	{"WAC.008", "r4q1k/p2bR1rp/2p2Q1N/5p2/5p2/2P5/PP3PPP/R5K1 w - - 0 1", "e7f7"},
	{"WAC.010", "2br2k1/2q3rn/p2NppQ1/2p1P3/Pp5R/4P3/1P3PPP/3R2K1 w - - 0 1", "h4h7"},
	{"WAC.012", "4k1r1/2p3r1/1pR1p3/3pP2p/3P2qP/P4N2/1PQ4P/5RK1 b - - 0 1", "g4f3"},
}

func TestWACBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("slow: tactical searches")
	}
	const depth = 4
	solved := 0
	for _, tc := range wacCases {
		pos, err := ParseFEN(tc.fen)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		res, err := SearchMove(loadEngine(t), defs, pos, depth, 60_000_000_000)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		ok := res.Move == tc.bm
		if ok {
			solved++
		}
		t.Logf("%s: got %s (score %d, %dM cycles) want %s %v",
			tc.name, res.Move, res.Score, res.Cycles/1_000_000, tc.bm, ok)
	}
	t.Logf("WAC baseline at depth %d: %d/%d solved", depth, solved, len(wacCases))
}
