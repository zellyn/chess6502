package refchess

import "math/rand"

// RandomMove picks a uniformly random legal move, for driving random-mover
// test matches. It returns ok=false if there are no legal moves
// (checkmate or stalemate).
func RandomMove(p *Position, rnd *rand.Rand) (Move, bool) {
	moves := p.LegalMoves()
	if len(moves) == 0 {
		return Move{}, false
	}
	return moves[rnd.Intn(len(moves))], true
}
