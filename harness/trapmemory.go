package harness

import (
	"io"

	"github.com/zellyn/goapple2/iie"
)

// TrapMemory is an iie.Memory with the harness's I/O traps layered on top:
// a main-bank store to CoutAddr emits the byte to Cout, and a main-bank
// store to ExitAddr ends the run with the stored byte as the exit code.
//
// Traps model main-bank RAM locations: writes made with RAMWRT on (i.e. to
// the aux bank) are ordinary memory writes and never fire a trap. Peek is
// promoted from the embedded *iie.Memory unchanged, so memory dumps remain
// side-effect free.
type TrapMemory struct {
	*iie.Memory
	CoutAddr uint16
	ExitAddr uint16
	Cout     io.Writer

	exited   bool
	exitCode byte
}

// Write implements cpu.Memory, applying the harness's store traps before
// delegating to the underlying iie.Memory.
func (t *TrapMemory) Write(addr uint16, val byte) {
	// Traps model main-bank RAM locations: ignore aux-bank stores.
	if !t.Memory.RamWrt {
		switch addr {
		case t.CoutAddr:
			if t.Cout != nil {
				t.Cout.Write([]byte{val})
			}
		case t.ExitAddr:
			t.exited = true
			t.exitCode = val
		}
	}
	t.Memory.Write(addr, val)
}

// Exited reports whether the exit trap has fired.
func (t *TrapMemory) Exited() bool { return t.exited }

// ExitCode returns the byte most recently stored to ExitAddr. Meaningful
// only once Exited reports true.
func (t *TrapMemory) ExitCode() byte { return t.exitCode }
