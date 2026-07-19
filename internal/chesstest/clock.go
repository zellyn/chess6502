package chesstest

// BankedClock implements chess-clock time management over per-move
// emulated-cycle budgets: unused cycles carry forward in a bank, and
// each move's spendable allocation is Base + bank/8, so the bank
// drains smoothly across several hard positions instead of feeding
// one greedy move. Total game time is conserved (Settle adds exactly
// Base - spent, clamped), so protocol (c) match budgets stay
// comparable. This is the reference for the eventual 6502 driver port
// (a 24-bit bank in zp; /8 is three shift chains).
type BankedClock struct {
	Base uint64 // per-move base allocation, cycles
	Cap  uint64 // bank ceiling; 0 = 8*Base
	bank uint64
}

// Alloc returns the next move's spendable budget.
func (c *BankedClock) Alloc() uint64 {
	return c.Base + c.bank/8
}

// Settle accounts a finished move: the bank gains Base and loses what
// was actually spent (the engine's hard-abort ceiling can overspend
// the allocation; the bank just clamps at zero).
func (c *BankedClock) Settle(spent uint64) {
	total := c.bank + c.Base
	if spent >= total {
		c.bank = 0
	} else {
		c.bank = total - spent
	}
	max := c.Cap
	if max == 0 {
		max = 8 * c.Base
	}
	if c.bank > max {
		c.bank = max
	}
}

// Bank exposes the current balance (diagnostics).
func (c *BankedClock) Bank() uint64 { return c.bank }
