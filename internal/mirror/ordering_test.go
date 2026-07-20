package mirror

import "testing"

// TestSEE checks the static exchange evaluation against hand-computed and
// canonical (Stockfish see.cpp) exchange values.
func TestSEE(t *testing.T) {
	cases := []struct {
		fen  string
		move string // UCI
		want int32
	}{
		// Undefended pawn grab: PxP wins a clean pawn.
		{"4k3/8/8/3p4/4P3/8/8/4K3 w - - 0 1", "e4d5", 100},
		// Canonical: Rxe5 wins the (undefended) e5 pawn.
		{"1k1r4/1pp4p/p7/4p3/8/P5P1/1PP4P/2K1R3 w - - 0 1", "e1e5", 100},
		// Canonical Nxe5: knight-for-pawn exchange that runs against
		// white. Stockfish reports -200 with its piece values; with our
		// vicVal (N=320,B=330,R=500,Q=975) the full swap folds to -220.
		{"1k1r3q/1ppn3p/p4b2/4p3/8/P2N2P1/1PP1R1BP/2K1Q3 w - - 0 1", "d3e5", -220},
		// Queen captures a pawn defended by a pawn: QxP then PxQ, so
		// SEE = pawn - queen = 100 - 975.
		{"3k4/8/2p5/3p4/8/8/8/3QK3 w - - 0 1", "d1d5", 100 - 975},
	}
	for _, tc := range cases {
		pos, err := ParseFEN(tc.fen)
		if err != nil {
			t.Fatal(err)
		}
		eng := NewEngine()
		eng.SetPosition(pos)
		var found bool
		for _, m := range eng.generate(false) {
			if m.UCI() == tc.move {
				got := eng.seeValue(m)
				if got != tc.want {
					t.Errorf("%s %s: SEE = %d, want %d", tc.fen, tc.move, got, tc.want)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: move %s not generated", tc.fen, tc.move)
		}
	}
}

// TestOrderingNodes: depth-6 node counts for the move-ordering variants
// vs the FtAll baseline (task #35, phase A). Better ordering means more
// cutoffs on the first move, so fewer nodes at the same depth.
func TestOrderingNodes(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	variants := []struct {
		name string
		feat byte
		ord  OrderParams
	}{
		{"baseline (asm 5-pass)", FtAll, OrderParams{}},
		{"SEE", FtAll | FtSEE, OrderParams{}},
		{"SEE+losinglast", FtAll | FtSEE, OrderParams{LosingLast: true}},
		{"history", FtAll | FtHistory, OrderParams{}},
		{"history+malus", FtAll | FtHistory, OrderParams{HistMalus: true}},
		{"SEE+history", FtAll | FtSEE | FtHistory, OrderParams{}},
		{"SEE+history+malus+ll", FtAll | FtSEE | FtHistory, OrderParams{HistMalus: true, LosingLast: true}},
	}
	fens := benchFENs(t)
	var baseTotal uint64
	for vi, v := range variants {
		var total uint64
		for _, fen := range fens {
			pos, err := ParseFEN(fen)
			if err != nil {
				t.Fatal(err)
			}
			eng := NewEngine()
			eng.Features = v.feat
			eng.Ord = v.ord
			eng.SetPosition(pos)
			eng.SearchFixed(6)
			total += eng.Nodes
		}
		if vi == 0 {
			baseTotal = total
		}
		t.Logf("%-24s total %9d nodes (%+.1f%% vs base)",
			v.name, total, 100*(float64(total)/float64(baseTotal)-1))
	}
}

// TestOrderingScoreParity: with the heuristic pruners OFF (pure fail-hard
// alpha-beta: pstruct eval + TT only, no LMR/futility/null), move
// reordering must not change the minimax value. A mismatch would flag a
// soundness bug in the ordered loop (dropped/duplicated/illegal move).
// (With LMR/futility on, ordering legitimately changes the *reduced*
// tree's score — that is the whole point of task #35, so it is NOT a
// parity condition.)
func TestOrderingScoreParity(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	const base = FtPstruct | FtKiller // pure ab + killers, no LMR/futil/null
	masks := [4]byte{base, base | FtSEE, base | FtHistory, base | FtSEE | FtHistory}
	for _, fen := range benchFENs(t) {
		var scores [4]int
		for i, mask := range masks {
			pos, err := ParseFEN(fen)
			if err != nil {
				t.Fatal(err)
			}
			eng := NewEngine()
			eng.Features = mask
			eng.SetPosition(pos)
			_, sc := eng.SearchFixed(6)
			scores[i] = sc
		}
		for i := 1; i < 4; i++ {
			if scores[i] != scores[0] {
				t.Errorf("%s: score mask %#x = %d, baseline = %d",
					fen, masks[i], scores[i], scores[0])
			}
		}
	}
}
