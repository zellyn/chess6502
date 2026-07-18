// Package mirror is a Go re-implementation of the asm engine with the
// same algorithm shape: 0x88 board + piece lists, PeSTO tapered eval
// with the asm's /32 taper arithmetic, fail-hard negamax with the same
// five-pass move loop, killers, futility/RFP, null move gates, delta-
// pruned quiescence, and a 4096-entry always-replace TT with the same
// bound and mate-adjustment semantics. It exists to run node-denominated
// experiments and eval-weight tuning at compiled speed; winners port
// back to asm. Cycle costs do NOT transfer — only tree shapes and eval
// quality do. The asm (asm/search.s and friends) is the spec, including
// its quirks; divergences are bugs here, not there.
package mirror

import "math/rand/v2"

// Piece encoding, matching asm/defs.inc: index<<4 | color<<3 | type.
const (
	Pawn   = 1
	Knight = 2
	Bishop = 3
	Rook   = 4
	Queen  = 5
	King   = 6

	TypeMask  = 0x07
	ColorMask = 0x08
	IndexMask = 0xF0

	NoSq   = 0xFF
	MaxPly = 32

	// Move flags.
	FlPromo  = 0x07
	FlEP     = 0x08
	FlDouble = 0x10
	FlCastle = 0x20

	// Castling rights bits.
	CrWK = 0x01
	CrWQ = 0x02
	CrBK = 0x04
	CrBQ = 0x08

	// Attack table bits.
	atkKnight = 0x01
	atkKing   = 0x02
	atkDiag   = 0x04
	atkOrtho  = 0x08
	atkWPawn  = 0x10
	atkBPawn  = 0x20

	Inf   = 32000
	Mate  = 30000
	Tempo = 10

	// Mate-zone boundaries as full 16-bit values. The asm tests only the
	// score's high byte: hi >= $74 is a winning mate, $80 <= hi < $8C a
	// losing one.
	mateZoneLo  = 0x7400  // score >= this: winning-mate zone
	nmateZoneHi = -0x7401 // score <= this (hi in $80..$8B): losing-mate zone

	// Feature bits (FEATURES).
	FtNull    = 0x01
	FtKiller  = 0x02
	FtFutil   = 0x04
	FtPstruct = 0x08
)

// TT bound codes.
const (
	ttUpper = 1
	ttLower = 2
	ttExact = 3
)

var (
	knightOffs = []int{-33, -31, -18, -14, 14, 18, 31, 33}
	kingOffs   = []int{-17, -16, -15, -1, 1, 15, 16, 17}
	diagOffs   = []int{-17, -15, 15, 17}
	orthoOffs  = []int{-16, -1, 1, 16}
)

// Difference-indexed tables, (to - from + 0x77) & 0xFF, as in
// cmd/gentables.
var (
	attackTab  [256]byte
	deltaTab   [256]int8
	castleMask [128]byte
	// typeAtkTab[type] = attack bits meaning "this type attacks across
	// this difference"; pawns special-cased.
	typeAtkTab = [7]byte{0, 0, atkKnight, atkDiag, atkOrtho, atkDiag | atkOrtho, atkKing}
)

func onBoard(sq int) bool { return sq >= 0 && sq < 128 && sq&0x88 == 0 }

// PSQT with base values baked in, white POV, indexed by 0x88 square.
// Black uses sq^0x70 and subtracts.
var (
	psqtMG [7][128]int
	psqtEG [7][128]int
	// phaseVal by piece type; phaseW = /32 taper weight by phase 0..24.
	phaseVal = [8]int{0, 0, 1, 1, 2, 4, 0, 0}
	phaseW   [25]int
)

// Zobrist keys: our own values (hash equality with the asm is not
// required), but the same scheme: 32 bits, per (kind, square) plus
// side-to-move, castle-nibble, and ep-file keys.
var (
	zobPiece [12][128]uint32
	zobStm   uint32
	zobCast  [16]uint32
	zobEP    [8]uint32
)

// vicVal: victim values for delta pruning, by piece type. A pseudo-legal
// king "capture" must always be searched, hence the huge value.
var vicVal = [8]int{0, 100, 320, 330, 500, 975, 20000, 0}

func init() {
	for from := range 128 {
		if !onBoard(from) {
			continue
		}
		mark := func(to int, bit byte, step int) {
			if !onBoard(to) {
				return
			}
			idx := (to - from + 0x77) & 0xFF
			attackTab[idx] |= bit
			if step != 0 {
				deltaTab[idx] = int8(step)
			}
		}
		for _, o := range knightOffs {
			mark(from+o, atkKnight, 0)
		}
		for _, o := range kingOffs {
			mark(from+o, atkKing, 0)
		}
		for _, o := range diagOffs {
			for to := from + o; onBoard(to); to += o {
				mark(to, atkDiag, o)
			}
		}
		for _, o := range orthoOffs {
			for to := from + o; onBoard(to); to += o {
				mark(to, atkOrtho, o)
			}
		}
		mark(from+0x0F, atkWPawn, 0)
		mark(from+0x11, atkWPawn, 0)
		mark(from-0x0F, atkBPawn, 0)
		mark(from-0x11, atkBPawn, 0)
	}

	for i := range castleMask {
		castleMask[i] = 0x0F
	}
	castleMask[0x04] = 0x0C
	castleMask[0x00] = 0x0D
	castleMask[0x07] = 0x0E
	castleMask[0x74] = 0x03
	castleMask[0x70] = 0x07
	castleMask[0x77] = 0x0B

	for t := range 6 {
		for sq := range 128 {
			if sq&0x88 != 0 {
				continue
			}
			rank := sq >> 4
			file := sq & 7
			idx := (7-rank)*8 + file // PeSTO tables are a8-first
			psqtMG[t+1][sq] = pestoPieceMG[t] + pestoMG[t][idx]
			psqtEG[t+1][sq] = pestoPieceEG[t] + pestoEG[t][idx]
		}
	}
	for p := 0; p <= 24; p++ {
		phaseW[p] = (p*32 + 12) / 24
	}

	rnd := rand.New(rand.NewPCG(0x6502c4e5, 0x0a11babe))
	for kind := range 12 {
		for sq := range 128 {
			if sq&0x88 != 0 {
				continue
			}
			zobPiece[kind][sq] = rnd.Uint32()
		}
	}
	zobStm = rnd.Uint32()
	for i := range zobCast {
		zobCast[i] = rnd.Uint32()
	}
	for i := range zobEP {
		zobEP[i] = rnd.Uint32()
	}
}

// kindOf maps a piece byte's low nibble to a Zobrist plane kind, as
// KINDTAB does: color*6 + type - 1.
func kindOf(piece byte) int {
	typ := int(piece & TypeMask)
	if piece&ColorMask != 0 {
		return 6 + typ - 1
	}
	return typ - 1
}
