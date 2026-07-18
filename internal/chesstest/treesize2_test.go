package chesstest

import "testing"

// TestTreeSizeQuick: cycles for a FIXED depth-5 search (single
// iteration, no soft-stop) under each feature mask.
func TestTreeSizeQuick(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	fen := "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1"
	for _, mask := range []byte{0x00, 0x01, 0x02, 0x04, 0x07} {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		m, err := NewMachine(loadEngine(t), defs, pos, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		SetFeatures(m, defs, mask)
		SetBudget(m, defs, 0, 5) // fixed depth 5
		exited, code, err := m.Run(200_000_000_000)
		if err != nil || !exited || code != 0 {
			t.Fatalf("mask %#02x: exited=%v code=%d err=%v", mask, exited, code, err)
		}
		t.Logf("features %#02x: %5dM cycles, fixed depth 5", mask, m.Cycles/1_000_000)
	}
}
