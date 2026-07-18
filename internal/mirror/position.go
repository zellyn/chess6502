package mirror

import (
	"fmt"
	"strconv"
	"strings"
)

// Move is (from, to, flags) exactly as the asm move stack stores it.
type Move struct {
	From, To, Flags byte
}

// NoMove is the "no move" sentinel (asm BESTFROM = NOSQ).
var NoMove = Move{From: NoSq, To: NoSq}

func sqName(sq byte) string {
	return string([]byte{'a' + sq&0x0F, '1' + sq>>4})
}

// UCI renders the move in UCI form ("e2e4", "e7e8q").
func (m Move) UCI() string {
	if m.From == NoSq {
		return "none"
	}
	s := sqName(m.From) + sqName(m.To)
	if p := m.Flags & FlPromo; p != 0 {
		s += string("  nbrq  "[p])
	}
	return s
}

// Position is the poked machine state: board, piece lists, and the
// incremental accumulators the asm keeps in zero page. Ply indexes the
// per-ply arrays in Engine.
type Position struct {
	Board    [128]byte
	PieceSq  [32]byte // slots 0-15 white (0 = king), 16-31 black
	Side     byte     // 0 = white, 8 = black
	EpSq     byte     // 0x88 square or NoSq
	Castle   byte     // CR_* bits
	Halfmove byte
	Ply      int

	// Incremental state (evalinit recomputes from the board).
	MG, EG  int
	Phase   int
	Hash    uint32
	PStruct int
	PDirty  bool
}

var pieceTypes = map[byte]byte{
	'p': Pawn, 'n': Knight, 'b': Bishop, 'r': Rook, 'q': Queen, 'k': King,
}

// ParseFEN mirrors chesstest.ParseFEN exactly, including its slot
// assignment: kings take slot 0; other pieces take slots 1..15 in FEN
// scan order (rank 8 to rank 1, file a to h). Slot order determines
// move-generation order, so tree shapes depend on it.
func ParseFEN(fen string) (*Position, error) {
	fields := strings.Fields(fen)
	if len(fields) < 4 {
		return nil, fmt.Errorf("FEN needs at least 4 fields: %q", fen)
	}
	pos := &Position{EpSq: NoSq}
	for i := range pos.PieceSq {
		pos.PieceSq[i] = NoSq
	}

	next := [2]byte{1, 1} // next free index per color (0 reserved for king)
	rank := 7
	file := 0
	for _, c := range fields[0] {
		switch {
		case c == '/':
			rank--
			file = 0
		case c >= '1' && c <= '8':
			file += int(c - '0')
		default:
			typ, ok := pieceTypes[byte(c|0x20)]
			if !ok {
				return nil, fmt.Errorf("bad FEN piece %q", c)
			}
			var color byte
			if c >= 'a' {
				color = 1
			}
			var index byte
			if typ == King {
				index = 0
			} else {
				index = next[color]
				next[color]++
				if index > 15 {
					return nil, fmt.Errorf("more than 16 pieces for one side")
				}
			}
			sq := byte(rank*16 + file)
			pos.Board[sq] = index<<4 | color<<3 | typ
			pos.PieceSq[index+16*color] = sq
			file++
		}
	}

	switch fields[1] {
	case "w":
		pos.Side = 0
	case "b":
		pos.Side = 8
	default:
		return nil, fmt.Errorf("bad side %q", fields[1])
	}

	if fields[2] != "-" {
		for _, c := range fields[2] {
			switch c {
			case 'K':
				pos.Castle |= CrWK
			case 'Q':
				pos.Castle |= CrWQ
			case 'k':
				pos.Castle |= CrBK
			case 'q':
				pos.Castle |= CrBQ
			}
		}
	}

	if fields[3] != "-" {
		if len(fields[3]) != 2 {
			return nil, fmt.Errorf("bad ep square %q", fields[3])
		}
		pos.EpSq = (fields[3][1]-'1')<<4 | (fields[3][0] - 'a')
	}
	if len(fields) > 4 {
		if hm, err := strconv.Atoi(fields[4]); err == nil && hm >= 0 && hm < 256 {
			pos.Halfmove = byte(hm)
		}
	}
	return pos, nil
}

// FEN renders the position (for interop with refchess in tests).
func (p *Position) FEN() string {
	var b strings.Builder
	for rank := 7; rank >= 0; rank-- {
		empty := 0
		for file := 0; file < 8; file++ {
			pc := p.Board[rank*16+file]
			if pc == 0 {
				empty++
				continue
			}
			if empty > 0 {
				fmt.Fprintf(&b, "%d", empty)
				empty = 0
			}
			c := " pnbrqk"[pc&TypeMask]
			if pc&ColorMask == 0 {
				c -= 32 // uppercase
			}
			b.WriteByte(byte(c))
		}
		if empty > 0 {
			fmt.Fprintf(&b, "%d", empty)
		}
		if rank > 0 {
			b.WriteByte('/')
		}
	}
	if p.Side == 0 {
		b.WriteString(" w ")
	} else {
		b.WriteString(" b ")
	}
	if p.Castle == 0 {
		b.WriteByte('-')
	} else {
		for i, c := range []byte{'K', 'Q', 'k', 'q'} {
			if p.Castle&(1<<i) != 0 {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte(' ')
	if p.EpSq == NoSq {
		b.WriteByte('-')
	} else {
		b.WriteString(sqName(p.EpSq))
	}
	fmt.Fprintf(&b, " %d 1", p.Halfmove)
	return b.String()
}

// slotOf returns the PieceSq slot for a piece byte:
// index | (color ? 16 : 0).
func slotOf(piece byte) int {
	return int(piece>>4) | int(piece&ColorMask)<<1
}
