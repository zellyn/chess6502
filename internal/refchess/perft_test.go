package refchess

import (
	"testing"
	"time"
)

// Published perft values: chessprogramming.org/Perft_Results. This is
// the mandatory correctness gate for the package: if these don't match
// exactly, the move generator has a bug (missing/extra move, wrong
// castling/en-passant/promotion handling, etc).
var perftCases = []struct {
	name   string
	fen    string
	counts []uint64 // counts[d-1] = perft(d)
}{
	{
		"startpos",
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		[]uint64{20, 400, 8902, 197281, 4865609},
	},
	{
		"kiwipete",
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		[]uint64{48, 2039, 97862, 4085603},
	},
	{
		"pos3-ep-pin",
		"8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",
		[]uint64{14, 191, 2812, 43238, 674624},
	},
	{
		"pos4-promo",
		"r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1",
		[]uint64{6, 264, 9467, 422333},
	},
	{
		"pos5",
		"rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 1 8",
		[]uint64{44, 1486, 62379, 2103487},
	},
	{
		"pos6",
		"r4rk1/1pp1qppp/p1np1n2/2b1p1B1/2B1P1b1/P1NP1N2/1PP1QPPP/R4RK1 w - - 0 10",
		[]uint64{46, 2079, 89890, 3894594},
	},
}

func TestPerft(t *testing.T) {
	start := time.Now()
	for _, tc := range perftCases {
		pos, err := ParseFEN(tc.fen)
		if err != nil {
			t.Fatalf("%s: ParseFEN: %v", tc.name, err)
		}
		for d, want := range tc.counts {
			depth := d + 1
			caseStart := time.Now()
			got := pos.Perft(depth)
			elapsed := time.Since(caseStart)
			if got != want {
				t.Errorf("%s perft(%d) = %d, want %d", tc.name, depth, got, want)
				continue
			}
			t.Logf("%s perft(%d) = %d ok (%s, %.0f nodes/s)",
				tc.name, depth, got, elapsed, float64(got)/elapsed.Seconds())
		}
	}
	t.Logf("total perft gate time: %s", time.Since(start))
}
