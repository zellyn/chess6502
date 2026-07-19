#!/bin/bash
# Standing rating-pool gauntlet. Usage: pool.sh <engine.bin> [tag]
# Engine: 30000ms emulated budget, -dither -bank. Opponents fixed:
#   TSCP-d3 (st=2 depth=3), Fairy-Max (st=2), NEG (st=2), minnow (st=2),
#   SF-n10 / SF-n100 / SF-n1000 (Stockfish 18, Threads=1, node-limited).
# 30 games per opponent from the committed tools/openings-pool.epd
# (sequential, paired colors). PGNs: tools/pgn/pool_<tag>_<opp>.pgn.
set -x
BIN=${1:?usage: pool.sh <engine.bin> [tag]}
TAG=${2:-$(git -C /Users/zellyn/gh/chess6502 rev-parse --short HEAD)}
cd /Users/zellyn/gh/chess6502/tools
ENG="-engine name=chess6502 cmd=./bin/chess6502-uci arg=-bin arg=$BIN arg=-defs arg=/Users/zellyn/gh/chess6502/asm/defs.inc arg=-budget arg=30000 arg=-dither arg=-bank proto=uci st=10 timemargin=5000"
OPEN="-openings file=openings-pool.epd format=epd order=sequential"
run() { # name, engine-args...
  local name=$1; shift
  ./cutechess-cli $ENG -engine name=$name "$@" $OPEN -rounds 15 -games 2 -repeat \
    -pgnout pgn/pool_${TAG}_${name}.pgn 2>&1 | grep -E 'Score of' | tail -1
}
run TSCP-d3   cmd=./tscp proto=xboard st=2 depth=3
run FairyMax  cmd=./fairymax proto=xboard st=2
run NEG       cmd=./neg proto=xboard st=2
run minnow    cmd=./minnow proto=uci st=2
run SF-n10    cmd=stockfish proto=uci option.Threads=1 st=10 nodes=10
run SF-n100   cmd=stockfish proto=uci option.Threads=1 st=10 nodes=100
run SF-n1000  cmd=stockfish proto=uci option.Threads=1 st=10 nodes=1000
echo POOL-DONE
