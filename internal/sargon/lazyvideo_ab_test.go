package sargon

import (
	"crypto/sha1"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestLazyVideoAB verifies the lazy floating-bus video scan is behavior-
// identical to the per-cycle scan (same RAM + screen after boot and a scripted
// game), and reports the wall-clock speedup.
func TestLazyVideoAB(t *testing.T) {
	if os.Getenv("SARGON_SLOW") == "" {
		t.Skip("set SARGON_SLOW=1")
	}
	run := func(lazy bool) (ramHash, scrHash string, boot time.Duration, steps uint64) {
		m, err := newMachine(dskPath, lazy)
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		if err := m.BootToPrompt(); err != nil {
			t.Fatal(err)
		}
		// A short deterministic interaction: Easy Mode + a couple of moves.
		if err := m.EasyMode(); err != nil {
			t.Fatal(err)
		}
		if _, err := m.SubmitMove("E2-E4", 15_000_000); err != nil {
			t.Fatalf("lazy=%v move1: %v", lazy, err)
		}
		if _, err := m.SubmitMove("D2-D4", 15_000_000); err != nil {
			t.Fatalf("lazy=%v move2: %v", lazy, err)
		}
		boot = time.Since(start)
		ram := m.PeekSlice(0x0000, 0xC000)
		ramHash = fmt.Sprintf("%x", sha1.Sum(ram))
		scrHash = fmt.Sprintf("%x", sha1.Sum([]byte(m.Screen())))
		return ramHash, scrHash, boot, m.Steps
	}

	lr, ls, lt, lsteps := run(true)
	nr, ns, nt, nsteps := run(false)

	fmt.Printf("lazy:    ram=%s screen=%s  %v (%d steps)\n", lr[:12], ls[:12], lt, lsteps)
	fmt.Printf("nonlazy: ram=%s screen=%s  %v (%d steps)\n", nr[:12], ns[:12], nt, nsteps)
	fmt.Printf("speedup: %.2fx (nonlazy %v / lazy %v)\n", float64(nt)/float64(lt), nt, lt)

	if lsteps != nsteps {
		t.Errorf("step counts differ: lazy=%d nonlazy=%d (execution diverged)", lsteps, nsteps)
	}
	if lr != nr {
		t.Errorf("RAM differs between lazy and non-lazy video scan")
	}
	if ls != ns {
		t.Errorf("screen differs between lazy and non-lazy video scan")
	}
	if lr == nr && ls == ns && lsteps == nsteps {
		fmt.Println("IDENTICAL play confirmed (RAM + screen + step count match)")
	}
}
