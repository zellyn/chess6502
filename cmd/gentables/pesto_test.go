package main

import "testing"

// Spot checks transcribed directly from the C source at
// https://www.chessprogramming.org/PeSTO%27s_Evaluation_Function
//
// For reference, the three full rows quoted below are (each row is 8
// consecutive squares, in source order):
//
//	mg_pawn_table row 1 (idx 8..15):
//	  98, 134,  61,  95,  68, 126, 34, -11,
//	mg_knight_table row 0 (idx 0..7):
//	  -167, -89, -34, -49,  61, -97, -15, -107,
//	eg_king_table row 7 (idx 56..63):
//	  -53, -34, -21, -11, -28, -14, -24, -43
const pieceP, pieceN, pieceB, pieceR, pieceQ, pieceK = 0, 1, 2, 3, 4, 5

func TestPestoPawnRow1MG(t *testing.T) {
	want := [8]int{98, 134, 61, 95, 68, 126, 34, -11}
	got := [8]int(pestoMG[pieceP][8:16])
	if got != want {
		t.Errorf("mg_pawn_table[8:16] = %v, want %v", got, want)
	}
}

func TestPestoKnightRow0MG(t *testing.T) {
	want := [8]int{-167, -89, -34, -49, 61, -97, -15, -107}
	got := [8]int(pestoMG[pieceN][0:8])
	if got != want {
		t.Errorf("mg_knight_table[0:8] = %v, want %v", got, want)
	}
}

func TestPestoKingRow7EG(t *testing.T) {
	want := [8]int{-53, -34, -21, -11, -28, -14, -24, -43}
	got := [8]int(pestoEG[pieceK][56:64])
	if got != want {
		t.Errorf("eg_king_table[56:64] = %v, want %v", got, want)
	}
}

func TestPestoBishopRow2EG(t *testing.T) {
	// eg_bishop_table row 2 (idx 16..23): 2,  -8,   0,  -1, -2,   6,   0,   4,
	want := [8]int{2, -8, 0, -1, -2, 6, 0, 4}
	got := [8]int(pestoEG[pieceB][16:24])
	if got != want {
		t.Errorf("eg_bishop_table[16:24] = %v, want %v", got, want)
	}
}

func TestPestoQueenRow6MG(t *testing.T) {
	// mg_queen_table row 6 (idx 48..55): -35,  -8,  11,   2,   8,  15,  -3,   1,
	want := [8]int{-35, -8, 11, 2, 8, 15, -3, 1}
	got := [8]int(pestoMG[pieceQ][48:56])
	if got != want {
		t.Errorf("mg_queen_table[48:56] = %v, want %v", got, want)
	}
}

func TestPestoRookRow0MG(t *testing.T) {
	// mg_rook_table row 0 (idx 0..7): 32,  42,  32,  51, 63,  9,  31,  43,
	want := [8]int{32, 42, 32, 51, 63, 9, 31, 43}
	got := [8]int(pestoMG[pieceR][0:8])
	if got != want {
		t.Errorf("mg_rook_table[0:8] = %v, want %v", got, want)
	}
}

// First/last entries of several tables, per source.
func TestPestoFirstLastEntries(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"mg_pawn_table[0]", pestoMG[pieceP][0], 0},
		{"mg_pawn_table[63]", pestoMG[pieceP][63], 0},
		{"eg_pawn_table[8]", pestoEG[pieceP][8], 178},
		{"eg_pawn_table[15]", pestoEG[pieceP][15], 187},
		{"mg_knight_table[0]", pestoMG[pieceN][0], -167},
		{"mg_knight_table[63]", pestoMG[pieceN][63], -23},
		{"eg_knight_table[0]", pestoEG[pieceN][0], -58},
		{"eg_knight_table[63]", pestoEG[pieceN][63], -64},
		{"mg_bishop_table[0]", pestoMG[pieceB][0], -29},
		{"mg_bishop_table[63]", pestoMG[pieceB][63], -21},
		{"mg_rook_table[63]", pestoMG[pieceR][63], -26},
		{"eg_rook_table[0]", pestoEG[pieceR][0], 13},
		{"eg_rook_table[63]", pestoEG[pieceR][63], -20},
		{"mg_queen_table[0]", pestoMG[pieceQ][0], -28},
		{"mg_queen_table[63]", pestoMG[pieceQ][63], -50},
		{"eg_queen_table[0]", pestoEG[pieceQ][0], -9},
		{"eg_queen_table[63]", pestoEG[pieceQ][63], -41},
		{"mg_king_table[0]", pestoMG[pieceK][0], -65},
		{"mg_king_table[63]", pestoMG[pieceK][63], 14},
		{"eg_king_table[0]", pestoEG[pieceK][0], -74},
		{"eg_king_table[63]", pestoEG[pieceK][63], -43},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

func TestPestoPieceValues(t *testing.T) {
	wantMG := [6]int{82, 337, 365, 477, 1025, 0}
	wantEG := [6]int{94, 281, 297, 512, 936, 0}
	if pestoPieceMG != wantMG {
		t.Errorf("pestoPieceMG = %v, want %v", pestoPieceMG, wantMG)
	}
	if pestoPieceEG != wantEG {
		t.Errorf("pestoPieceEG = %v, want %v", pestoPieceEG, wantEG)
	}
}

// Plausibility bounds: all tables except the middlegame king table should
// stay within a modest range; the mg king table is known to swing wider
// (source values range from -65 to +29).
func TestPestoBounds(t *testing.T) {
	for p := 0; p < 6; p++ {
		for sq := 0; sq < 64; sq++ {
			if v := pestoMG[p][sq]; p != pieceK {
				if v < -400 || v > 400 {
					t.Errorf("pestoMG[%d][%d] = %d out of plausible bounds", p, sq, v)
				}
			} else {
				if v < -200 || v > 200 {
					t.Errorf("pestoMG king[%d] = %d out of plausible bounds", sq, v)
				}
			}
			if v := pestoEG[p][sq]; v < -400 || v > 400 {
				t.Errorf("pestoEG[%d][%d] = %d out of plausible bounds", p, sq, v)
			}
		}
	}
}
