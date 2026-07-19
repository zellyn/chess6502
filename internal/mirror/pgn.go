package mirror

import (
	"fmt"
	"os"
	"regexp"
	"strings"
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

// PGNStats reports what a PGN extraction pass consumed and produced.
type PGNStats struct {
	Games   int // games replayed to a decisive/drawn result
	Skipped int // games skipped (no result token, or parse error)
	Samples int // quiet labeled positions collected
}

// pgnQuietSample returns the fixed eval base (white POV) and pawn-
// structure features for gp if it is a quiet, not-in-check position,
// mirroring the self-play collector in PlayGame. The engine is used as
// scratch and left pointing at gp.
func pgnQuietSample(eng *Engine, gp *Position) (Sample, bool) {
	eng.SetPosition(gp)
	if eng.curInCheck() {
		return Sample{}, false
	}
	if !isQuiet(eng) {
		return Sample{}, false
	}
	eng.SetPosition(gp)
	base := eng.Pos.taperedWhite()
	if gp.Side == 0 {
		base += Tempo
	} else {
		base -= Tempo
	}
	return Sample{Base: base, F: *extractPawnFeatures(&eng.Pos)}, true
}

var pgnComment = regexp.MustCompile(`\{[^}]*\}`)
var pgnMoveNo = regexp.MustCompile(`\d+\.`)
var pgnFENTag = regexp.MustCompile(`\[FEN\s+"([^"]*)"\]`)

const startposFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

// PGNFenRows replays a PGN file and returns quiet, non-check positions
// as FEN + white-POV result, mirroring PGNSamples' gates. This is the
// king-bucket corpus source (which needs board placement).
func PGNFenRows(path string) ([]FenRow, PGNStats, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, PGNStats{}, err
	}
	eng := NewEngine()
	var out []FenRow
	var st PGNStats
	for _, blk := range strings.Split(string(data), "[Event ") {
		if blk == "" {
			continue
		}
		i := strings.Index(blk, "\n\n")
		if i < 0 {
			continue
		}
		header := blk[:i]
		text := pgnMoveNo.ReplaceAllString(pgnComment.ReplaceAllString(blk[i:], ""), "")
		fields := strings.Fields(text)
		if len(fields) == 0 {
			continue
		}
		result, ok := map[string]float64{"1-0": 1, "0-1": 0, "1/2-1/2": 0.5}[fields[len(fields)-1]]
		if !ok {
			st.Skipped++
			continue
		}
		startFEN := startposFEN
		if mm := pgnFENTag.FindStringSubmatch(header); mm != nil && strings.TrimSpace(mm[1]) != "" {
			startFEN = mm[1]
		}
		gp, err := ParseFEN(startFEN)
		if err != nil {
			st.Skipped++
			continue
		}
		plyGate := 8
		if startFEN != startposFEN {
			plyGate = 2
		}
		var pending []FenRow
		bad := false
		gate := 0
		for ply, tok := range fields {
			if tok == "1-0" || tok == "0-1" || tok == "1/2-1/2" || tok == "*" {
				break
			}
			m, err := sanToMove(eng, gp, tok)
			if err != nil {
				bad = true
				break
			}
			if err := applyUCI(eng, gp, m.UCI()); err != nil {
				bad = true
				break
			}
			if ply >= plyGate {
				if _, ok := pgnQuietSample(eng, gp); ok {
					gate++
					if gate%2 == 0 { // every-2nd, matching self-play density
						pending = append(pending, FenRow{FEN: gp.FEN(), R: result})
					}
				}
			}
		}
		if bad {
			st.Skipped++
			continue
		}
		out = append(out, pending...)
		st.Games++
		st.Samples += len(pending)
	}
	return out, st, nil
}

// PGNSamples replays every game in a cutechess-style PGN file and
// collects quiet, non-check, labeled positions after ply gate. Sampling
// mirrors the self-play collector (ply >= 8, quiet, not in check) but
// takes every everyN-th qualifying position (everyN <= 1 takes all);
// external games are far less self-correlated than self-play, so a
// smaller stride is fine. The result label is the game outcome
// (white POV: 1 / 0.5 / 0).
func PGNSamples(path string, everyN int) ([]Sample, PGNStats, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, PGNStats{}, err
	}
	if everyN < 1 {
		everyN = 1
	}
	eng := NewEngine()
	var out []Sample
	var st PGNStats
	blocks := strings.Split(string(data), "[Event ")
	for _, blk := range blocks {
		if blk == "" {
			continue
		}
		i := strings.Index(blk, "\n\n")
		if i < 0 {
			continue
		}
		header := blk[:i]
		text := pgnComment.ReplaceAllString(blk[i:], "")
		text = pgnMoveNo.ReplaceAllString(text, "")
		fields := strings.Fields(text)
		if len(fields) == 0 {
			continue
		}
		result, ok := map[string]float64{"1-0": 1, "0-1": 0, "1/2-1/2": 0.5}[fields[len(fields)-1]]
		if !ok {
			st.Skipped++
			continue
		}
		// Honor a [FEN "..."] setup header (the rating-pool games start
		// from openings-pool.epd positions, not the standard start).
		startFEN := startposFEN
		if mm := pgnFENTag.FindStringSubmatch(header); mm != nil && strings.TrimSpace(mm[1]) != "" {
			startFEN = mm[1]
		}
		gp, err := ParseFEN(startFEN)
		if err != nil {
			st.Skipped++
			continue
		}
		// Standard-start games carry ~8 plies of opening book to skip;
		// pool games already start from a curated mid-game setup, so
		// only skip the first couple near-duplicate plies.
		plyGate := 8
		if startFEN != startposFEN {
			plyGate = 2
		}
		var pending []Sample
		gate := 0
		bad := false
		for ply, tok := range fields {
			if tok == "1-0" || tok == "0-1" || tok == "1/2-1/2" || tok == "*" {
				break
			}
			m, err := sanToMove(eng, gp, tok)
			if err != nil {
				bad = true
				break
			}
			if err := applyUCI(eng, gp, m.UCI()); err != nil {
				bad = true
				break
			}
			// gp now holds the position after this move (ply index ply).
			if ply >= plyGate {
				if s, ok := pgnQuietSample(eng, gp); ok {
					gate++
					if gate%everyN == 0 {
						pending = append(pending, s)
					}
				}
			}
		}
		if bad {
			st.Skipped++
			continue
		}
		for i := range pending {
			pending[i].R = result
		}
		out = append(out, pending...)
		st.Games++
		st.Samples += len(pending)
	}
	return out, st, nil
}
