package refchess

import "testing"

// mustMake plays s (UCI) on p or fails the test.
func mustMake(t *testing.T, p *Position, s string) {
	t.Helper()
	m, err := ParseMove(s)
	if err != nil {
		t.Fatalf("ParseMove(%q): %v", s, err)
	}
	if err := p.Make(m); err != nil {
		t.Fatalf("Make(%q) on %s: %v", s, p.FEN(), err)
	}
}

func TestZobristTransposition(t *testing.T) {
	start, err := ParseFEN(StartFEN)
	if err != nil {
		t.Fatal(err)
	}

	// Knights out and back: same position as the start, reached via a
	// four-move round trip. Knight moves never touch castling rights or
	// leave an en passant square, so the key should exactly match.
	p1, _ := ParseFEN(StartFEN)
	for _, mv := range []string{"g1f3", "g8f6", "f3g1", "f6g8"} {
		mustMake(t, p1, mv)
	}
	if p1.ZobristKey() != start.ZobristKey() {
		t.Errorf("knights-out-and-back should transpose to start: keys differ (%x vs %x)",
			p1.ZobristKey(), start.ZobristKey())
	}

	// Same resulting position via two different move orders.
	p2, _ := ParseFEN(StartFEN)
	p3, _ := ParseFEN(StartFEN)
	for _, mv := range []string{"b1c3", "b8c6", "g1f3", "g8f6"} {
		mustMake(t, p2, mv)
	}
	for _, mv := range []string{"g1f3", "g8f6", "b1c3", "b8c6"} {
		mustMake(t, p3, mv)
	}
	if p2.FEN() != p3.FEN() {
		t.Fatalf("sanity check failed: the two move orders didn't reach the same position: %q vs %q",
			p2.FEN(), p3.FEN())
	}
	if p2.ZobristKey() != p3.ZobristKey() {
		t.Errorf("different move orders reaching the same position should hash equal")
	}
}

func TestZobristDistinguishesState(t *testing.T) {
	base, err := ParseFEN("r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	noCastle, err := ParseFEN("r3k2r/8/8/8/8/8/8/R3K2R w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	if base.ZobristKey() == noCastle.ZobristKey() {
		t.Error("positions differing only in castling rights hashed equal")
	}

	epSet, err := ParseFEN("4k3/8/8/8/3pP3/8/8/4K3 b - e3 0 1")
	if err != nil {
		t.Fatal(err)
	}
	epUnset, err := ParseFEN("4k3/8/8/8/3pP3/8/8/4K3 b - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	if epSet.ZobristKey() == epUnset.ZobristKey() {
		t.Error("positions differing only in en passant rights hashed equal")
	}
}

// TestZobristIgnoresMoveCounters checks that the halfmove clock and
// fullmove number, which don't affect legal-move generation or
// repetition semantics, don't leak into the hash either.
func TestZobristIgnoresMoveCounters(t *testing.T) {
	a, err := ParseFEN("4k3/8/8/8/8/8/8/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseFEN("4k3/8/8/8/8/8/8/4K3 w - - 17 42")
	if err != nil {
		t.Fatal(err)
	}
	if a.ZobristKey() != b.ZobristKey() {
		t.Error("halfmove/fullmove counters should not affect the Zobrist key")
	}
}
