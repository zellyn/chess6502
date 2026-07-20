package sargon

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// pieceColorAt returns 'W', 'b', or '.' for the target square in the FEN.
func pieceColorAt(pieces []fenPiece, sq Square) byte {
	for _, p := range pieces {
		if p.sq == sq {
			if p.white() {
				return 'W'
			}
			return 'b'
		}
	}
	return '.'
}

// newestMoveToken scans the whole move list and returns the bottom-most
// non-empty token in either column, tagged with the column.
func newestMoveToken(m *Machine) (tok string, rightCol bool) {
	for r := 23; r >= 10; r-- {
		row := m.TextRow(r)
		if len(row) < 30 {
			continue
		}
		if t := strings.TrimSpace(row[22:30]); t != "" {
			return t, true
		}
		if t := strings.TrimSpace(row[10:18]); t != "" {
			return t, false
		}
	}
	return "", false
}

// TestSetupSideToMoveBehavior sets positions of both colours (a fresh boot per
// case, matching real gauntlet usage) and confirms that when Sargon is asked to
// move (CTRL-S takes the side to move), it plays a piece of the side-to-move
// colour -- the authoritative check that side-to-move landed correctly.
func TestSetupSideToMoveBehavior(t *testing.T) {
	if os.Getenv("SARGON_SLOW") == "" {
		t.Skip("set SARGON_SLOW=1")
	}
	cases := []struct {
		name string
		fen  string
		want byte
	}{
		{"pool-00 white", "rn1qkb1r/pp2pppp/2p4n/3pPb2/3P1B2/8/PPP2PPP/RN1QKBNR w KQkq -", 'W'},
		{"pool-00 black", "rn1qkb1r/pp2pppp/2p4n/3pPb2/3P1B2/8/PPP2PPP/RN1QKBNR b KQkq -", 'b'},
		{"pool-13 white", "r1bqk1nr/1ppp1ppp/2n5/p1b1p3/2B1P3/2N2N2/PPPP1PPP/R1BQK2R w KQkq a6", 'W'},
		{"black-open", "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq -", 'b'},
	}
	for _, tc := range cases {
		m, err := NewMachine(dskPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := m.BootToPrompt(); err != nil {
			t.Fatal(err)
		}
		if err := m.EasyMode(); err != nil {
			t.Fatal(err)
		}
		m.Run(1_000_000)
		if err := m.SetupPosition(tc.fen); err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		pieces, _, _ := parseFEN(tc.fen)
		if err := m.InfiniteLevel(); err != nil {
			t.Fatal(err)
		}
		before, _ := newestMoveToken(m)
		m.Key(0x13) // CTRL-S: Sargon takes the side to move and plays it.
		var got string
		var col bool
		for i := 0; i < 100; i++ {
			m.Run(500_000)
			if tok, c := newestMoveToken(m); tok != "" && tok != before {
				got, col = tok, c
				break
			}
			if i == 50 {
				m.ForceMove() // CTRL-T if no book move yet
			}
		}
		if got == "" {
			t.Errorf("%s: Sargon made no move (screen:\n%s)", tc.name, m.Screen())
			continue
		}
		from, _ := ParseSquare(got[0:2])
		gotColor := pieceColorAt(pieces, from)
		colName := "left-col"
		if col {
			colName = "right-col"
		}
		fmt.Printf("%-14s Sargon played %-8q from %s -> %c piece (%s); want %c\n",
			tc.name, got, from.Algebraic(), gotColor, colName, tc.want)
		if gotColor != tc.want {
			t.Errorf("%s: Sargon moved a %c piece from %s, want side-to-move %c",
				tc.name, gotColor, from.Algebraic(), tc.want)
		}
	}
}
