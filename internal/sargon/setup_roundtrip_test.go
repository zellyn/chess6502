package sargon

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"
)

const dskPath = "../../assets/sargon-iii.dsk"

// loadEPD returns the FEN strings from tools/openings-pool.epd.
func loadEPD(t *testing.T) []string {
	f, err := os.Open("../../tools/openings-pool.epd")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// edgeCases are hand-picked positions exercising a/h files, 1st/8th ranks, both
// colours to move, sparse and crowded boards, and (where Sargon can represent
// them) castling-relevant K/R placement.
var edgeCases = []struct {
	name string
	fen  string
}{
	{"kings-only", "4k3/8/8/8/8/8/8/4K3 w - -"},                                   // sparest legal board, white to move
	{"kings-only-b", "4k3/8/8/8/8/8/8/4K3 b - -"},                                 // same, black to move
	{"knights-corners", "n6n/8/8/8/8/8/8/N6N w - -"},                             // pieces on a/h files, ranks 1 & 8
	{"rooks-corners", "r3k2r/8/8/8/8/8/8/R3K2R w KQkq -"},                        // K & R on home squares (castling)
	{"black-to-move", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR b KQkq -"},    // standard, black to move
	{"pawns-edge", "4k3/P6P/8/8/8/8/p6p/4K3 w - -"},                              // pawns on a/h files
	{"crowded", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -"},          // full start (crowded)
	{"full-back", "rnbqkbnr/8/8/8/8/8/8/RNBQKBNR w KQkq -"},                      // both back ranks only
	{"scattered", "7k/8/8/3Q4/4q3/8/8/K7 b - -"},                                 // queens mid-board, kings a1/h8
}

func TestSetupRoundTripPool(t *testing.T) {
	if os.Getenv("SARGON_SLOW") == "" {
		t.Skip("set SARGON_SLOW=1 (drives the emulator; ~minutes)")
	}
	fens := loadEPD(t)
	m, err := NewMachine(dskPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.BootToPrompt(); err != nil {
		t.Fatal(err)
	}
	pass, fail := 0, 0
	for _, fen := range fens {
		id := fen
		if i := strings.Index(fen, "id "); i >= 0 {
			id = strings.Trim(fen[i+3:], " ;\"")
		}
		rep, err := m.SetupAndValidate(fen)
		if err != nil {
			t.Errorf("%s: setup error: %v", id, err)
			fail++
			continue
		}
		if !rep.OK() {
			t.Errorf("%s: round-trip MISMATCH:\n%s", id, rep)
			fail++
			continue
		}
		pass++
		fmt.Printf("OK  %-10s stm=$%02X\n", id, rep.STMByte)
	}
	fmt.Printf("\npool round-trip: %d passed, %d failed of %d\n", pass, fail, len(fens))
}

func TestSetupRoundTripEdges(t *testing.T) {
	if os.Getenv("SARGON_SLOW") == "" {
		t.Skip("set SARGON_SLOW=1")
	}
	m, err := NewMachine(dskPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.BootToPrompt(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range edgeCases {
		rep, err := m.SetupAndValidate(tc.fen)
		if err != nil {
			t.Errorf("%s: setup error: %v", tc.name, err)
			continue
		}
		status := "OK "
		if !rep.OK() {
			status = "XX "
		}
		fmt.Printf("%s %-14s stm=$%02X note=%q\n", status, tc.name, rep.STMByte, rep.Err)
		if !rep.OK() && rep.Err == "" {
			// Only fail when the position is representable but mismatched.
			t.Errorf("%s: MISMATCH:\n%s", tc.name, rep)
		}
	}
}
