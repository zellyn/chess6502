// Command sargon-xboard exposes headless Sargon III as an xboard (CECP) engine
// so cutechess-cli can pit it against another engine (e.g. our own).
//
// It plays the side that moves *second*: the opponent's moves arrive as
// "usermove" and are typed into Sargon as the keyboard player's move; Sargon's
// reply is emitted as the engine's move. Configure cutechess with this engine
// as Black (Sargon's default: the keyboard player is White).
//
// Usage (as a cutechess engine):
//
//	cutechess-cli \
//	  -engine name=us cmd="go run ./cmd/uci" proto=uci \
//	  -engine name=SargonIII cmd="go run ./cmd/sargon-xboard -level 3" proto=xboard \
//	  -each tc=inf -games 1 -pgnout smoke.pgn
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/zellyn/chess6502/internal/sargon"
)

func main() {
	dsk := flag.String("dsk", "assets/sargon-iii.dsk", "path to Sargon III .dsk image")
	level := flag.Int("level", 1, "Sargon level 1-9 (used only when -budget-cycles=0)")
	easy := flag.Bool("easy", true, "enable Easy Mode (ponder-free; reliable) in level mode")
	budgetCycles := flag.Uint64("budget-cycles", 0, "fair mode: Infinite level + CTRL-T after this many 6502 cycles/move (0 = use -level)")
	wait := flag.Uint64("wait", 40_000_000, "level mode: max CPU steps to wait for a Sargon reply")
	debug := flag.Bool("debug", false, "log protocol traffic to stderr")
	flag.Parse()

	e := &engine{
		dsk: *dsk, level: *level, easy: *easy, budgetCycles: *budgetCycles,
		wait: *wait, debug: *debug,
	}
	e.run()
}

type engine struct {
	dsk          string
	level        int
	easy         bool
	budgetCycles uint64
	wait         uint64
	debug        bool

	m       *sargon.Machine
	out     *bufio.Writer
	mu      sync.Mutex // guards out (main loop + think goroutine both write)
	booted  chan struct{}
	bootErr error

	// force/go handshake state (guarded by smu). In force mode Sargon's reply
	// is computed but held until "go".
	smu          sync.Mutex
	force        bool
	pendingReply string
	haveReply    bool
}

func (e *engine) run() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	e.out = bufio.NewWriter(os.Stdout)
	defer e.out.Flush()

	// Boot Sargon in the background (~2-3s) so the protocol loop stays
	// responsive to ping and the handshake; usermoves wait for it.
	e.startBoot()

	for in.Scan() {
		line := strings.TrimSpace(in.Text())
		if line == "" {
			continue
		}
		if e.debug {
			fmt.Fprintf(os.Stderr, "< %s\n", line)
		}
		if e.handle(line) {
			return
		}
	}
}

func (e *engine) send(format string, args ...any) {
	s := fmt.Sprintf(format, args...)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.debug {
		fmt.Fprintf(os.Stderr, "> %s\n", s)
	}
	fmt.Fprintln(e.out, s)
	e.out.Flush()
}

// handle processes one command; returns true to quit.
func (e *engine) handle(line string) bool {
	cmd := line
	arg := ""
	if i := strings.IndexByte(line, ' '); i >= 0 {
		cmd, arg = line[:i], strings.TrimSpace(line[i+1:])
	}

	switch cmd {
	case "xboard":
		// no-op
	case "protover":
		e.send(`feature myname="SargonIII" usermove=1 setboard=0 ping=1 sigint=0 sigterm=0 colors=0 done=1`)
	case "new":
		// Boot already started at process launch; only reboot if a game was
		// already played on this machine.
		if e.m != nil {
			e.startBoot()
		}
		e.smu.Lock()
		e.force, e.pendingReply, e.haveReply = false, "", false
		e.smu.Unlock()
	case "force":
		e.smu.Lock()
		e.force = true
		e.smu.Unlock()
	case "ping":
		e.send("pong %s", arg)
	case "usermove":
		e.doUserMove(arg)
	case "go":
		// Leave force mode and make Sargon move for the current side (Black).
		// If its reply to the last usermove is already computed, emit it now;
		// otherwise the in-flight think goroutine will emit it when done (it
		// sees force cleared). A "go" with no prior usermove (engine as White)
		// is unsupported.
		e.smu.Lock()
		e.force = false
		r := ""
		if e.haveReply {
			r, e.pendingReply, e.haveReply = e.pendingReply, "", false
		}
		e.smu.Unlock()
		if r != "" {
			e.send("move %s", r)
		}
	case "result", "?", "draw", "computer", "hard", "easy", "post", "nopost",
		"random", "level", "st", "sd", "time", "otim", "accepted", "rejected",
		"name", "rating", "ics", "cores", "memory", "setboard":
		// ignored / not applicable
	case "quit":
		return true
	default:
		// ignore unknown commands per CECP
	}
	return false
}

// startBoot boots a fresh machine in a goroutine and publishes it on e.booted.
func (e *engine) startBoot() {
	e.booted = make(chan struct{})
	e.m = nil
	e.bootErr = nil
	go func(done chan struct{}) {
		m, err := sargon.NewMachine(e.dsk)
		if err == nil {
			err = m.BootToPrompt()
		}
		if err == nil && e.budgetCycles > 0 {
			err = m.InfiniteLevel() // fair mode: CTRL-T at our cycle budget
		} else {
			if err == nil && e.level != 1 {
				err = m.Level(e.level)
			}
			if err == nil && e.easy {
				err = m.EasyMode()
			}
		}
		if err != nil {
			e.bootErr = err
		} else {
			e.m = m
		}
		close(done)
	}(e.booted)
}

// ensureBooted blocks until the background boot finishes.
func (e *engine) ensureBooted() error {
	<-e.booted
	return e.bootErr
}

// doUserMove processes an opponent move. The actual thinking runs in a
// goroutine so the I/O loop keeps answering ping (liveness) while Sargon boots
// and searches; otherwise cutechess declares a stalled connection.
func (e *engine) doUserMove(coord string) {
	go e.think(coord)
}

func (e *engine) think(coord string) {
	if err := e.ensureBooted(); err != nil {
		e.send("tellusererror boot failed: %v", err)
		log.Printf("boot failed: %v", err)
		return
	}
	sargonText, err := coordToSargon(coord)
	if err != nil {
		e.send("Illegal move: %s", coord)
		return
	}
	var res sargon.MoveResult
	if e.budgetCycles > 0 {
		res, err = e.m.RequestMove(sargonText, e.budgetCycles)
	} else {
		res, err = e.m.SubmitMove(sargonText, e.wait)
	}
	if err != nil {
		// Could be our move mating Sargon (GameOver handled below), else error.
		if res.GameOver {
			e.reportResult(res, true)
			return
		}
		e.send("tellusererror submit %q: %v", coord, err)
		return
	}
	// Prefer the RAM-decoded from/to (coordinate notation); castling/ep/promo
	// all come through as squares.
	mv := res.SargonMove
	reply := mv.From.Algebraic() + mv.To.Algebraic() + promoSuffix(res.SargonText)

	// Emit now unless we're in force mode, in which case hold the reply until
	// "go" arrives (which will emit it, or clear force so we emit here).
	e.smu.Lock()
	if e.force {
		e.pendingReply, e.haveReply = reply, true
		e.smu.Unlock()
		return
	}
	e.smu.Unlock()

	e.send("move %s", reply)
	if res.GameOver {
		e.reportResult(res, false)
	}
}

// reportResult prints an xboard game-result line. ourMoveMated=true means the
// opponent's (user's) move ended the game; otherwise Sargon's move did.
func (e *engine) reportResult(res sargon.MoveResult, ourMoveMated bool) {
	switch {
	case strings.Contains(res.Message, "STALEMATE"), strings.Contains(res.Message, "DRAW"):
		e.send("1/2-1/2 {%s}", res.Message)
	case strings.Contains(res.Message, "MATE"):
		if ourMoveMated {
			e.send("1-0 {White mates}") // user (White) mated Sargon
		} else {
			e.send("0-1 {Black mates}") // Sargon (Black) mated the user
		}
	default:
		e.send("1/2-1/2 {game over: %s}", res.Message)
	}
}

// coordToSargon converts xboard coordinate notation ("e2e4", "e7e8q",
// "e1g1") to Sargon's "FROM-TO" text (with an under-promotion suffix).
func coordToSargon(coord string) (string, error) {
	coord = strings.ToLower(strings.TrimSpace(coord))
	if len(coord) < 4 {
		return "", fmt.Errorf("short move %q", coord)
	}
	from := strings.ToUpper(coord[0:2])
	to := strings.ToUpper(coord[2:4])
	if _, ok := sargon.ParseSquare(from); !ok {
		return "", fmt.Errorf("bad from %q", coord)
	}
	if _, ok := sargon.ParseSquare(to); !ok {
		return "", fmt.Errorf("bad to %q", coord)
	}
	move := from + "-" + to
	// Promotion: Sargon promotes to queen on plain Return; append a letter for
	// under-promotion (N/R/B). 'q' needs no suffix.
	if len(coord) >= 5 {
		switch coord[4] {
		case 'n', 'r', 'b':
			move += strings.ToUpper(string(coord[4]))
		case 'q':
			// default
		}
	}
	return move, nil
}

// promoSuffix extracts a promotion letter from Sargon's move text (e.g.
// "H7-H8/Q") and returns it lowercased for coordinate notation, or "".
func promoSuffix(text string) string {
	if i := strings.IndexByte(text, '/'); i >= 0 && i+1 < len(text) {
		return strings.ToLower(string(text[i+1]))
	}
	return ""
}
