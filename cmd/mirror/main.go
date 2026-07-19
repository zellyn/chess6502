// Command mirror drives the Go mirror engine (internal/mirror) for
// fast, node-denominated experiments: fixed-depth node counts, Texel
// tuning of the pawn-structure weights, and self-play validation
// matches. See docs/plan.md task #20.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/zellyn/chess6502/internal/mirror"
	"github.com/zellyn/chess6502/internal/sprt"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "nodes":
		nodes(os.Args[2:])
	case "gen":
		gen(os.Args[2:])
	case "tune":
		tune(os.Args[2:])
	case "match":
		match(os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: mirror <command> [flags]
  nodes   fixed-depth node counts by feature mask
  gen     self-play a chunk of games, appending labeled rows to -data
  tune    Texel-tune pstruct weights from a -data file
  match   self-play match between weight/feature configurations`)
	os.Exit(2)
}

func nodes(args []string) {
	fs := flag.NewFlagSet("nodes", flag.ExitOnError)
	fen := fs.String("fen", "r1b1k2r/ppp2ppp/2nqpn2/3p4/3P4/P1P1BN2/2P1PPPP/2RQKB1R w Kkq - 2 8", "position")
	depth := fs.Int("depth", 6, "fixed depth")
	fs.Parse(args)
	for _, mask := range []byte{0x00, 0x01, 0x02, 0x04, 0x07, 0x0F} {
		pos, err := mirror.ParseFEN(*fen)
		check(err)
		eng := mirror.NewEngine()
		eng.Features = mask
		eng.SetPosition(pos)
		start := time.Now()
		best, score := eng.SearchFixed(*depth)
		fmt.Printf("features %#02x: %10d nodes, depth %d, best %s score %d (%v)\n",
			mask, eng.Nodes, *depth, best.UCI(), score, time.Since(start).Round(time.Millisecond))
	}
}

func gen(args []string) {
	fs := flag.NewFlagSet("gen", flag.ExitOnError)
	depth := fs.Int("depth", 5, "self-play depth")
	games := fs.Int("games", 300, "games this chunk")
	workers := fs.Int("workers", runtime.NumCPU()-2, "parallel games")
	seed := fs.Uint64("seed", 6502, "RNG seed (vary per chunk)")
	openings := fs.Int("openings", 300, "generated opening lines")
	data := fs.String("data", "texel.rows", "row file to append to")
	fs.Parse(args)

	lines, err := mirror.GenOpenings(sprt.Openings, *openings, *seed)
	check(err)
	start := time.Now()
	samples, err := mirror.GenerateData(lines, mirror.DefaultWeights, *depth,
		*games, *workers, *seed, func(games, n int) {
			fmt.Printf("  %d games, %d samples (%v)\n", games, n, time.Since(start).Round(time.Second))
		})
	check(err)
	check(mirror.AppendRows(*data, mirror.SampleRows(samples)))
	fmt.Printf("chunk done: %d games, %d samples appended to %s in %v\n",
		*games, len(samples), *data, time.Since(start).Round(time.Second))
}

func tune(args []string) {
	fs := flag.NewFlagSet("tune", flag.ExitOnError)
	data := fs.String("data", "texel.rows", "row file from gen")
	fs.Parse(args)

	rows, err := mirror.LoadRows(*data)
	check(err)
	fmt.Printf("loaded %d rows from %s\n", len(rows), *data)

	k := mirror.FitK(rows, mirror.DefaultWeights)
	fmt.Printf("fitted sigmoid K = %.2f\n", k)

	tuned, before, after := mirror.Tune(rows, mirror.DefaultWeights, k,
		func(step string, loss float64) { fmt.Printf("  %s: loss %.6f\n", step, loss) })
	fmt.Printf("loss: %.6f -> %.6f\n", before, after)
	fmt.Printf("tuned weights: %+v\n", tuned)
	fmt.Printf("asm PASSEDBONUS: .byte %d, %d, %d, %d, %d, %d, %d, %d\n",
		tuned.Passed[0], tuned.Passed[1], tuned.Passed[2], tuned.Passed[3],
		tuned.Passed[4], tuned.Passed[5], tuned.Passed[6], tuned.Passed[7])
}

func match(args []string) {
	fs := flag.NewFlagSet("match", flag.ExitOnError)
	depth := fs.Int("depth", 5, "fixed depth")
	pairs := fs.Int("pairs", 150, "game pairs (2 games each)")
	workers := fs.Int("workers", runtime.NumCPU()-2, "parallel pairs")
	seed := fs.Uint64("seed", 6502, "RNG seed")
	aMask := fs.Uint("afeat", 0x0F, "A feature mask")
	bMask := fs.Uint("bfeat", 0x0F, "B feature mask")
	aw := fs.String("aweights", "default", "A pstruct weights: default|tuned|w:d,i,p1..p6,s,o")
	bw := fs.String("bweights", "default", "B pstruct weights")
	fs.Parse(args)

	lines, err := mirror.GenOpenings(sprt.Openings, *pairs, *seed)
	check(err)
	a := mirror.PlayerCfg{Features: byte(*aMask), Weights: parseWeights(*aw), Depth: *depth}
	b := mirror.PlayerCfg{Features: byte(*bMask), Weights: parseWeights(*bw), Depth: *depth}
	start := time.Now()
	res, err := mirror.Match(a, b, lines, *pairs, *workers, *seed)
	check(err)
	fmt.Printf("A(%#02x %s) vs B(%#02x %s) depth %d: %s (%v)\n",
		byte(*aMask), *aw, byte(*bMask), *bw, *depth, res, time.Since(start).Round(time.Second))
}

func parseWeights(s string) mirror.Weights {
	switch s {
	case "default":
		return mirror.DefaultWeights
	case "tuned":
		return mirror.TunedWeights
	}
	var w mirror.Weights
	n, err := fmt.Sscanf(s, "w:%d,%d,%d,%d,%d,%d,%d,%d,%d,%d",
		&w.Doubled, &w.Isolated, &w.Passed[1], &w.Passed[2], &w.Passed[3],
		&w.Passed[4], &w.Passed[5], &w.Passed[6], &w.Shield, &w.OpenFile)
	if err != nil || n != 10 {
		fmt.Fprintf(os.Stderr, "bad weights %q (want w:d,i,p1,p2,p3,p4,p5,p6,s,o)\n", s)
		os.Exit(2)
	}
	return w
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
