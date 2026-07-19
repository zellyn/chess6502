package mirror

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// sanToMove finds the unique legal move matching a SAN token.
func sanToMove(eng *Engine, gp *Position, san string) (Move, error) {
	san = strings.TrimRight(san, "+#")
	legal := legalMoves(eng, gp)

	if san == "O-O" || san == "O-O-O" {
		toFile := byte(6)
		if san == "O-O-O" {
			toFile = 2
		}
		for _, m := range legal {
			if m.Flags&FlCastle != 0 && m.To&0x07 == toFile {
				return m, nil
			}
		}
		return NoMove, fmt.Errorf("no castle %q", san)
	}

	promo := byte(0)
	if i := strings.IndexByte(san, '='); i >= 0 {
		promo = map[byte]byte{'N': Knight, 'B': Bishop, 'R': Rook, 'Q': Queen}[san[i+1]]
		san = san[:i]
	}
	if len(san) < 2 {
		return NoMove, fmt.Errorf("bad SAN %q", san)
	}
	to := (san[len(san)-1]-'1')<<4 | (san[len(san)-2] - 'a')
	rest := san[:len(san)-2]
	piece := byte(Pawn)
	if len(rest) > 0 {
		if t, ok := map[byte]byte{'K': King, 'Q': Queen, 'R': Rook, 'B': Bishop, 'N': Knight}[rest[0]]; ok {
			piece = t
			rest = rest[1:]
		}
	}
	rest = strings.TrimSuffix(rest, "x")

	var found []Move
	for _, m := range legal {
		if m.To != to || gp.Board[m.From]&TypeMask != piece || m.Flags&FlPromo != promo {
			continue
		}
		ok := true
		for _, c := range []byte(rest) {
			switch {
			case c >= 'a' && c <= 'h':
				ok = ok && m.From&0x07 == c-'a'
			case c >= '1' && c <= '8':
				ok = ok && m.From>>4 == c-'1'
			}
		}
		if ok {
			found = append(found, m)
		}
	}
	if len(found) != 1 {
		return NoMove, fmt.Errorf("SAN %q: %d matches", san, len(found))
	}
	return found[0], nil
}

// pgnMidgameFENs replays games from a cutechess PGN and returns FENs at
// the given plies.
func pgnMidgameFENs(t *testing.T, path string, games int, plies []int) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("no pgn: %v", err)
	}
	comment := regexp.MustCompile(`\{[^}]*\}`)
	moveno := regexp.MustCompile(`\d+\.`)
	var fens []string
	blocks := strings.Split(string(data), "[Event ")
	eng := NewEngine()
	for _, blk := range blocks {
		if len(fens) >= games*len(plies) || blk == "" {
			continue
		}
		i := strings.Index(blk, "\n\n")
		if i < 0 {
			continue
		}
		text := comment.ReplaceAllString(blk[i:], "")
		text = moveno.ReplaceAllString(text, "")
		fields := strings.Fields(text)
		gp, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
		if err != nil {
			t.Fatal(err)
		}
		want := map[int]bool{}
		for _, p := range plies {
			want[p] = true
		}
		for ply, tok := range fields {
			if tok == "1-0" || tok == "0-1" || tok == "1/2-1/2" || tok == "*" {
				break
			}
			m, err := sanToMove(eng, gp, tok)
			if err != nil {
				t.Fatalf("%s game: ply %d: %v", path, ply, err)
			}
			if err := applyUCI(eng, gp, m.UCI()); err != nil {
				t.Fatal(err)
			}
			if want[ply+1] {
				fens = append(fens, gp.FEN())
			}
		}
	}
	return fens
}

// benchFENs: the treesize position plus middlegame positions from real
// engine games (tools/pgn), deduped (deterministic games can repeat
// whole lines), the A/B measurement set.
func benchFENs(t *testing.T) []string {
	fens := []string{
		"r1b1k2r/ppp2ppp/2nqpn2/3p4/3P4/P1P1BN2/2P1PPPP/2RQKB1R w Kkq - 2 8",
	}
	seen := map[string]bool{fens[0]: true}
	for _, f := range pgnMidgameFENs(t,
		"../../tools/pgn/chess6502_vs_tscp_d3_lmr2.pgn", 4, []int{16, 24, 32}) {
		if !seen[f] {
			seen[f] = true
			fens = append(fens, f)
		}
	}
	return fens
}

// TestLMRSweepNodes: depth-6 node counts across LMR parameter variants
// (phase A of the sweep; promising variants graduate to self-play).
func TestLMRSweepNodes(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	variants := []struct {
		name string
		p    LMRParams
	}{
		{"base 4,7,3,5,k0,e1", DefaultLMR},
		{"late1=3", LMRParams{3, 7, 3, 5, false, true}},
		{"late1=2", LMRParams{2, 7, 3, 5, false, true}},
		{"late2=6", LMRParams{4, 6, 3, 5, false, true}},
		{"late2=8", LMRParams{4, 8, 3, 5, false, true}},
		{"3,6", LMRParams{3, 6, 3, 5, false, true}},
		{"2,5", LMRParams{2, 5, 3, 5, false, true}},
		{"rem1=2", LMRParams{4, 7, 2, 5, false, true}},
		{"rem2=4", LMRParams{4, 7, 3, 4, false, true}},
		{"3,6,2,4", LMRParams{3, 6, 2, 4, false, true}},
		{"killers", LMRParams{4, 7, 3, 5, true, true}},
		{"3,6+killers", LMRParams{3, 6, 3, 5, true, true}},
		{"no-evasion-pvs", LMRParams{4, 7, 3, 5, false, false}},
		{"2,6,2,4+killers", LMRParams{2, 6, 2, 4, true, true}},
	}
	fens := benchFENs(t)
	var baseTotal uint64
	for vi, v := range variants {
		var total uint64
		for _, fen := range fens {
			pos, err := ParseFEN(fen)
			if err != nil {
				t.Fatal(err)
			}
			eng := NewEngine()
			eng.LMR = v.p
			eng.SetPosition(pos)
			eng.SearchFixed(6)
			total += eng.Nodes
		}
		if vi == 0 {
			baseTotal = total
		}
		t.Logf("%-18s total %8d nodes (%+.1f%% vs base)",
			v.name, total, 100*(float64(total)/float64(baseTotal)-1))
	}
}

// TestQSShapeNodes: depth-6 total and QS node counts across QS-shape
// variants (phase A; the interesting frontier is 30-50% cheaper QS at
// small fixed-depth Elo cost).
func TestQSShapeNodes(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	variants := []struct {
		name string
		qs   QSParams
	}{
		{"base (uncapped)", QSParams{}},
		{"cap4", QSParams{PlyCap: 4}},
		{"cap6", QSParams{PlyCap: 6}},
		{"cap8", QSParams{PlyCap: 8}},
		{"recap2", QSParams{RecapAfter: 2}},
		{"recap3", QSParams{RecapAfter: 3}},
		{"recap4", QSParams{RecapAfter: 4}},
		{"cap8+recap2", QSParams{PlyCap: 8, RecapAfter: 2}},
		{"cap8+recap4", QSParams{PlyCap: 8, RecapAfter: 4}},
		{"cap6+recap2", QSParams{PlyCap: 6, RecapAfter: 2}},
		{"recap1", QSParams{RecapAfter: 1}},
		{"cap3", QSParams{PlyCap: 3}},
		{"cap2", QSParams{PlyCap: 2}},
		{"cap4+recap1", QSParams{PlyCap: 4, RecapAfter: 1}},
		{"cap4+recap2", QSParams{PlyCap: 4, RecapAfter: 2}},
	}
	fens := benchFENs(t)
	var baseTotal, baseQS uint64
	for vi, v := range variants {
		var total, qsn uint64
		for _, fen := range fens {
			pos, err := ParseFEN(fen)
			if err != nil {
				t.Fatal(err)
			}
			eng := NewEngine()
			eng.QS = v.qs
			eng.SetPosition(pos)
			eng.SearchFixed(6)
			total += eng.Nodes
			qsn += eng.QSNodes
		}
		if vi == 0 {
			baseTotal, baseQS = total, qsn
		}
		t.Logf("%-16s total %8d (%+6.1f%%)  qs %8d (%+6.1f%%, %2.0f%% of nodes)",
			v.name, total, 100*(float64(total)/float64(baseTotal)-1),
			qsn, 100*(float64(qsn)/float64(baseQS)-1), 100*float64(qsn)/float64(total))
	}
}

// TestFutilityGuardNodes: fixed-depth node counts, current vs fixed
// mate-zone guard, all features on.
func TestFutilityGuardNodes(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	var curTotal, fixTotal uint64
	for i, fen := range benchFENs(t) {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		var n [2]uint64
		for j, fix := range []bool{false, true} {
			eng := NewEngine()
			eng.FixFutilityGuard = fix
			eng.SetPosition(pos)
			eng.SearchFixed(6)
			n[j] = eng.Nodes
		}
		curTotal += n[0]
		fixTotal += n[1]
		t.Logf("fen %d: current %8d  fixed %8d  (%+.1f%%)  %s",
			i, n[0], n[1], 100*(float64(n[1])/float64(n[0])-1), fen)
	}
	t.Logf("TOTAL: current %d  fixed %d  (%+.1f%%)",
		curTotal, fixTotal, 100*(float64(fixTotal)/float64(curTotal)-1))
}
