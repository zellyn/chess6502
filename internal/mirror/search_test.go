package mirror

import (
	"math/rand/v2"
	"testing"
)

// TestIncrementalConsistency random-walks make/unmake, verifying the
// incremental accumulators (MG/EG/Phase/Hash/PStruct) always match a
// from-scratch evalinit — the mirror of the asm hash-consistency test.
func TestIncrementalConsistency(t *testing.T) {
	rnd := rand.New(rand.NewPCG(7, 9))
	for game := range 10 {
		pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
		if err != nil {
			t.Fatal(err)
		}
		eng := NewEngine()
		eng.SetPosition(pos)
		for ply := range 80 {
			moves := legalMoves(eng, &eng.Pos)
			if len(moves) == 0 {
				break
			}
			eng.make(moves[rnd.IntN(len(moves))])
			cur := eng.Pos
			check := NewEngine()
			check.SetPosition(&cur)
			if check.Pos.MG != cur.MG || check.Pos.EG != cur.EG ||
				check.Pos.Phase != cur.Phase || check.Pos.Hash != cur.Hash ||
				check.Pos.PStruct != cur.PStruct {
				t.Fatalf("game %d ply %d (%s): incremental mg=%d eg=%d ph=%d h=%08x ps=%d, scratch mg=%d eg=%d ph=%d h=%08x ps=%d",
					game, ply, cur.FEN(),
					cur.MG, cur.EG, cur.Phase, cur.Hash, cur.PStruct,
					check.Pos.MG, check.Pos.EG, check.Pos.Phase, check.Pos.Hash, check.Pos.PStruct)
			}
			eng.Pos.Ply = 0 // commit the move as the new root
		}
	}
}

// TestMateSearch: the asm mate suite's shapes — exact mate scores.
func TestMateSearch(t *testing.T) {
	cases := []struct {
		fen   string
		depth int
		move  string
		score int
	}{
		// KR mate in 2 (from the asm id test): c6b6 mating at ply 3.
		{"k7/8/2K5/8/8/8/8/6R1 w - - 0 1", 6, "c6b6", Mate - 3},
		// Back-rank mate in 1.
		{"6k1/5ppp/8/8/8/8/8/4R2K w - - 0 1", 4, "e1e8", Mate - 1},
	}
	for _, tc := range cases {
		pos, err := ParseFEN(tc.fen)
		if err != nil {
			t.Fatal(err)
		}
		eng := NewEngine()
		eng.SetPosition(pos)
		best, score := eng.SearchFixed(tc.depth)
		if best.UCI() != tc.move || score != tc.score {
			t.Errorf("%s: got %s/%d, want %s/%d", tc.fen, best.UCI(), score, tc.move, tc.score)
		}
	}
}

// TestSelfPlaySmoke: one quick game must terminate with a legal result.
func TestSelfPlaySmoke(t *testing.T) {
	rnd := rand.New(rand.NewPCG(3, 5))
	cfg := PlayerCfg{Features: 0x0F, Weights: DefaultWeights, Depth: 4}
	rec, err := PlayGame(cfg, cfg, []string{"e2e4", "e7e5"}, rnd, true)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Reason == "" || rec.Plies == 0 {
		t.Fatalf("bad game record: %+v", rec)
	}
	t.Logf("game: result=%.1f plies=%d reason=%s samples=%d",
		rec.Result, rec.Plies, rec.Reason, len(rec.Samples))
}

// TestTreeSizeNodes logs fixed-depth node counts for the same position
// and masks as the asm TestTreeSizeDeep, for cross-eyeballing tree-
// shape responses (not exact parity — Zobrist keys differ).
func TestTreeSizeNodes(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	fen := "r1b1k2r/ppp2ppp/2nqpn2/3p4/3P4/P1P1BN2/2P1PPPP/2RQKB1R w Kkq - 2 8"
	for _, mask := range []byte{0x00, 0x01, 0x07, 0x0F} {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		eng := NewEngine()
		eng.Features = mask
		eng.SetPosition(pos)
		best, score := eng.SearchFixed(6)
		t.Logf("features %#02x: %9d nodes, fixed depth 6, best %s score %d",
			mask, eng.Nodes, best.UCI(), score)
	}
}
