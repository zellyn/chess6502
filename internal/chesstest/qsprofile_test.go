package chesstest

import "testing"

// TestQSProfile: where do quiescence cycles go? Splits a budget
// search's cycles into full-width plies (PLY < MAXDEPTH) and qs plies
// (PLY >= MAXDEPTH), with a per-routine breakdown of each.
func TestQSProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("diagnostic: run explicitly")
	}
	fens := []string{
		"r1b1k2r/ppp2ppp/2nqpn2/3p4/3P4/P1P1BN2/2P1PPPP/2RQKB1R w Kkq - 2 8",
		"r2q1rk1/pp1nbppp/2p1pn2/3p2B1/2PP4/2N1PN2/PPQ2PPP/R3KB1R w KQ - 6 8",
	}
	bin := loadEngine(t)
	labels, err := ParseLabelFile("../../asm/engine.lbl")
	if err != nil {
		t.Fatal(err)
	}
	for _, fen := range fens {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		m, err := NewMachine(bin, defs, pos, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		SetBudget(m, defs, 30_000_000, 24)
		m.Mem.Main[defs["HALFMOVE"]] = pos.Halfmove
		full := &Profile{PCCycles: make([]uint64, 65536)}
		qs := &Profile{PCCycles: make([]uint64, 65536)}
		plyAddr, mdAddr := defs["PLY"], defs["MAXDEPTH"]
		exited, code, err := m.RunProfile(400_000_000_000, func(pc uint16, cycles uint8) {
			p := full
			if m.Mem.Main[plyAddr] >= m.Mem.Main[mdAddr] {
				p = qs
			}
			c := uint64(cycles)
			p.PCCycles[pc] += c
			p.Total += c
		})
		if err != nil || !exited || code > 2 {
			t.Fatalf("exited=%v code=%d err=%v", exited, code, err)
		}
		tot := full.Total + qs.Total
		t.Logf("%s\n  full-width: %dM (%d%%)  qs: %dM (%d%%)", fen[:20],
			full.Total/1e6, 100*full.Total/tot, qs.Total/1e6, 100*qs.Total/tot)
		for name, p := range map[string]*Profile{"FULL": full, "QS": qs} {
			out := name + ":"
			for i, r := range p.ByRoutine(labels) {
				if i >= 10 {
					break
				}
				out += " " + r.Name + "=" + itoaPct(r.Cycles, tot)
			}
			t.Log(out)
		}
	}
}

func itoaPct(c, tot uint64) string {
	return string(rune('0'+(1000*c/tot)/100)) + string(rune('0'+(1000*c/tot)/10%10)) + "." + string(rune('0'+(1000*c/tot)%10)) + "%"
}
