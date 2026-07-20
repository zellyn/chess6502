#!/bin/zsh
# Matched benchmark gauntlet: our engine vs Sargon III, BOTH at ~30M 6502
# cycles/move (our standing ~30s budget), via RequestMove/CTRL-T on Sargon's
# Infinite level. Color-paired (cutechess alternates colors each game; Sargon
# plays White via the adapter's CTRL-S path). Detached/logged batch (survives
# the launcher): run with `nohup runs/sargon-match.sh > LOG 2>&1 &`, then poll
# LOG for the SARGON-MATCH-DONE marker.
#
# NOTE: games start from the standard position (opening variety via our engine's
# -dither), NOT tools/openings-pool.epd: Sargon reconstructs its board from its
# move list, so setboard/position-poke does not take (see docs/sargon.md). The
# opening pool is a follow-up pending Sargon master-board RE.
#
# Args: [ROUNDS] [BUDGET_CYCLES] [OUTDIR]   (defaults: 20 30000000 scratch)
set -e
cd "$(dirname "$0")/.."
REPO="$PWD"
ROUNDS="${1:-20}"          # rounds x 2 games = total games (color-paired)
BUDGET="${2:-30000000}"    # Sargon cycles/move (CTRL-T budget)
OUT="${3:-/tmp/sargon-match}"
mkdir -p "$OUT"
US_BUDGET_MS=$(( BUDGET / 1020 ))   # our engine's matched emulated-ms budget

echo "=== sargon-match: $((ROUNDS*2)) games, ${BUDGET} cyc/move (~$((BUDGET/1020500))s), our -budget ${US_BUDGET_MS}ms ==="
echo "building binaries..."
go build -o "$OUT/us" ./cmd/uci
go build -o "$OUT/sargon-xb" ./cmd/sargon-xboard
cp "$REPO/asm/engine.bin" "$OUT/engine.bin"

cat > "$OUT/run-us.sh" <<EOF
#!/bin/zsh
exec "$OUT/us" -bin "$OUT/engine.bin" -defs "$REPO/asm/defs.inc" -budget ${US_BUDGET_MS} -dither
EOF
cat > "$OUT/run-sargon.sh" <<EOF
#!/bin/zsh
exec "$OUT/sargon-xb" -dsk "$REPO/assets/sargon-iii.dsk" -budget-cycles ${BUDGET}
EOF
chmod +x "$OUT/run-us.sh" "$OUT/run-sargon.sh"

echo "starting cutechess at $(date)"
"$REPO/tools/cutechess-cli" \
  -engine name=us cmd="$OUT/run-us.sh" proto=uci \
  -engine name=SargonIII cmd="$OUT/run-sargon.sh" proto=xboard \
  -each tc=inf -rounds "$ROUNDS" -games 2 -maxmoves 160 -ratinginterval 1 \
  -pgnout "$OUT/sargon-match.pgn" || true

echo "cutechess exited at $(date)"
echo "SARGON-MATCH-DONE"
