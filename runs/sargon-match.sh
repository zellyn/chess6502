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
exec "$OUT/sargon-xb" -dsk "$REPO/assets/sargon-iii.dsk" -budget-cycles ${BUDGET} -debug 2>>"$OUT/sargon-debug.log"
EOF
chmod +x "$OUT/run-us.sh" "$OUT/run-sargon.sh"

# Watchdog: a repetition draw can deadlock cutechess — Sargon declares the
# 3-fold one ply before cutechess counts it, so the result claim is rejected and
# both engines sit idle forever. If BOTH engine processes stay ~idle for a while,
# kill the Sargon process so cutechess ends that game (disconnect) and moves on.
# Such games are really draws; they are logged so they can be discounted.
watchdog() {
  local idle=0
  while true; do
    sleep 15
    pgrep -f "$REPO/tools/cutechess-cli" >/dev/null 2>&1 || return
    local uc sc
    uc=$(ps -o %cpu= -p "$(pgrep -f "$OUT/us" | head -1)" 2>/dev/null | tr -d ' ')
    sc=$(ps -o %cpu= -p "$(pgrep -f "$OUT/sargon-xb" | head -1)" 2>/dev/null | tr -d ' ')
    if [[ "${uc%%.*}" == "0" && "${sc%%.*}" == "0" ]]; then
      idle=$((idle+1))
    else
      idle=0
    fi
    if (( idle >= 3 )); then
      echo "WATCHDOG: killing stalled sargon-xb (likely a repetition-draw deadlock)"
      pkill -f "$OUT/sargon-xb"
      idle=0
    fi
  done
}
watchdog &
WATCHDOG_PID=$!

echo "starting cutechess at $(date)"
# restart=on for Sargon: cutechess starts a fresh sargon-xb process each game,
# so Sargon boots clean every time (an in-process reboot corrupts the emulator).
# Both engines report scores (our UCI engine; Sargon a material eval via the
# adapter), so cutechess adjudicates: -resign ends decided games fast (Sargon is
# usually up material) and -draw ends dead-equal shuffles, avoiding long games
# and Sargon's own draw-claim (which can hang cutechess on a 3-fold off-by-one).
"$REPO/tools/cutechess-cli" \
  -engine name=us cmd="$OUT/run-us.sh" proto=uci \
  -engine name=SargonIII cmd="$OUT/run-sargon.sh" proto=xboard restart=on \
  -each tc=inf -rounds "$ROUNDS" -games 2 -maxmoves 160 -ratinginterval 1 \
  -resign movecount=5 score=900 -draw movenumber=40 movecount=10 score=20 \
  -pgnout "$OUT/sargon-match.pgn" || true

kill "$WATCHDOG_PID" 2>/dev/null
echo "cutechess exited at $(date)"
echo "SARGON-MATCH-DONE"
