package sargon

import (
	"fmt"
	"strings"
)

// Arbitrary-position setup via Sargon's CTRL-A "Analysis Mode" board editor.
//
// MECHANISM (reverse-engineered empirically; see docs/sargon.md). While the
// CTRL-A editor is active, Sargon holds the position being edited as a plain
// 0x88-style board array in zero page based at $80: the byte for a square is at
//
//	editorBoardBase + rank*16 + file    (rank 0 = rank 1, file 0 = file a)
//
// Each byte encodes the piece directly (independent of the slot-based $60-$7F
// piece list): empty = $80; a piece = colorBit | typeCode, where colorBit is
// $40 (white) or $50 (black) and typeCode is P=$0 N=$8 B=$A R=$C Q=$E K=$F.
//
// On RETURN (exit), Sargon's editor reconciles that board into its AUTHORITATIVE
// state: it rebuilds the $60-$7F piece-slot list AND the $1120-$113F new-game
// template, and assigns castle status (K/R on home squares => may castle). That
// authoritative state PERSISTS — Sargon plays legal moves from it and does not
// revert to the standard start (verified). So we set a position by driving the
// editor's DATA, not by pantomiming cursor/piece keystrokes: press CTRL-A, poke
// the $80 board, press RETURN. Only two keypresses, each confirmed by a RAM
// state change (never a fixed delay), so nothing can be dropped or misordered.
//
// Side-to-move is set with CTRL-C ("Change Color with the Move", the manual's
// supported way to pick whose turn it is after Analysis Mode). $0002 reflects
// it: $10 = white to move, $60 = black to move (verified behaviorally: with the
// same board, white-to-move makes Sargon play a white piece, black-to-move a
// black piece).
const (
	editorBoardBase = 0x0080 // 0x88-style editor board: base + rank*16 + file
	editorEmpty     = 0x80   // empty-square byte in the editor board
	editorWhiteBit  = 0x40   // colour bits: $40 white, $50 black
	editorBlackBit  = 0x50

	// stmAddr holds Sargon's side-to-move after a position has been set up (it
	// is $FF before any setup). Found by diffing white- vs black-to-move over
	// several boards: it is the only stable, board-invariant byte that encodes
	// the moving side. ($0002 looks like a flag right after CTRL-C but is really
	// zero-page scratch that the editor/redraw clobbers, so it is NOT reliable.)
	stmAddr  = 0x0035
	stmWhite = 0x01
	stmBlack = 0x05
)

// editorTypeCode maps a piece type to its editor-board low nibble.
func editorTypeCode(t PieceType) (byte, bool) {
	switch t {
	case Pawn:
		return 0x0, true
	case Knight:
		return 0x8, true
	case Bishop:
		return 0xA, true
	case Rook:
		return 0xC, true
	case Queen:
		return 0xE, true
	case King:
		return 0xF, true
	}
	return 0, false
}

// editorPieceByte encodes a piece as the editor-board byte for its square.
func editorPieceByte(t PieceType, white bool) (byte, bool) {
	lo, ok := editorTypeCode(t)
	if !ok {
		return 0, false
	}
	hi := byte(editorWhiteBit)
	if !white {
		hi = editorBlackBit
	}
	return hi | lo, true
}

// decodeEditorByte turns an editor-board byte back into a piece character
// ('.', upper-case white, lower-case black), or '?' if unrecognised.
func decodeEditorByte(b byte) byte {
	if b == editorEmpty {
		return '.'
	}
	if b&0xC0 != 0x40 { // not a "present" piece (bit6 set, bit7 clear)
		return '?'
	}
	white := b&0x10 == 0
	var t PieceType
	switch b & 0x0F {
	case 0x0:
		t = Pawn
	case 0x8:
		t = Knight
	case 0xA:
		t = Bishop
	case 0xC:
		t = Rook
	case 0xE:
		t = Queen
	case 0xF:
		t = King
	default:
		return '?'
	}
	c := t.Letter()
	if !white {
		c += 'a' - 'A'
	}
	return c
}

// fenPiece is a parsed piece: its type (high bit tags black) and 0x88 square.
type fenPiece struct {
	typ PieceType
	sq  Square
}

func (p fenPiece) white() bool     { return p.typ&0x80 == 0 }
func (p fenPiece) kind() PieceType { return p.typ &^ 0x80 }

// SetupPosition sets Sargon's board to an arbitrary FEN/EPD position using the
// CTRL-A board editor, then VALIDATES the result by independent read-back and
// returns an error unless the landed position matches the target exactly. It
// also sets and verifies the side-to-move.
//
// Call it at the move-entry prompt (after BootToPrompt), before any move. It
// changes neither level nor Easy Mode. Castling rights and en passant are not
// set explicitly — Sargon infers castling from K/R home squares (correct for
// the opening pool) and does not carry an ep target (see docs/sargon.md).
func (m *Machine) SetupPosition(fen string) error {
	rep, err := m.SetupAndValidate(fen)
	if err != nil {
		return err
	}
	if !rep.OK() {
		return fmt.Errorf("SetupPosition %q failed validation:\n%s", fen, rep)
	}
	return nil
}

// SetupReport captures the target and the independent read-backs, for tests and
// diagnostics.
type SetupReport struct {
	FEN         string
	Target      string // 8-line board, rank 8 first
	PieceList   string // decoded from the $60-$7F slot list (Board())
	EditorBoard string // decoded from the re-entered $80 editor board
	WhiteToMove bool
	STMByte     byte
	STMOK       bool
	Err         string // non-fatal note (e.g. count warnings)
}

// OK reports whether every read-back channel agrees with the target and the
// side-to-move landed correctly.
func (r SetupReport) OK() bool {
	return r.Err == "" &&
		r.PieceList == r.Target &&
		r.EditorBoard == r.Target &&
		r.STMOK
}

func (r SetupReport) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "  target vs piece-list($60) vs editor($80), STM=$%02X ok=%v\n", r.STMByte, r.STMOK)
	tl := strings.Split(r.Target, "\n")
	pl := strings.Split(r.PieceList, "\n")
	el := strings.Split(r.EditorBoard, "\n")
	for i := 0; i < 8; i++ {
		mark := "  "
		if get(tl, i) != get(pl, i) || get(tl, i) != get(el, i) {
			mark = ">>"
		}
		fmt.Fprintf(&sb, "  %s %s   %s   %s\n", mark, get(tl, i), get(pl, i), get(el, i))
	}
	if r.Err != "" {
		fmt.Fprintf(&sb, "  note: %s\n", r.Err)
	}
	return sb.String()
}

func get(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return "????????"
}

// SetupAndValidate performs the setup and returns a full read-back report
// without treating a mismatch as an error (the caller inspects rep.OK()).
func (m *Machine) SetupAndValidate(fen string) (SetupReport, error) {
	pieces, whiteToMove, err := parseFEN(fen)
	if err != nil {
		return SetupReport{}, err
	}
	rep := SetupReport{FEN: fen, WhiteToMove: whiteToMove, Target: renderTarget(pieces)}
	if note := checkPieceCounts(pieces); note != "" {
		rep.Err = note
	}

	// 1. Enter the CTRL-A editor; confirmed by the $60-$6F list being cleared.
	m.Key(0x01)
	if !m.waitEditorOpen() {
		return rep, fmt.Errorf("SetupPosition: CTRL-A editor did not open (screen:\n%s)", m.Screen())
	}
	// 2. Poke the whole 8x8 board.
	m.writeEditorBoard(pieces)
	// 3. Exit with RETURN; confirmed by the piece list being repopulated.
	m.Key(0x0D)
	if !m.waitEditorClosed() {
		return rep, fmt.Errorf("SetupPosition: editor exit did not complete (screen:\n%s)", m.Screen())
	}
	// 4. Side-to-move via CTRL-C, then verify the flag.
	if err := m.setSideToMove(whiteToMove); err != nil {
		return rep, err
	}

	// Read the side-to-move flag now, before the editor re-entry below (which
	// clobbers other zero-page scratch).
	rep.STMByte = m.Peek(stmAddr)
	want := byte(stmWhite)
	if !whiteToMove {
		want = stmBlack
	}
	rep.STMOK = rep.STMByte == want

	// Read-back channel A: the derived $60-$7F piece-slot list.
	rep.PieceList = m.ReadPieceList().Board()
	// Read-back channel B: re-enter the editor so Sargon rebuilds its $80 board
	// from the authoritative state, read the actual piece types, then exit.
	rep.EditorBoard = m.readbackEditorBoard()
	return rep, nil
}

// writeEditorBoard clears the 64 on-board squares then places each piece. It
// touches only real squares (files a-h, ranks 1-8); the 0x88 off-board columns
// hold editor state and are left alone.
func (m *Machine) writeEditorBoard(pieces []fenPiece) {
	for r := 0; r < 8; r++ {
		for f := 0; f < 8; f++ {
			m.Poke(uint16(editorBoardBase+r*16+f), editorEmpty)
		}
	}
	for _, p := range pieces {
		b, ok := editorPieceByte(p.kind(), p.white())
		if !ok {
			continue
		}
		addr := editorBoardBase + int(p.sq.Rank())*16 + int(p.sq.File())
		m.Poke(uint16(addr), b)
	}
}

// readbackEditorBoard re-enters CTRL-A (Sargon rebuilds its $80 board from the
// current authoritative position), decodes the 8x8 board to the same 8-line
// format as PieceList.Board(), then exits the editor leaving the position
// unchanged.
func (m *Machine) readbackEditorBoard() string {
	m.Key(0x01)
	m.waitEditorOpen()
	var sb strings.Builder
	for r := 7; r >= 0; r-- {
		for f := 0; f < 8; f++ {
			sb.WriteByte(decodeEditorByte(m.Peek(uint16(editorBoardBase + r*16 + f))))
		}
		sb.WriteByte('\n')
	}
	m.Key(0x0D)
	m.waitEditorClosed()
	return sb.String()
}

// waitEditorOpen polls until the CTRL-A editor has taken over, signalled by the
// $60-$6F piece list being cleared to the captured sentinel ($80).
func (m *Machine) waitEditorOpen() bool {
	for i := 0; i < 60; i++ {
		if err := m.Run(pollChunk); err != nil {
			return false
		}
		cleared := true
		for a := WhiteListAddr; a < WhiteListAddr+16; a++ {
			if m.Peek(uint16(a)) != editorEmpty {
				cleared = false
				break
			}
		}
		if cleared {
			m.Run(pollChunk) // small settle so the board array is fully init'd
			return true
		}
	}
	return false
}

// waitEditorClosed polls until the editor's exit has rebuilt the piece list:
// the list is no longer all-captured and is stable for two consecutive reads.
func (m *Machine) waitEditorClosed() bool {
	var last PieceList
	stable := 0
	for i := 0; i < 60; i++ {
		if err := m.Run(pollChunk); err != nil {
			return false
		}
		cur := m.ReadPieceList()
		populated := false
		for _, s := range cur {
			if s != editorEmpty {
				populated = true
				break
			}
		}
		if populated && cur == last {
			if stable++; stable >= 2 {
				m.Run(pollChunk)
				return true
			}
		} else {
			stable = 0
		}
		last = cur
	}
	return false
}

// setSideToMove makes it white's or black's turn using CTRL-C, verifying the
// $0002 flag. It is idempotent: it only toggles when the flag is wrong.
func (m *Machine) setSideToMove(whiteToMove bool) error {
	want := byte(stmWhite)
	if !whiteToMove {
		want = stmBlack
	}
	for attempt := 0; attempt < 3; attempt++ {
		if m.Peek(stmAddr) == want {
			return nil
		}
		m.Key(0x03) // CTRL-C
		if err := m.Run(3_000_000); err != nil {
			return err
		}
	}
	if got := m.Peek(stmAddr); got != want {
		return fmt.Errorf("SetupPosition: side-to-move flag $%02X, want $%02X", got, want)
	}
	return nil
}

// renderTarget renders the parsed FEN pieces to the same 8-line board string as
// PieceList.Board() (rank 8 first, white upper-case, black lower-case).
func renderTarget(pieces []fenPiece) string {
	var g [8][8]byte
	for r := range g {
		for f := range g[r] {
			g[r][f] = '.'
		}
	}
	for _, p := range pieces {
		c := p.kind().Letter()
		if !p.white() {
			c += 'a' - 'A'
		}
		g[p.sq.Rank()][p.sq.File()] = c
	}
	var sb strings.Builder
	for r := 7; r >= 0; r-- {
		sb.Write(g[r][:])
		sb.WriteByte('\n')
	}
	return sb.String()
}

// checkPieceCounts returns a non-empty note if the position has piece counts
// the fixed slot list cannot represent (e.g. a promoted 3rd knight/2nd queen),
// which the editor's exit reconciliation would silently drop. Legal opening-pool
// positions never trip this.
func checkPieceCounts(pieces []fenPiece) string {
	limits := map[PieceType]int{Pawn: 8, Knight: 2, Bishop: 2, Rook: 2, Queen: 1, King: 1}
	var wc, bc map[PieceType]int = map[PieceType]int{}, map[PieceType]int{}
	for _, p := range pieces {
		if p.white() {
			wc[p.kind()]++
		} else {
			bc[p.kind()]++
		}
	}
	var notes []string
	for t, lim := range limits {
		if wc[t] > lim {
			notes = append(notes, fmt.Sprintf("white has %d %v (>%d slots)", wc[t], t, lim))
		}
		if bc[t] > lim {
			notes = append(notes, fmt.Sprintf("black has %d %v (>%d slots)", bc[t], t, lim))
		}
	}
	if wc[King] != 1 || bc[King] != 1 {
		notes = append(notes, fmt.Sprintf("kings: white=%d black=%d (need 1 each)", wc[King], bc[King]))
	}
	return strings.Join(notes, "; ")
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
