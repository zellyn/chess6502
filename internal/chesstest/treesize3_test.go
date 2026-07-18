package chesstest

import "testing"

// TestTreeSizeDeep: masks 0x00 vs 0x07 at fixed depth 6 - null move
// needs remaining >= 4 to fire, so it only shows at deeper searches.
func TestTreeSizeDeep(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	fen := "r1b1k2r/ppp2ppp/2nqpn2/3p4/3P4/P1P1BN2/2P1PPPP/2RQKB1R w Kkq - 2 8"
	for _, mask := range []byte{0x00, 0x01, 0x07, 0x0F, 0x1F} {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		m, err := NewMachine(loadEngine(t), defs, pos, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		SetFeatures(m, defs, mask)
		SetBudget(m, defs, 0, 6) // fixed depth 6
		exited, code, err := m.Run(300_000_000_000)
		if err != nil || !exited || code != 0 {
			t.Fatalf("mask %#02x: exited=%v code=%d err=%v", mask, exited, code, err)
		}
		t.Logf("features %#02x: %5dM cycles, fixed depth 6", mask, m.Cycles/1_000_000)
	}
}
