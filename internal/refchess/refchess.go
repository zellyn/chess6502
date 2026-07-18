// Package refchess is an independent reference implementation of FIDE
// chess rules. It exists to referee moves produced by other engines
// (notably the 6502 assembly engine elsewhere in this repo) and to
// adjudicate test matches, so it deliberately imports nothing from the
// rest of the repo: stdlib only.
//
// Squares are numbered 0-63, a1=0, b1=1, ..., h1=7, a2=8, ..., h8=63
// (i.e. sq = rank*8 + file, both 0-based). The board is a plain 8x8
// mailbox array; sliding-piece and knight/king generation do explicit
// file/rank bounds checks rather than using 0x88 sentinels.
package refchess

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
)

// Piece types. 0 is reserved to mean "empty square".
const (
	pawn = 1 + iota
	knight
	bishop
	rook
	queen
	king
)

// Colors, also used as the SideToMove()/side-to-move encoding.
const (
	white = 0
	black = 1
)

// pieceColor and pieceType decode a board byte: bit 3 is color, bits 0-2
// are the piece type. 0 means empty and pieceType/pieceColor should not
// be called on it.
func pieceColor(pc byte) byte { return (pc >> 3) & 1 }
func pieceType(pc byte) byte  { return pc & 7 }
func makePiece(color, typ byte) byte {
	return color<<3 | typ
}

// Move is a from-to move, 0-63 squares, with an optional promotion piece
// ('n', 'b', 'r', 'q'; 0 for none). It does not encode any other flags
// (capture, en passant, castling) — those are derived from the position
// when the move is applied.
type Move struct {
	From, To byte
	Promo    byte
}

// String renders m in UCI notation, e.g. "e2e4" or "e7e8q".
func (m Move) String() string {
	b := []byte{
		'a' + m.From%8, '1' + m.From/8,
		'a' + m.To%8, '1' + m.To/8,
	}
	if m.Promo != 0 {
		b = append(b, m.Promo)
	}
	return string(b)
}

// ParseMove parses UCI move notation ("e2e4", "e7e8q").
func ParseMove(s string) (Move, error) {
	if len(s) != 4 && len(s) != 5 {
		return Move{}, fmt.Errorf("refchess: bad move %q: want 4 or 5 characters", s)
	}
	from, err := parseSquare(s[0:2])
	if err != nil {
		return Move{}, fmt.Errorf("refchess: bad move %q: %w", s, err)
	}
	to, err := parseSquare(s[2:4])
	if err != nil {
		return Move{}, fmt.Errorf("refchess: bad move %q: %w", s, err)
	}
	var promo byte
	if len(s) == 5 {
		switch s[4] {
		case 'n', 'b', 'r', 'q':
			promo = s[4]
		default:
			return Move{}, fmt.Errorf("refchess: bad move %q: bad promotion piece %q", s, s[4])
		}
	}
	return Move{From: from, To: to, Promo: promo}, nil
}

func parseSquare(s string) (byte, error) {
	if len(s) != 2 {
		return 0, fmt.Errorf("bad square %q", s)
	}
	f, r := s[0], s[1]
	if f < 'a' || f > 'h' || r < '1' || r > '8' {
		return 0, fmt.Errorf("bad square %q", s)
	}
	return byte(int(r-'1')*8 + int(f-'a')), nil
}

func sqName(sq byte) string {
	return string([]byte{'a' + sq%8, '1' + sq/8})
}

// Position is a chess position: board plus the usual FEN side state.
// It is a plain value type (no pointers/slices inside), so a shallow
// struct copy is always a full, independent copy — see Copy.
type Position struct {
	board    [64]byte // 0 = empty, else pieceColor<<3 | pieceType
	side     byte     // white or black: side to move
	castle   byte     // bit0 WK, bit1 WQ, bit2 BK, bit3 BQ
	epSquare int      // -1 if none, else the target square of an en passant capture
	halfmove int      // halfmove clock (50-move rule)
	fullmove int      // fullmove number, starts at 1
}

// Copy returns an independent deep copy of p. Position holds no pointers
// or slices, so a value copy already suffices.
func (p *Position) Copy() *Position {
	cp := *p
	return &cp
}

// SideToMove returns white (0) or black (1).
func (p *Position) SideToMove() byte { return p.side }

// HalfmoveClock returns the number of halfmoves since the last capture
// or pawn move (FIDE 50-move rule counter).
func (p *Position) HalfmoveClock() int { return p.halfmove }

// InCheck reports whether the side to move is in check.
func (p *Position) InCheck() bool {
	return p.attacked(p.findKing(p.side), 1-p.side)
}

func (p *Position) findKing(side byte) int {
	want := makePiece(side, king)
	for sq, pc := range p.board {
		if pc == want {
			return sq
		}
	}
	return -1
}

// InsufficientMaterial reports whether the position is an automatic draw
// by insufficient mating material: king vs king; king+minor vs king; or
// king+bishop vs king+bishop with same-colored bishops. This is the
// narrow FIDE-recognized set, not a general "can anybody ever win" test.
func (p *Position) InsufficientMaterial() bool {
	var whiteNonKing, blackNonKing []byte
	whiteBishopSq, blackBishopSq := -1, -1
	for sq, pc := range p.board {
		if pc == 0 {
			continue
		}
		typ := pieceType(pc)
		if typ == king {
			continue
		}
		if pieceColor(pc) == white {
			whiteNonKing = append(whiteNonKing, typ)
			if typ == bishop {
				whiteBishopSq = sq
			}
		} else {
			blackNonKing = append(blackNonKing, typ)
			if typ == bishop {
				blackBishopSq = sq
			}
		}
	}
	switch {
	case len(whiteNonKing) == 0 && len(blackNonKing) == 0:
		return true // KK
	case len(whiteNonKing) == 0 && len(blackNonKing) == 1 && (blackNonKing[0] == knight || blackNonKing[0] == bishop):
		return true // KNK or KBK
	case len(blackNonKing) == 0 && len(whiteNonKing) == 1 && (whiteNonKing[0] == knight || whiteNonKing[0] == bishop):
		return true // KNK or KBK
	case len(whiteNonKing) == 1 && len(blackNonKing) == 1 && whiteNonKing[0] == bishop && blackNonKing[0] == bishop:
		return squareColor(whiteBishopSq) == squareColor(blackBishopSq) // KB vs KB
	default:
		return false
	}
}

func squareColor(sq int) int { return (sq/8 + sq%8) % 2 }

// ParseFEN parses a Forsyth-Edwards Position string. The halfmove clock
// and fullmove number fields are optional and default to 0 and 1.
func ParseFEN(fen string) (*Position, error) {
	fields := strings.Fields(fen)
	if len(fields) < 4 {
		return nil, fmt.Errorf("refchess: FEN needs at least 4 fields: %q", fen)
	}
	p := &Position{epSquare: -1, fullmove: 1}

	rank, file := 7, 0
	for _, c := range fields[0] {
		switch {
		case c == '/':
			if file != 8 {
				return nil, fmt.Errorf("refchess: bad FEN rank length: %q", fen)
			}
			rank--
			file = 0
		case c >= '1' && c <= '8':
			file += int(c - '0')
		default:
			typ, color, ok := charToPiece(c)
			if !ok {
				return nil, fmt.Errorf("refchess: bad FEN piece %q in %q", c, fen)
			}
			if rank < 0 || file > 7 {
				return nil, fmt.Errorf("refchess: bad FEN board: %q", fen)
			}
			p.board[rank*8+file] = makePiece(color, typ)
			file++
		}
	}
	if rank != 0 || file != 8 {
		return nil, fmt.Errorf("refchess: bad FEN board dimensions: %q", fen)
	}

	switch fields[1] {
	case "w":
		p.side = white
	case "b":
		p.side = black
	default:
		return nil, fmt.Errorf("refchess: bad FEN side to move %q", fields[1])
	}

	if fields[2] != "-" {
		for _, c := range fields[2] {
			switch c {
			case 'K':
				p.castle |= 1
			case 'Q':
				p.castle |= 2
			case 'k':
				p.castle |= 4
			case 'q':
				p.castle |= 8
			default:
				return nil, fmt.Errorf("refchess: bad FEN castling rights %q", fields[2])
			}
		}
	}

	if fields[3] != "-" {
		sq, err := parseSquare(fields[3])
		if err != nil {
			return nil, fmt.Errorf("refchess: bad FEN en passant square %q", fields[3])
		}
		p.epSquare = int(sq)
	}

	if len(fields) > 4 {
		n, err := strconv.Atoi(fields[4])
		if err != nil {
			return nil, fmt.Errorf("refchess: bad FEN halfmove clock %q", fields[4])
		}
		p.halfmove = n
	}
	if len(fields) > 5 {
		n, err := strconv.Atoi(fields[5])
		if err != nil {
			return nil, fmt.Errorf("refchess: bad FEN fullmove number %q", fields[5])
		}
		p.fullmove = n
	}
	return p, nil
}

func charToPiece(c rune) (typ, color byte, ok bool) {
	lower := c
	color = white
	if c >= 'a' && c <= 'z' {
		color = black
	} else {
		lower = c + ('a' - 'A')
	}
	switch lower {
	case 'p':
		typ = pawn
	case 'n':
		typ = knight
	case 'b':
		typ = bishop
	case 'r':
		typ = rook
	case 'q':
		typ = queen
	case 'k':
		typ = king
	default:
		return 0, 0, false
	}
	return typ, color, true
}

// FEN renders p back to Forsyth-Edwards notation.
func (p *Position) FEN() string {
	var sb strings.Builder
	for rank := 7; rank >= 0; rank-- {
		empty := 0
		for file := 0; file < 8; file++ {
			pc := p.board[rank*8+file]
			if pc == 0 {
				empty++
				continue
			}
			if empty > 0 {
				sb.WriteByte(byte('0' + empty))
				empty = 0
			}
			sb.WriteByte(pieceChar(pc))
		}
		if empty > 0 {
			sb.WriteByte(byte('0' + empty))
		}
		if rank > 0 {
			sb.WriteByte('/')
		}
	}

	if p.side == white {
		sb.WriteString(" w ")
	} else {
		sb.WriteString(" b ")
	}

	if p.castle == 0 {
		sb.WriteByte('-')
	} else {
		if p.castle&1 != 0 {
			sb.WriteByte('K')
		}
		if p.castle&2 != 0 {
			sb.WriteByte('Q')
		}
		if p.castle&4 != 0 {
			sb.WriteByte('k')
		}
		if p.castle&8 != 0 {
			sb.WriteByte('q')
		}
	}

	sb.WriteByte(' ')
	if p.epSquare < 0 {
		sb.WriteByte('-')
	} else {
		sb.WriteString(sqName(byte(p.epSquare)))
	}

	fmt.Fprintf(&sb, " %d %d", p.halfmove, p.fullmove)
	return sb.String()
}

func pieceChar(pc byte) byte {
	var c byte
	switch pieceType(pc) {
	case pawn:
		c = 'p'
	case knight:
		c = 'n'
	case bishop:
		c = 'b'
	case rook:
		c = 'r'
	case queen:
		c = 'q'
	case king:
		c = 'k'
	}
	if pieceColor(pc) == white {
		c -= 'a' - 'A'
	}
	return c
}

// Zobrist hashing. The key is a pure function of board + side to move +
// castling rights + en passant square, so two positions reached by
// different move orders (a transposition) always hash equal, and any
// difference in castling rights or en passant square always hashes
// different. Table is filled deterministically at init so ZobristKey is
// stable within a process (that's all repetition tracking needs).
var (
	zobristPiece  [2][7][64]uint64 // [color][type][square]; type 0 unused
	zobristSide   uint64
	zobristCastle [4]uint64
	zobristEPFile [8]uint64
)

func init() {
	r := rand.New(rand.NewSource(0xC0FFEE))
	for c := 0; c < 2; c++ {
		for t := 1; t <= 6; t++ {
			for sq := 0; sq < 64; sq++ {
				zobristPiece[c][t][sq] = r.Uint64()
			}
		}
	}
	zobristSide = r.Uint64()
	for i := range zobristCastle {
		zobristCastle[i] = r.Uint64()
	}
	for i := range zobristEPFile {
		zobristEPFile[i] = r.Uint64()
	}
}

// ZobristKey returns a hash of the position suitable for repetition
// tracking: it depends only on piece placement, side to move, castling
// rights and en passant square (not on the halfmove/fullmove counters).
func (p *Position) ZobristKey() uint64 {
	var key uint64
	for sq, pc := range p.board {
		if pc == 0 {
			continue
		}
		key ^= zobristPiece[pieceColor(pc)][pieceType(pc)][sq]
	}
	if p.side == black {
		key ^= zobristSide
	}
	for i := 0; i < 4; i++ {
		if p.castle&(1<<i) != 0 {
			key ^= zobristCastle[i]
		}
	}
	if p.epSquare >= 0 {
		key ^= zobristEPFile[p.epSquare%8]
	}
	return key
}
