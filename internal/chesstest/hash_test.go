package chesstest

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// parseLabels reads an ld65 -Ln VICE label file ("al 00C000 .LABEL").
func parseLabels(t *testing.T, path string) map[string]uint16 {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	labels := map[string]uint16{}
	for line := range strings.SplitSeq(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) >= 3 && f[0] == "al" && strings.HasPrefix(f[2], ".") {
			if v, err := strconv.ParseUint(f[1], 16, 32); err == nil && v <= 0xFFFF {
				labels[f[2][1:]] = uint16(v)
			}
		}
	}
	return labels
}

// TestHashConsistency: after a search fully unwinds, the engine's
// incremental Zobrist hash must equal a from-scratch computation done in
// Go using the key tables read out of the engine binary's own memory.
// Any make/unmake hash asymmetry perturbs this.
func TestHashConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("slow: full searches")
	}
	labels := parseLabels(t, filepath.Join("..", "..", "asm", "engine.lbl"))
	for _, name := range []string{"ZPLANE0", "STMKEY", "CASTKEYS", "EPKEYS", "KINDTAB"} {
		if _, ok := labels[name]; !ok {
			t.Fatalf("label %s missing from engine.lbl", name)
		}
	}
	fens := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 1 8",
		"8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 b - - 3 9",
		"rnbqkbnr/ppppp1pp/8/8/4Pp2/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 3",
	}
	for _, fen := range fens {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		m, err := NewMachine(loadEngine(t), defs, pos, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		m.Mem.Main[defs["MAXDEPTH"]] = 3
		m.Mem.Main[defs["HALFMOVE"]] = pos.Halfmove
		if exited, _, err := m.Run(20_000_000_000); err != nil || !exited {
			t.Fatalf("%s: exited=%v err=%v", fen, exited, err)
		}

		var want [4]byte
		xor4 := func(addr uint16) {
			for i := range want {
				want[i] ^= m.Mem.Main[addr+uint16(i)]
			}
		}
		zplane0 := labels["ZPLANE0"]
		for slot, sq := range pos.PieceSq {
			if sq == 0xFF {
				continue
			}
			piece := pos.Board[sq]
			kind := m.Mem.Main[labels["KINDTAB"]+uint16(piece&0x0F)]
			// The four planes are contiguous 1536-byte blocks.
			for p := range want {
				want[p] ^= m.Mem.Main[zplane0+uint16(p)*1536+uint16(kind)*128+uint16(sq)]
			}
			_ = slot
		}
		if pos.Side != 0 {
			xor4(labels["STMKEY"])
		}
		xor4(labels["CASTKEYS"] + 4*uint16(pos.Castle))
		if pos.EpSq != 0xFF {
			xor4(labels["EPKEYS"] + 4*uint16(pos.EpSq&7))
		}

		var got [4]byte
		copy(got[:], m.Mem.Main[defs["HASH0"]:defs["HASH0"]+4])
		if got != want {
			t.Errorf("%s: hash after search = %x, want %x", fen, got, want)
		}
	}
}
