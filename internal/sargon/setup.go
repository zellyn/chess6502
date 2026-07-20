package sargon

import (
	"fmt"
	"strings"
)

// KNOWN LIMITATION — SetupPosition does NOT currently work: Sargon reconstructs
// its board from the on-screen move list (replaying from a template), so the
// $60-$7F piece list is a derived copy. Poking it is silently reverted on the
// next move (a fresh game replays zero moves -> standard start). Setting an
// arbitrary position needs the move-gen master board / move-list state, which
// is not yet reverse-engineered. Kept for the FEN parsing + slot assignment,
// which are correct and unit-tested; do not rely on it to change Sargon's
// position until the master board is found. See docs/sargon.md.
//
// SetupPosition sets Sargon's board to a FEN/EPD position by overwriting the
// zero-page piece-square list ($60-$7F). It handles the piece placement and
// side-to-move check; castling rights and en passant are NOT poked (Sargon
// infers castling from whether K/R sit on home squares — correct for the
// common case). Only white-to-move positions are supported (the openings pool
// is all "w"); a black-to-move FEN is rejected.
//
// Piece slots are fixed by type (0-7 pawns, 8/9 knights, 10/11 bishops, 12/13
// rooks, 14 queen, 15 king; +16 for black). A position with more pieces of a
// type than slots (e.g. a promoted 3rd knight) is rejected — outside the pool.
//
// Must be called at the move-entry prompt (after BootToPrompt), before any
// move. It does not change level/mode.
func (m *Machine) SetupPosition(fen string) error {
	board, whiteToMove, err := parseFEN(fen)
	if err != nil {
		return err
	}
	if !whiteToMove {
		return fmt.Errorf("SetupPosition: only white-to-move positions supported")
	}

	// Assign the pieces of each color to their fixed slots.
	white, err := assignSlots(board, true)
	if err != nil {
		return fmt.Errorf("white: %w", err)
	}
	black, err := assignSlots(board, false)
	if err != nil {
		return fmt.Errorf("black: %w", err)
	}

	for i := 0; i < 16; i++ {
		m.Poke(uint16(WhiteListAddr+i), byte(white[i]))
		m.Poke(uint16(BlackListAddr+i), byte(black[i]))
	}
	// Let the display/derived state catch up.
	return m.Run(pollChunk)
}

// fenPiece is a parsed piece: its type and 0x88 square.
type fenPiece struct {
	typ PieceType
	sq  Square
}

// parseFEN parses the piece-placement + active-color fields of a FEN/EPD.
func parseFEN(fen string) (pieces []fenPiece, whiteToMove bool, err error) {
	fen = strings.TrimSpace(fen)
	fields := strings.Fields(fen)
	if len(fields) < 2 {
		return nil, false, fmt.Errorf("bad FEN %q", fen)
	}
	ranks := strings.Split(fields[0], "/")
	if len(ranks) != 8 {
		return nil, false, fmt.Errorf("FEN needs 8 ranks, got %d", len(ranks))
	}
	// FEN ranks are listed 8 (top) down to 1.
	for ri, rank := range ranks {
		rankIdx := 7 - ri // 0 = rank 1
		file := 0
		for _, c := range rank {
			if c >= '1' && c <= '8' {
				file += int(c - '0')
				continue
			}
			if file > 7 {
				return nil, false, fmt.Errorf("FEN rank overflow: %q", rank)
			}
			pt, white := fenPieceType(c)
			if pt == Empty {
				return nil, false, fmt.Errorf("bad FEN piece %q", string(c))
			}
			sq := Square(byte(rankIdx)<<4 | byte(file))
			p := fenPiece{typ: pt, sq: sq}
			if !white {
				p.typ |= 0x80 // tag black in the high bit for bucketing
			}
			pieces = append(pieces, p)
			file++
		}
	}
	return pieces, fields[1] == "w", nil
}

func fenPieceType(c rune) (PieceType, bool) {
	white := c >= 'A' && c <= 'Z'
	switch c {
	case 'P', 'p':
		return Pawn, white
	case 'N', 'n':
		return Knight, white
	case 'B', 'b':
		return Bishop, white
	case 'R', 'r':
		return Rook, white
	case 'Q', 'q':
		return Queen, white
	case 'K', 'k':
		return King, white
	}
	return Empty, false
}

// slotsForType maps a piece type to its piece-list slot indices (within a color
// half, 0-15).
func slotsForType(t PieceType) []int {
	switch t {
	case Pawn:
		return []int{0, 1, 2, 3, 4, 5, 6, 7}
	case Knight:
		return []int{8, 9}
	case Bishop:
		return []int{10, 11}
	case Rook:
		return []int{12, 13}
	case Queen:
		return []int{14}
	case King:
		return []int{15}
	}
	return nil
}

// assignSlots produces the 16-byte piece-square list for one color from the FEN
// pieces: each slot holds its piece's square, or CapturedSquare if that slot's
// piece is absent.
func assignSlots(pieces []fenPiece, white bool) ([16]Square, error) {
	var out [16]Square
	for i := range out {
		out[i] = CapturedSquare
	}
	// Track next free slot per type.
	next := map[PieceType]int{}
	for _, p := range pieces {
		isWhite := p.typ&0x80 == 0
		if isWhite != white {
			continue
		}
		t := p.typ &^ 0x80
		slots := slotsForType(t)
		idx := next[t]
		if idx >= len(slots) {
			return out, fmt.Errorf("too many %v (promotion) — unsupported", t)
		}
		out[slots[idx]] = p.sq
		next[t]++
	}
	return out, nil
}
