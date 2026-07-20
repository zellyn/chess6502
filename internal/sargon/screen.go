package sargon

import "strings"

// textRowBase returns the base address in text page 1 ($400-$7FF) for the
// given screen row (0-23), using the Apple II's interleaved layout:
//
//	base = 0x400 + (row%8)*0x80 + (row/8)*0x28
func textRowBase(row int) uint16 {
	return 0x400 + uint16(row%8)*0x80 + uint16(row/8)*0x28
}

// decodeChar converts an Apple II text screen byte to a printable ASCII
// character. Screen bytes carry inverse/flash/normal in the high two bits; the
// low 6 bits select the glyph. There is no lowercase on a ][+.
func decodeChar(b byte) byte {
	low := b & 0x3F
	if low < 0x20 {
		return low + 0x40 // @, A-Z, [ \ ] ^ _
	}
	return low // space, punctuation, digits, symbols
}

// TextRow scrapes one 40-column row of text page 1 as an ASCII string.
func (m *Machine) TextRow(row int) string {
	base := textRowBase(row)
	var sb strings.Builder
	for col := 0; col < 40; col++ {
		sb.WriteByte(decodeChar(m.A2.RamRead(base + uint16(col))))
	}
	return sb.String()
}

// Screen returns all 24 rows of text page 1 joined by newlines.
func (m *Machine) Screen() string {
	rows := make([]string, 24)
	for r := 0; r < 24; r++ {
		rows[r] = m.TextRow(r)
	}
	return strings.Join(rows, "\n")
}

// ScreenContains reports whether any text row contains substr.
func (m *Machine) ScreenContains(substr string) bool {
	for r := 0; r < 24; r++ {
		if strings.Contains(m.TextRow(r), substr) {
			return true
		}
	}
	return false
}
