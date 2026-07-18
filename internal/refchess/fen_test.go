package refchess

import "testing"

func TestFENRoundTrip(t *testing.T) {
	fens := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",
		"r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1",
		"rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 1 8",
		"r4rk1/1pp1qppp/p1np1n2/2b1p1B1/2B1P1b1/P1NP1N2/1PP1QPPP/R4RK1 w - - 0 10",
		"8/8/8/8/8/8/8/K6k w - - 0 1",
		"4k3/8/8/8/8/8/8/4K2R w K - 0 1",
		"r3k3/8/8/8/8/8/8/4K3 b q - 12 34",
	}
	for _, fen := range fens {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatalf("ParseFEN(%q): %v", fen, err)
		}
		got := pos.FEN()
		if got != fen {
			t.Errorf("round trip mismatch:\n got  %q\n want %q", got, fen)
		}
	}
}

func TestParseFENErrors(t *testing.T) {
	bad := []string{
		"",
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR",              // missing side/castle/ep
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR x KQkq - 0 1", // bad side
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNX w KQkq - 0 1", // bad piece
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP w KQkq - 0 1",          // too few ranks
	}
	for _, fen := range bad {
		if _, err := ParseFEN(fen); err == nil {
			t.Errorf("ParseFEN(%q): expected error, got nil", fen)
		}
	}
}

func TestMoveStringAndParse(t *testing.T) {
	cases := []struct {
		s string
		m Move
	}{
		{"e2e4", Move{From: 12, To: 28}},
		{"e7e8q", Move{From: 52, To: 60, Promo: 'q'}},
		{"a1h8", Move{From: 0, To: 63}},
	}
	for _, tc := range cases {
		m, err := ParseMove(tc.s)
		if err != nil {
			t.Fatalf("ParseMove(%q): %v", tc.s, err)
		}
		if m != tc.m {
			t.Errorf("ParseMove(%q) = %+v, want %+v", tc.s, m, tc.m)
		}
		if got := m.String(); got != tc.s {
			t.Errorf("Move(%+v).String() = %q, want %q", m, got, tc.s)
		}
	}

	badMoves := []string{"", "e2e", "e2e44", "i2e4", "e2e9", "e2e4x"}
	for _, s := range badMoves {
		if _, err := ParseMove(s); err == nil {
			t.Errorf("ParseMove(%q): expected error, got nil", s)
		}
	}
}
