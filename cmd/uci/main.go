// Command uci presents the emulated 6502 chess engine as a UCI engine
// (for cutechess-cli, lichess-bot, and friends). See internal/ucibridge.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/zellyn/chess6502/internal/chesstest"
	"github.com/zellyn/chess6502/internal/ucibridge"
)

func main() {
	var (
		binfile  = flag.String("bin", "asm/engine.bin", "engine binary")
		defsfile = flag.String("defs", "asm/defs.inc", "memory-layout defs")
		budget   = flag.Uint64("budget", 0, "fixed emulated ms per move (0: derive from go command)")
	)
	flag.Parse()

	bin, err := os.ReadFile(*binfile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defs, err := chesstest.ParseDefs(*defsfile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	b := &ucibridge.Bridge{Bin: bin, Defs: defs, FixedBudgetMs: *budget}
	if err := b.Run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
