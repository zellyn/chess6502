// Package sargon provides a headless Apple II harness for driving Sargon III
// (1983) as a programmatically-controllable chess opponent.
//
// It boots the sargon-iii.dsk disk image inside the goapple2 emulator with no
// GUI, injects moves via the keyboard, and scrapes the 40x24 text screen and
// raw RAM so that an external driver can read Sargon's board and moves.
package sargon

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/zellyn/goapple2"
	"github.com/zellyn/goapple2/cards"
	"github.com/zellyn/goapple2/disk"
	"github.com/zellyn/goapple2/util"
	"github.com/zellyn/goapple2/videoscan"
)

// DefaultRomDir is where the Apple II ROMs live in the sibling goapple2
// checkout. Override with the GOAPPLE2_ROMS environment variable.
const DefaultRomDir = "/Users/zellyn/gh/goapple2/data/roms"

// nullPlotter satisfies videoscan.Plotter with no-op rendering (headless).
type nullPlotter struct{}

func (nullPlotter) Plot(videoscan.PlotData) {}
func (nullPlotter) OncePerFrame()           {}

// Machine wraps a headless goapple2.Apple2 booting a single floppy in slot 6.
type Machine struct {
	A2    *goapple2.Apple2
	Steps uint64 // total CPU steps executed via Run/RunUntil
}

func romDir() string {
	if d := os.Getenv("GOAPPLE2_ROMS"); d != "" {
		return d
	}
	return DefaultRomDir
}

// NewMachine constructs a headless Apple ][+ (48K + language card) with a Disk
// II controller in slot 6 and the given .dsk mounted in drive 1. It does not
// step the CPU; call Run/RunUntil to advance.
func NewMachine(dskPath string) (*Machine, error) {
	dir := romDir()
	romPath := filepath.Join(dir, "apple2+.rom")
	charPath := filepath.Join(dir, "apple2-chars.rom")
	diskRomPath := filepath.Join(dir, "Apple Disk II 16 Sector Interface Card ROM P5 - 341-0027.bin")

	rom := util.ReadRomOrDie(romPath, 12288)
	charRom := util.ReadSmallCharacterRomOrDie(charPath)
	diskRom := util.ReadRomOrDie(diskRomPath, 256)

	a2 := goapple2.NewApple2(nullPlotter{}, rom, charRom)

	// Language card in slot 0 (matches the GUI setup; harmless for a ][+ 48K
	// program and required by anything that banks $D000).
	lc, err := cards.NewLanguageCard(rom, "Language Card", 0, a2)
	if err != nil {
		return nil, fmt.Errorf("language card: %w", err)
	}
	if err := a2.AddCard(lc); err != nil {
		return nil, fmt.Errorf("add language card: %w", err)
	}

	diskCard, err := cards.NewDiskCard(diskRom, 6, a2)
	if err != nil {
		return nil, fmt.Errorf("disk card: %w", err)
	}
	if err := a2.AddCard(diskCard); err != nil {
		return nil, fmt.Errorf("add disk card: %w", err)
	}

	d, err := disk.DiskFromFile(dskPath, 0)
	if err != nil {
		return nil, fmt.Errorf("load disk %q: %w", dskPath, err)
	}
	diskCard.LoadDisk(d, 0)

	return &Machine{A2: a2}, nil
}

// Run steps the CPU n times.
func (m *Machine) Run(n uint64) error {
	for i := uint64(0); i < n; i++ {
		if err := m.A2.Step(); err != nil {
			return err
		}
		m.Steps++
	}
	return nil
}

// RunUntil steps the CPU until pred() returns true or maxSteps have been
// executed. It reports whether pred was satisfied and how many steps were run.
func (m *Machine) RunUntil(maxSteps uint64, pred func(*Machine) bool) (ok bool, err error) {
	for i := uint64(0); i < maxSteps; i++ {
		if err := m.A2.Step(); err != nil {
			return false, err
		}
		m.Steps++
		if pred(m) {
			return true, nil
		}
	}
	return false, nil
}

// Peek reads a byte of Apple II main RAM ($0000-$BFFF).
func (m *Machine) Peek(addr uint16) byte {
	return m.A2.RamRead(addr)
}

// PeekSlice returns a copy of main RAM in [start, start+n).
func (m *Machine) PeekSlice(start uint16, n int) []byte {
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = m.A2.RamRead(start + uint16(i))
	}
	return out
}

// Type injects an ASCII string via the keyboard, one key at a time. Newlines
// are sent as carriage returns (Apple II Return = $0D).
func (m *Machine) Type(s string) {
	for _, r := range s {
		if r == '\n' {
			r = '\r'
		}
		m.A2.Keypress(byte(r))
	}
}

// Key injects a single key code (already the low 7 bits; the high bit is set by
// Keypress). Useful for control characters like ESC ($1B) or ctrl-A ($01).
func (m *Machine) Key(code byte) {
	m.A2.Keypress(code)
}

// TypePaced injects a string one key at a time, stepping the CPU stepsPerKey
// cycles after each so Sargon's input routine reads and echoes each character
// before the next arrives. Injecting a whole string at once races Sargon's
// keyboard poll and drops characters. Newlines become carriage returns.
func (m *Machine) TypePaced(s string, stepsPerKey uint64) error {
	for _, r := range s {
		if r == '\n' {
			r = '\r'
		}
		m.A2.Keypress(byte(r))
		if err := m.Run(stepsPerKey); err != nil {
			return err
		}
	}
	return nil
}
