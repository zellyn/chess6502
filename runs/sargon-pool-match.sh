#!/bin/zsh
# VARIED-OPENING benchmark gauntlet: our engine vs Sargon III, starting each
# game from an unbalanced opening in tools/openings-pool.epd (kills the ~70%
# draw rate of standard-start games, where opening-book symmetry dominates).
#
# Each opening is played twice with reversed colors (-repeat -games 2), so
# Sargon and our engine each play both sides of every position. cutechess sends
# "setboard <fen>" to both engines; the adapter applies it via the validated
# CTRL-A editor SetupPosition (see docs/sargon.md).
#
# SARGON PONDERING HANDICAP: Easy Mode (required for reliable headless reads)
# disables Sargon's pondering, which weakens it vs the real machine. To
# approximate the lost pondering, give Sargon MULT x our per-move cycle budget
# (-budget-multiplier). Bracket the truth by running MULT=1.5 and MULT=2.0.
#
# Detached/logged batch: run with tmux (NOT nohup), e.g.
#   tmux new -s sargonpool 'runs/sargon-pool-match.sh 20 30000000 2.0 /tmp/pool2x'
# then poll the log for the SARGON-POOL-DONE marker.
#
# Args: [ROUNDS] [BUDGET_CYCLES] [SARGON_MULT] [OUTDIR]
#   defaults: 20 30000000 2.0 /tmp/sargon-pool
set -e
cd "$(dirname "$0")/.."
REPO="$PWD"
ROUNDS="${1:-20}"          # distinct openings used (<=40); x2 games each
BUDGET="${2:-30000000}"    # OUR Sargon-matched cycles/move
MULT="${3:-2.0}"           # Sargon gets MULT x BUDGET cycles (pondering proxy)
OUT="${4:-/tmp/sargon-pool}"
mkdir -p "$OUT"
US_BUDGET_MS=$(( BUDGET / 1020 ))   # our engine's matched emulated-ms budget

echo "=== sargon-pool: $((ROUNDS*2)) games from openings-pool.epd ==="
echo "    our budget=${BUDGET}cyc (~$((BUDGET/1020500))s / ${US_BUDGET_MS}ms), Sargon MULT=${MULT}"
echo "building binaries..."
go build -o "$OUT/us" ./cmd/uci
go build -o "$OUT/sargon-xb" ./cmd/sargon-xboard
cp "$REPO/asm/engine.bin" "$OUT/engine.bin"

cat > "$OUT/run-us.sh" <<EOF
#!/bin/zsh
exec "$OUT/us" -bin "$OUT/engine.bin" -defs "$REPO/asm/defs.inc" -budget ${US_BUDGET_MS}
EOF
cat > "$OUT/run-sargon.sh" <<EOF
#!/bin/zsh
exec "$OUT/sargon-xb" -dsk "$REPO/assets/sargon-iii.dsk" -budget-cycles ${BUDGET} -budget-multiplier ${MULT} -debug 2>>"$OUT/sargon-debug.log"
EOF
chmod +x "$OUT/run-us.sh" "$OUT/run-sargon.sh"

# No -dither here: opening variety comes from the pool, so our engine plays its
# best line (deterministic, stronger) rather than randomizing.
# SARGON-DECLARED-DRAW games (adapter resigns to end a repetition/50-move draw
# that a "1/2-1/2" claim would deadlock) are logged in sargon-debug.log and are
# really draws — reclassify when tallying (grep the debug log).
echo "starting cutechess at $(date)"
"$REPO/tools/cutechess-cli" \
  -engine name=us cmd="$OUT/run-us.sh" proto=uci \
  -engine name=SargonIII cmd="$OUT/run-sargon.sh" proto=xboard restart=on \
  -openings file="$REPO/tools/openings-pool.epd" format=epd order=sequential \
  -repeat \
  -each tc=inf -rounds "$ROUNDS" -games 2 -maxmoves 160 -ratinginterval 1 \
  -resign movecount=5 score=900 -draw movenumber=40 movecount=10 score=20 \
  -pgnout "$OUT/sargon-pool.pgn" || true

echo "cutechess exited at $(date)"
echo "SARGON-POOL-DONE"
