package chesstest

import "testing"

// TestTreeSize measures cycles to complete ID-to-depth-5 under each
// feature mask. Pruning features must shrink the tree; this is the
// fast sanity check behind the Elo-noisy self-play gates.
func TestTreeSize(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	fens := []string{
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"r1b1k2r/ppp2ppp/2nqpn2/3p4/3P4/P1P1BN2/2P1PPPP/2RQKB1R w Kkq - 2 8",
		"r4rk1/1pp1qppp/p1np1n2/2b1p1B1/2B1P1b1/P1NP1N2/1PP1QPPP/R4RK1 w - - 0 10",
	}
	for _, mask := range []byte{0x00, 0x01, 0x02, 0x04, 0x07} {
		var total uint64
		for _, fen := range fens {
			pos, err := ParseFEN(fen)
			if err != nil {
				t.Fatal(err)
			}
			m, err := NewMachine(loadEngine(t), defs, pos, 0, nil)
			if err != nil {
				t.Fatal(err)
			}
			SetFeatures(m, defs, mask)
			m.Mem.Main[defs["MAXDEPTH"]] = 5
			m.Mem.Main[defs["BUDGET0"]] = 0xFF // huge: complete all iterations
			m.Mem.Main[defs["BUDGET1"]] = 0xFF
			m.Mem.Main[defs["BUDGET2"]] = 0xFF
			exited, code, err := m.Run(100_000_000_000)
			if err != nil || !exited || (code != 0 && code != 2) {
				t.Fatalf("mask %#02x: exited=%v code=%d err=%v", mask, exited, code, err)
			}
			total += m.Cycles
		}
		t.Logf("features %#02x: %5dM cycles to depth 5 (3 positions)", mask, total/1_000_000)
	}
}
