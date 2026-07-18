package mirror

// Perft counts leaf nodes at the given depth (legal-move tree), for
// validation against the asm engine's perft and published values.
func Perft(pos *Position, depth int) uint64 {
	e := NewEngine()
	e.Features = 0 // no pstruct recomputation churn
	e.SetPosition(pos)
	return e.perft(depth)
}

func (e *Engine) perft(depth int) uint64 {
	if depth == 0 {
		return 1
	}
	var count uint64
	// Each recursion level generates into its own per-ply slice, so
	// iterating while recursing is safe.
	for _, m := range e.generate(false) {
		e.make(m)
		king := e.Pos.PieceSq[int(e.Pos.Side^ColorMask)<<1]
		if !e.attacked(king, e.Pos.Side) {
			count += e.perft(depth - 1)
		}
		e.unmake()
	}
	return count
}
