package harness

import (
	"bytes"
	"testing"
	"time"
)

// loopBin is a tiny hand-assembled busy loop — no ca65 dependency in
// tests. It increments a zero-page counter forever:
//
//	$2000  A9 00        LDA #$00
//	$2002  85 10        STA $10
//	$2004  E6 10  loop:  INC $10
//	$2006  D0 FC        BNE loop
//	$2008  4C 04 20     JMP loop
//
// It never stores to a trap address, so it runs until Run's cycle budget
// cuts it off, making it a simple stand-in for measuring raw emulated
// instruction throughput.
var loopBin = []byte{
	0xA9, 0x00,
	0x85, 0x10,
	0xE6, 0x10,
	0xD0, 0xFC,
	0x4C, 0x04, 0x20,
}

const loopOrg = 0x2000

// coutBin stores 'O' to the COUT trap, then exits with code 0:
//
//	$2000  A9 4F        LDA #'O'
//	$2002  8D F0 BF     STA $BFF0
//	$2005  A9 00        LDA #$00
//	$2007  8D FF BF     STA $BFFF
//	$200A  00           BRK (unreachable)
var coutBin = []byte{
	0xA9, 0x4F,
	0x8D, 0xF0, 0xBF,
	0xA9, 0x00,
	0x8D, 0xFF, 0xBF,
	0x00,
}

// BenchmarkRun measures raw emulated-instruction throughput by running
// loopBin for a large, fixed cycle budget per b.N iteration and reporting
// emulated-MHz and its speedup relative to a real IIe's 1.0205 MHz clock.
func BenchmarkRun(b *testing.B) {
	const maxCycles = 50_000_000

	var totalCycles uint64
	start := time.Now()
	for i := 0; i < b.N; i++ {
		m, err := New(Config{
			Bin:      loopBin,
			Org:      loopOrg,
			Entry:    loopOrg,
			CoutAddr: 0xBFF0,
			ExitAddr: 0xBFFF,
		})
		if err != nil {
			b.Fatalf("New: %v", err)
		}
		exited, _, err := m.Run(maxCycles)
		if err != nil {
			b.Fatalf("Run: %v", err)
		}
		if exited {
			b.Fatalf("loop binary unexpectedly hit the exit trap")
		}
		totalCycles += m.Cycles
	}
	wall := time.Since(start)

	mhz := float64(totalCycles) / wall.Seconds() / 1e6
	b.ReportMetric(mhz, "emulated-MHz")
	b.ReportMetric(mhz*1e6/EffectiveHz, "x-realtime")
}

// TestCout runs coutBin and checks that the byte it stores to the COUT
// trap lands in Config.Cout, and that the machine reports the exit code it
// stored.
func TestCout(t *testing.T) {
	var out bytes.Buffer
	m, err := New(Config{
		Bin:      coutBin,
		Org:      loopOrg,
		Entry:    loopOrg,
		CoutAddr: 0xBFF0,
		ExitAddr: 0xBFFF,
		Cout:     &out,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	exited, exitCode, err := m.Run(1000)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !exited {
		t.Fatalf("Run did not exit within the cycle budget")
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if got := out.String(); got != "O" {
		t.Fatalf("Cout = %q, want %q", got, "O")
	}
}

// TestExitTrapIgnoredWithRamWrtOn is a bank-awareness regression test:
// stores to ExitAddr must only fire the exit trap when RamWrt is off (main
// bank). It flips RAMWRT by writing $C005/$C004 directly through Mem, the
// same soft switches a running program would hit.
func TestExitTrapIgnoredWithRamWrtOn(t *testing.T) {
	m, err := New(Config{
		Bin:      loopBin,
		Org:      loopOrg,
		Entry:    loopOrg,
		CoutAddr: 0xBFF0,
		ExitAddr: 0xBFFF,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	m.Mem.Write(0xC005, 0) // RAMWRT on: $0200-$BFFF writes go to aux
	m.Mem.Write(0xBFFF, 0x42)
	if m.Mem.Exited() {
		t.Fatalf("exit trap fired on an aux-bank store (RamWrt on); want ignored")
	}

	m.Mem.Write(0xC004, 0) // RAMWRT off: back to main bank
	m.Mem.Write(0xBFFF, 0x42)
	if !m.Mem.Exited() {
		t.Fatalf("exit trap did not fire on a main-bank store (RamWrt off)")
	}
	if got := m.Mem.ExitCode(); got != 0x42 {
		t.Fatalf("ExitCode() = %#02x, want 0x42", got)
	}
}
