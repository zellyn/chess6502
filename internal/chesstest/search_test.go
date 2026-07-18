package chesstest

import (
	"os"
	"path/filepath"
	"testing"
)

var engineBin []byte

func loadEngine(t *testing.T) []byte {
	t.Helper()
	if engineBin == nil {
		var err error
		engineBin, err = os.ReadFile(filepath.Join("..", "..", "asm", "engine.bin"))
		if err != nil {
			t.Fatal(err)
		}
	}
	return engineBin
}

const mate = 30000

// moveMatches supports a trailing "*" wildcard for the promotion piece.
func moveMatches(got, want string) bool {
	if len(want) > 0 && want[len(want)-1] == '*' {
		return len(got) >= len(want)-1 && got[:len(want)-1] == want[:len(want)-1]
	}
	return got == want
}

func TestMates(t *testing.T) {
	cases := []struct {
		name    string
		fen     string
		depth   byte
		want    string // expected move; "" = any
		score   int16  // exact expected score; 1 = "just positive"
		noMoves bool
	}{
		{name: "back-rank-mate-in-1", fen: "6k1/5ppp/8/8/8/8/8/R6K w - - 0 1",
			depth: 2, want: "a1a8", score: mate - 1},
		{name: "scholars-mate-in-1", fen: "r1bqkb1r/pppp1ppp/2n2n2/4p2Q/2B1P3/8/PPPP1PPP/RNB1K1NR w KQkq - 4 4",
			depth: 2, want: "h5f7", score: mate - 1},
		{name: "black-mate-in-1", fen: "8/8/8/8/8/1k6/2q5/K7 b - - 0 1",
			depth: 2, want: "", score: mate - 1},
		{name: "kr-mate-in-2", fen: "k7/8/2K5/8/8/8/8/6R1 w - - 0 1",
			depth: 3, want: "c6b6", score: mate - 3},
		{name: "stalemate", fen: "k7/8/1Q6/8/8/8/8/K7 b - - 0 1",
			depth: 2, noMoves: true, score: 0},
		{name: "already-mated", fen: "k6R/8/1K6/8/8/8/8/8 b - - 0 1",
			depth: 2, noMoves: true, score: -mate},
		{name: "promotion-mates", fen: "k7/2P5/1K6/8/8/8/8/8 w - - 0 1",
			depth: 2, want: "c7c8*", score: mate - 1}, // c8=Q and c8=R both mate
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pos, err := ParseFEN(tc.fen)
			if err != nil {
				t.Fatal(err)
			}
			res, err := SearchMove(loadEngine(t), defs, pos, tc.depth, 6_000_000_000)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("move=%q score=%d cycles=%d", res.Move, res.Score, res.Cycles)
			if tc.noMoves {
				if res.Move != "" {
					t.Errorf("got move %q, want none", res.Move)
				}
			} else if res.Move == "" {
				t.Errorf("engine found no move")
			} else if tc.want != "" && !moveMatches(res.Move, tc.want) {
				t.Errorf("move = %q, want %q", res.Move, tc.want)
			}
			if res.Score != tc.score {
				t.Errorf("score = %d, want %d", res.Score, tc.score)
			}
		})
	}
}

func TestSanity(t *testing.T) {
	// Startpos at depth 2: any legal move, small score.
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	res, err := SearchMove(loadEngine(t), defs, pos, 2, 6_000_000_000)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("startpos d2: move=%q score=%d cycles=%d", res.Move, res.Score, res.Cycles)
	if res.Move == "" || res.Score < -200 || res.Score > 200 {
		t.Errorf("implausible startpos result: %q %d", res.Move, res.Score)
	}

	// Queen-up position: white should be clearly better.
	pos, err = ParseFEN("k7/8/8/8/8/8/1Q6/K7 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	res, err = SearchMove(loadEngine(t), defs, pos, 2, 6_000_000_000)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("queen-up d2: move=%q score=%d", res.Move, res.Score)
	if res.Score < 700 {
		t.Errorf("queen-up score = %d, want >= 700", res.Score)
	}
}
