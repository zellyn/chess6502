package refchess

// Perft counts leaf nodes of the legal-move tree rooted at p, to the
// given depth (the standard chess move-generation correctness test —
// see chessprogramming.org/Perft). depth<=0 counts the position itself
// as one node. At depth==1 it returns len(LegalMoves()) directly rather
// than recursing one more level and unmaking, a cheap "bulk counting"
// win since the deepest ply is the majority of the work.
func (p *Position) Perft(depth int) uint64 {
	if depth <= 0 {
		return 1
	}
	moves := p.LegalMoves()
	if depth == 1 {
		return uint64(len(moves))
	}
	var nodes uint64
	for _, m := range moves {
		cp := p.Copy()
		cp.applyMove(m)
		nodes += cp.Perft(depth - 1)
	}
	return nodes
}
