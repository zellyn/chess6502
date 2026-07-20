package sargon

import (
	"fmt"
	"strings"
)

// Timing / step budgets. The emulator runs ~1 CPU cycle per Step, i.e. ~1e6
// steps per emulated second on a 1 MHz Apple II.
const (
	// MaxBootSteps bounds the wait for the program to reach its move-entry
	// state. Sargon III finishes loading from the (slow, emulated) Disk II in
	// roughly 100M cycles.
	MaxBootSteps = 160_000_000

	// pollChunk is how many CPU steps to run between checks while waiting.
	pollChunk = 500_000

	// BootSettleSteps is run after the board first appears, to let Sargon
	// finish loading before the first move is entered (~30M cycles margin).
	BootSettleSteps = 35_000_000
)

// initialWhitePawns is the piece-list signature that appears once Sargon has
// set up the board and is waiting for the first move: white pawns on a2-h2.
var initialWhitePawns = [8]Square{0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17}

// BootToPrompt runs the CPU until Sargon III has loaded and set up its board
// (the piece list shows the initial position), i.e. it is ready for the first
// move. Returns an error if that state isn't reached within MaxBootSteps.
func (m *Machine) BootToPrompt() error {
	ready := func(m *Machine) bool {
		for i, want := range initialWhitePawns {
			if Square(m.A2.RamRead(uint16(WhiteListAddr+i))) != want {
				return false
			}
		}
		// Also require the title to be on screen, to avoid matching a
		// transient identical-looking table during load.
		return m.ScreenContains("SARGON III")
	}
	ok, err := m.RunUntil(MaxBootSteps, ready)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("sargon did not reach move-entry state within %d steps", MaxBootSteps)
	}
	// The board is set up well before Sargon finishes loading the rest of the
	// program (opening book / overlays) from the emulated Disk II. Entering a
	// move during that window corrupts later input, so settle first.
	return m.Run(BootSettleSteps)
}

// Level sets Sargon's playing level (1-9) by injecting the corresponding
// shifted digit (SHIFT-n). Must be called when it is the player's turn.
//
// Level -> avg response time (per the manual): 1=5s, 2=15s, 3=30s, 4=1m,
// 5=2m, 6=3m, 7=6m, 8=10m, 9=infinite. Use level 3 for ~30s/move.
func (m *Machine) Level(n int) error {
	if n < 1 || n > 9 {
		return fmt.Errorf("level %d out of range 1-9", n)
	}
	// SHIFT-<digit> on the Apple ][+ keyboard: 1..9 -> ! @ # $ % ^ & * (
	shifted := []byte("!@#$%^&*(")
	m.Key(shifted[n-1])
	// Give the key time to be consumed.
	return m.Run(pollChunk)
}

// EasyMode enables Sargon's Easy Mode (CTRL-E), which stops it from thinking on
// the player's time (pondering). This is strongly recommended when driving
// Sargon programmatically: while pondering, Sargon continuously overwrites its
// $60-$7F piece list as search scratch, so the board can only be read reliably
// from RAM (and our own moves confirmed) once pondering is off. It also makes
// per-move timing ponder-free and reproducible, matching a non-pondering
// opponent. Per the manual it roughly halves Sargon's effective strength at a
// given level, so choose the level accordingly. Call once, on the player's turn.
func (m *Machine) EasyMode() error {
	m.Key(0x05) // CTRL-E
	return m.Run(pollChunk)
}

// MoveResult describes the outcome of submitting a player move.
type MoveResult struct {
	// SargonMove is Sargon's reply, decoded from the piece-list diff.
	SargonMove Move
	// SargonText is Sargon's reply as shown on the text move-list (e.g.
	// "E7-E6", "E4XD5", "0-0"), best-effort.
	SargonText string
	// Message is any message-line text detected (CHECK, CHECKMATE, etc.).
	Message string
	// GameOver is set if a terminal message (mate/stalemate/draw) was seen.
	GameOver bool
	// Board is the RAM piece list read once Sargon returned to idle.
	Board PieceList
}

// SubmitMove types a player move (e.g. "E2-E4", or "E4-D5" for a capture; add
// a trailing promotion piece letter like "E7-E8N" to under-promote) followed
// by Return, waits for Sargon to accept it and reply, and returns Sargon's
// reply decoded from RAM plus the on-screen text.
//
// maxThinkSteps bounds how long to wait for Sargon's reply (e.g. ~10M for
// level 1, more for higher levels). It errors if the move is rejected as
// illegal or no reply appears in time.
func (m *Machine) SubmitMove(move string, maxThinkSteps uint64) (MoveResult, error) {
	var res MoveResult
	move = strings.ToUpper(strings.TrimSpace(move))

	// Parse the from/to squares so we can confirm our own move actually
	// registered (keystrokes are occasionally dropped, and without this check
	// a stale piece-list change would be mistaken for Sargon's reply, causing
	// desync). move is "E2-E4" or "E4XD5" (we accept '-' or 'X'); a trailing
	// promotion letter is ignored for the square parse.
	from, to, ok := parseFromTo(move)
	if !ok {
		return res, fmt.Errorf("cannot parse move %q", move)
	}

	// Settle: after moving, Sargon redraws before returning to its keyboard
	// loop; typing during that window drops characters.
	if err := m.Run(3_000_000); err != nil {
		return res, err
	}

	before := m.ReadPieceList()
	fromIdx := whiteIndexAt(before, from)
	if fromIdx < 0 {
		return res, fmt.Errorf("no white piece on %s for move %q", from.Algebraic(), move)
	}

	// Type the move, then confirm it registered (white mover now on `to`).
	// Retry on a dropped/garbled entry, clearing the input line first.
	const stepsPerKey = 200_000
	accepted := false
	for attempt := 0; attempt < 4 && !accepted; attempt++ {
		if attempt > 0 {
			// Clear a partial/garbled entry: Return submits & rejects it,
			// re-prompting an empty field.
			m.Key(0x0D)
			if err := m.Run(1_500_000); err != nil {
				return res, err
			}
		}
		if err := m.TypePaced(move+"\r", stepsPerKey); err != nil {
			return res, err
		}
		// Wait up to ~4M cycles for our white piece to land on `to`.
		var s uint64
		for s < 4_000_000 {
			if err := m.Run(pollChunk); err != nil {
				return res, err
			}
			s += pollChunk
			if Square(m.A2.RamRead(uint16(WhiteListAddr+fromIdx))) == to {
				accepted = true
				break
			}
		}
	}
	if !accepted {
		return res, fmt.Errorf("move %q not accepted after retries (screen: %q)", move, m.messageLine())
	}

	// Snapshot after our move applied (captures any black piece we took).
	// Sargon rewrites the $60-$7F piece list in place during its search, so
	// the list is only trustworthy when Sargon is idle at the input prompt.
	// The authoritative "Sargon has replied" signal is therefore the text
	// move-list: wait until a new token appears in the SARGON column.
	mid := m.ReadPieceList()
	prevReply := m.scrapeReplyText()

	var steps uint64
	replied := false
	for steps < maxThinkSteps {
		if err := m.Run(pollChunk); err != nil {
			return res, err
		}
		steps += pollChunk
		if tok := m.scrapeReplyText(); tok != "" && tok != prevReply {
			replied = true
			break
		}
	}
	if !replied {
		res.Message, res.GameOver = m.scrapeMessage()
		if res.GameOver {
			return res, nil // mate/stalemate: our move ended the game
		}
		return res, fmt.Errorf("no reply from sargon within %d steps after move %q", maxThinkSteps, move)
	}

	// Let Sargon fully return to the idle input loop before trusting the
	// piece list, then decode the board from RAM as a cross-check.
	if err := m.Run(2_000_000); err != nil {
		return res, err
	}
	res.SargonText = m.scrapeReplyText()
	after := m.ReadPieceList()
	if mv, ok := DiffMove(mid, after, true /* black */); ok {
		res.SargonMove = mv
	}
	res.Board = after
	res.Message, res.GameOver = m.scrapeMessage()
	return res, nil
}

// parseFromTo extracts the from/to squares from a move like "E2-E4", "E4XD5",
// or "E7-E8Q". Separator may be '-' or 'X'.
func parseFromTo(move string) (from, to Square, ok bool) {
	if len(move) < 5 {
		return 0, 0, false
	}
	from, ok1 := ParseSquare(move[0:2])
	if move[2] != '-' && move[2] != 'X' {
		return 0, 0, false
	}
	to, ok2 := ParseSquare(move[3:5])
	return from, to, ok1 && ok2
}

// whiteIndexAt returns the white piece-list index (0-15) occupying square sq,
// or -1.
func whiteIndexAt(pl PieceList, sq Square) int {
	for i := 0; i < 16; i++ {
		if pl[i] == sq {
			return i
		}
	}
	return -1
}

// messageLine returns the trimmed contents of the message-line row (row 5).
func (m *Machine) messageLine() string {
	return strings.TrimSpace(m.TextRow(5))
}

// blackChanged reports whether any black piece-list entry (16-31) differs.
func blackChanged(a, b PieceList) bool {
	for i := 16; i < 32; i++ {
		if a[i] != b[i] {
			return true
		}
	}
	return false
}

// scrapeReplyText returns Sargon's most-recent move token from the on-screen
// move list (the last non-empty entry in the SARGON column). Best-effort.
func (m *Machine) scrapeReplyText() string {
	last := ""
	for r := 10; r < 24; r++ {
		row := m.TextRow(r)
		if len(row) < 28 {
			continue
		}
		tok := strings.TrimSpace(row[22:28])
		if tok != "" {
			last = tok
		}
	}
	return last
}

// scrapeMessage scans the screen for status keywords and reports the first one
// found plus whether it is terminal.
func (m *Machine) scrapeMessage() (msg string, gameOver bool) {
	terminal := []string{"CHECKMATE", "STALEMATE", "DRAW", "MATE", "RESIGN"}
	for _, k := range terminal {
		if m.ScreenContains(k) {
			return k, true
		}
	}
	if m.ScreenContains("CHECK") {
		return "CHECK", false
	}
	return "", false
}
