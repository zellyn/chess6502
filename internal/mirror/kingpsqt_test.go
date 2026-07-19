package mirror

import (
	"math"
	"testing"
)

// TestKBDeltaConsistency checks that the eval-side kbDelta and the
// tuner-side linear decomposition agree for a random parameter vector,
// so tuned tables transfer to the match. (Small differences are allowed
// from the integer taper truncation vs the tuner's linear frac.)
func TestKBDeltaConsistency(t *testing.T) {
	fens := []string{
		"r1b1k2r/ppp2ppp/2nqpn2/3p4/3P4/P1P1BN2/2P1PPPP/2RQKB1R w Kkq - 2 8",
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"r3k2r/pppq1ppp/2np1n2/2b1p1B1/2B1P3/2NP1N2/PPPQ1PPP/R3K2R w KQkq - 0 1",
		"8/2k5/3p4/p2P1p2/P2P1P2/8/2K5/8 w - - 0 1",
		"r2q1rk1/pp2bppp/2n1pn2/2pp4/3P4/2P1PN2/PP1NBPPP/R1BQ1RK1 b - - 0 1",
	}
	// A pseudo-random but deterministic parameter vector.
	params := make([]float64, kbNParams)
	seed := uint64(0x1234)
	for i := range params {
		seed = seed*6364136223846793005 + 1
		params[i] = float64(int(seed>>40)%41) - 20 // -20..20
	}
	kb := kbTablesFromParams(params)

	for _, fen := range fens {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		// eval side: difference of full eval with/without KB.
		base := NewEngine()
		base.SetPosition(pos)
		e0 := base.eval()
		kbe := NewEngine()
		kbe.KB = kb
		kbe.SetPosition(pos)
		e1 := kbe.eval()
		evalDelta := e1 - e0

		// tuner side: white-POV linear prediction minus base. eval()
		// reports from the side to move, so negate for black to move.
		ex := buildKBExample(NewEngine(), pos, DefaultWeights, 0.5)
		lin := 0.0
		for _, f := range ex.feats {
			lin += f.c * params[f.idx]
		}
		if pos.Side != 0 {
			lin = -lin
		}

		if math.Abs(float64(evalDelta)-lin) > 2.0 {
			t.Errorf("%s: eval delta %d vs linear %.2f (mismatch)", fen, evalDelta, lin)
		}
	}
}

// TestKBSymmetry: a color-symmetric position under color-symmetric
// tables has zero white-POV bucket delta.
func TestKBSymmetry(t *testing.T) {
	params := make([]float64, kbNParams)
	for i := range params {
		params[i] = float64(i%7) - 3
	}
	kb := kbTablesFromParams(params)
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	e := NewEngine()
	e.KB = kb
	e.SetPosition(pos)
	if d := e.kbDelta(); d != 0 {
		t.Errorf("symmetric start position: kbDelta = %d, want 0", d)
	}
}
