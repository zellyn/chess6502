package sargon

import "testing"

func TestSquareAlgebraic(t *testing.T) {
	cases := []struct {
		sq  Square
		alg string
	}{
		{0x00, "a1"}, {0x07, "h1"}, {0x14, "e2"}, {0x34, "e4"},
		{0x64, "e7"}, {0x54, "e6"}, {0x70, "a8"}, {0x77, "h8"},
	}
	for _, c := range cases {
		if got := c.sq.Algebraic(); got != c.alg {
			t.Errorf("Square($%02X).Algebraic() = %q, want %q", byte(c.sq), got, c.alg)
		}
		parsed, ok := ParseSquare(c.alg)
		if !ok || parsed != c.sq {
			t.Errorf("ParseSquare(%q) = $%02X,%v want $%02X", c.alg, byte(parsed), ok, byte(c.sq))
		}
	}
	if CapturedSquare != 0x80 || Square(CapturedSquare).Valid() {
		t.Errorf("captured sentinel should be off-board")
	}
}

// initialList returns the piece list for the starting position.
func initialList() PieceList {
	var pl PieceList
	// white pawns a2-h2, black pawns a7-h7
	for f := 0; f < 8; f++ {
		pl[f] = Square(0x10 + f)    // white pawn on rank 2
		pl[16+f] = Square(0x60 + f) // black pawn on rank 7
	}
	// back-rank slots N,N,B,B,R,R,Q,K on b,g,c,f,a,h,d,e
	homes := []byte{0x01, 0x06, 0x02, 0x05, 0x00, 0x07, 0x03, 0x04} // white rank 1
	for i, h := range homes {
		pl[8+i] = Square(h)
		pl[24+i] = Square(0x70 + (h & 0x0F)) // black rank 8
	}
	return pl
}

func TestDiffMoveNormal(t *testing.T) {
	prev := initialList()
	cur := prev
	cur[20] = 0x54 // black e-pawn e7->e6
	mv, ok := DiffMove(prev, cur, true)
	if !ok || mv.From != 0x64 || mv.To != 0x54 || mv.CapturedIndex != -1 {
		t.Fatalf("normal move decode wrong: %+v ok=%v", mv, ok)
	}
	if mv.From.Algebraic() != "e7" || mv.To.Algebraic() != "e6" {
		t.Errorf("got %s-%s, want e7-e6", mv.From.Algebraic(), mv.To.Algebraic())
	}
}

func TestDiffMoveCapture(t *testing.T) {
	prev := initialList()
	// simulate: black pawn (idx20) captures a white piece on d5; white d-pawn
	// (idx3) becomes captured.
	prev[20] = 0x54 // black e-pawn on e6
	prev[3] = 0x43  // white d-pawn on d5 (about to be captured)
	cur := prev
	cur[20] = 0x43 // black e-pawn takes on d5
	cur[3] = CapturedSquare
	mv, ok := DiffMove(prev, cur, true)
	if !ok || mv.From != 0x54 || mv.To != 0x43 {
		t.Fatalf("capture decode wrong: %+v ok=%v", mv, ok)
	}
	if mv.CapturedIndex != 3 {
		t.Errorf("CapturedIndex = %d, want 3", mv.CapturedIndex)
	}
}

func TestDiffMoveCastle(t *testing.T) {
	prev := initialList()
	// clear f1,g1 so the king/rook can castle kingside (white). King idx15
	// e1->g1, rook idx13 h1->f1.
	cur := prev
	cur[15] = 0x06 // king to g1
	cur[13] = 0x05 // rook to f1
	mv, ok := DiffMove(prev, cur, false)
	if !ok {
		t.Fatal("castle not decoded")
	}
	// King is the primary mover.
	if mv.MovedIndex != 15 || mv.From != 0x04 || mv.To != 0x06 {
		t.Errorf("king move wrong: idx=%d %s-%s", mv.MovedIndex, mv.From.Algebraic(), mv.To.Algebraic())
	}
	if mv.ExtraIndex != 13 || mv.ExtraFrom != 0x07 || mv.ExtraTo != 0x05 {
		t.Errorf("rook move wrong: idx=%d %s-%s", mv.ExtraIndex, mv.ExtraFrom.Algebraic(), mv.ExtraTo.Algebraic())
	}
}

func TestParseFromTo(t *testing.T) {
	for _, c := range []struct {
		in       string
		from, to Square
		ok       bool
	}{
		{"E2-E4", 0x14, 0x34, true},
		{"E4XD5", 0x34, 0x43, true},
		{"E7-E8Q", 0x64, 0x74, true},
		{"garbage", 0, 0, false},
	} {
		from, to, ok := parseFromTo(c.in)
		if ok != c.ok || (ok && (from != c.from || to != c.to)) {
			t.Errorf("parseFromTo(%q) = $%02X,$%02X,%v want $%02X,$%02X,%v",
				c.in, byte(from), byte(to), ok, byte(c.from), byte(c.to), c.ok)
		}
	}
}
