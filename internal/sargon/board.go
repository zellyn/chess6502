package sargon

import "fmt"

// Sargon III keeps its live position as a 32-entry piece-square list in zero
// page. Each byte is the piece's board square in 0x88-style coordinates
// (square = rank*16 + file, with rank 0 = rank 1 and file 0 = file a, so valid
// squares are $00-$77 and bit 7 is always clear). A captured piece's square
// byte is set to CapturedSquare ($80).
//
// The list is split into 16 white entries at $60-$6F and 16 black entries at
// $70-$7F. The index within each half selects a fixed piece slot (the type is
// implied by the slot; a promoted pawn keeps its slot but changes type — see
// PieceIndexType's caveat):
//
//	index 0-7 : pawns on files a-h (initially)
//	index 8   : knight   (b-file home)
//	index 9   : knight   (g-file home)
//	index 10  : bishop   (c-file home)
//	index 11  : bishop   (f-file home)
//	index 12  : rook     (a-file home)
//	index 13  : rook     (h-file home)
//	index 14  : queen    (d-file home)
//	index 15  : king     (e-file home)
const (
	WhiteListAddr  = 0x0060 // $60-$6F: 16 white piece squares
	BlackListAddr  = 0x0070 // $70-$7F: 16 black piece squares
	PieceListAddr  = WhiteListAddr
	PieceListLen   = 32
	CapturedSquare = 0x80 // sentinel stored in a captured piece's square byte
)

// Square is a Sargon 0x88-style board square: rank*16 + file, rank 0 == rank 1,
// file 0 == file a. Valid squares have both nibbles in 0-7.
type Square byte

// Valid reports whether s names a real board square (both nibbles 0-7).
func (s Square) Valid() bool { return s&0x88 == 0 }

// Captured reports whether s is the captured-piece sentinel.
func (s Square) Captured() bool { return s == CapturedSquare }

// File returns 0-7 (a-h). Rank returns 0-7 (rank 1-8).
func (s Square) File() int { return int(s & 0x0F) }
func (s Square) Rank() int { return int(s >> 4) }

// Algebraic renders the square as e.g. "e4". Off-board/captured squares render
// as "??".
func (s Square) Algebraic() string {
	if !s.Valid() {
		return "??"
	}
	return fmt.Sprintf("%c%c", 'a'+s.File(), '1'+s.Rank())
}

// ParseSquare parses "e4"/"E4" into a Square.
func ParseSquare(a string) (Square, bool) {
	if len(a) != 2 {
		return 0, false
	}
	f := a[0]
	if f >= 'A' && f <= 'H' {
		f += 'a' - 'A'
	}
	if f < 'a' || f > 'h' || a[1] < '1' || a[1] > '8' {
		return 0, false
	}
	return Square(byte(a[1]-'1')<<4 | byte(f-'a')), true
}

// PieceType is a chess piece kind.
type PieceType byte

const (
	Empty PieceType = iota
	Pawn
	Knight
	Bishop
	Rook
	Queen
	King
)

func (p PieceType) Letter() byte {
	switch p {
	case Pawn:
		return 'P'
	case Knight:
		return 'N'
	case Bishop:
		return 'B'
	case Rook:
		return 'R'
	case Queen:
		return 'Q'
	case King:
		return 'K'
	}
	return '.'
}

// PieceIndexType returns the piece type for a piece-list index (0-15), based on
// the fixed slot assignment.
//
// Caveat: this is the *starting* type of the slot. Slots 8-15 never change
// type, but a pawn slot (0-7) that promotes will report Pawn here — detect
// promotions from the move text ("/Q") or a reached back rank rather than
// trusting this after a promotion.
func PieceIndexType(i int) PieceType {
	switch {
	case i >= 0 && i <= 7:
		return Pawn
	case i == 8 || i == 9:
		return Knight
	case i == 10 || i == 11:
		return Bishop
	case i == 12 || i == 13:
		return Rook
	case i == 14:
		return Queen
	case i == 15:
		return King
	}
	return Empty
}

// PieceList is a snapshot of the 32 piece-square bytes ($60-$7F).
// Indices 0-15 are white, 16-31 are black.
type PieceList [PieceListLen]Square

// ReadPieceList reads the live piece-square list from zero page.
func (m *Machine) ReadPieceList() PieceList {
	var pl PieceList
	for i := 0; i < PieceListLen; i++ {
		pl[i] = Square(m.A2.RamRead(uint16(PieceListAddr + i)))
	}
	return pl
}

// IsWhite reports whether piece-list index i is a white piece.
func (pl PieceList) IsWhite(i int) bool { return i < 16 }

// Board renders the piece list as an 8x8 grid of ASCII, rank 8 at the top,
// white pieces uppercase and black lowercase, empty squares '.'. Intended for
// debugging and cross-checking against the ESC board display.
func (pl PieceList) Board() string {
	var g [8][8]byte
	for r := range g {
		for f := range g[r] {
			g[r][f] = '.'
		}
	}
	for i, sq := range pl {
		if !sq.Valid() {
			continue // captured / off-board
		}
		white := i < 16
		typ := PieceIndexType(i % 16)
		ch := typ.Letter()
		if !white {
			ch += 'a' - 'A'
		}
		g[sq.Rank()][sq.File()] = ch
	}
	out := make([]byte, 0, 8*9)
	for r := 7; r >= 0; r-- {
		out = append(out, g[r][:]...)
		out = append(out, '\n')
	}
	return string(out)
}

// Move is a from/to move decoded from a piece-list diff.
type Move struct {
	From, To Square
	// CapturedIndex is the piece-list index of a piece that was captured
	// (its square became CapturedSquare), or -1 if none.
	CapturedIndex int
	// Indices that moved (from/to belong to MovedIndex). For castling two
	// indices move; ExtraFrom/ExtraTo carry the rook.
	MovedIndex         int
	ExtraFrom, ExtraTo Square
	ExtraIndex         int
}

// DiffMove decodes the move that transformed prev into cur by comparing the
// two piece lists, looking only at the given side ("white" = indices 0-15,
// "black" = 16-31). It returns the primary from/to and any captured/rook
// indices. Returns ok=false if no square in that side changed.
//
// It handles: normal moves, captures (a piece on the *other* side becomes
// CapturedSquare), and castling (the mover's king and one rook both move).
func DiffMove(prev, cur PieceList, black bool) (mv Move, ok bool) {
	mv.CapturedIndex = -1
	mv.ExtraIndex = -1
	lo, hi := 0, 16
	if black {
		lo, hi = 16, 32
	}
	var moved []int
	for i := lo; i < hi; i++ {
		if prev[i] != cur[i] {
			moved = append(moved, i)
		}
	}
	// Find a captured piece on the opposing side.
	olo, ohi := 16, 32
	if black {
		olo, ohi = 0, 16
	}
	for i := olo; i < ohi; i++ {
		if prev[i] != cur[i] && cur[i].Captured() {
			mv.CapturedIndex = i
		}
	}
	switch len(moved) {
	case 0:
		return mv, false
	case 1:
		i := moved[0]
		mv.MovedIndex, mv.From, mv.To = i, prev[i], cur[i]
		return mv, true
	default:
		// Castling (king + rook) or an unusual multi-change. Report the
		// king slot as primary if present.
		king := -1
		for _, i := range moved {
			if PieceIndexType(i%16) == King {
				king = i
			}
		}
		if king < 0 {
			king = moved[0]
		}
		mv.MovedIndex, mv.From, mv.To = king, prev[king], cur[king]
		for _, i := range moved {
			if i != king {
				mv.ExtraIndex, mv.ExtraFrom, mv.ExtraTo = i, prev[i], cur[i]
			}
		}
		return mv, true
	}
}
