#!/bin/bash
# Resume of pool.sh @c96f604: legs that never ran (minnow + SF ladder).
set -x
cd /Users/zellyn/gh/chess6502/tools
TAG=c96f604
ENG="-engine name=chess6502 cmd=./bin/chess6502-uci arg=-bin arg=/Users/zellyn/gh/chess6502/asm/engine.bin arg=-defs arg=/Users/zellyn/gh/chess6502/asm/defs.inc arg=-budget arg=30000 arg=-dither arg=-bank proto=uci st=10 timemargin=5000"
OPEN="-openings file=openings-pool.epd format=epd order=sequential"
run() { # name, engine-args...
  local name=$1; shift
  ./cutechess-cli $ENG -engine name=$name "$@" $OPEN -rounds 15 -games 2 -repeat \
    -pgnout pgn/pool_${TAG}_${name}.pgn 2>&1 | grep -E 'Score of' | tail -1
}
run minnow    cmd=./minnow proto=uci st=2
run SF-n10    cmd=stockfish proto=uci option.Threads=1 st=10 nodes=10
run SF-n100   cmd=stockfish proto=uci option.Threads=1 st=10 nodes=100
run SF-n1000  cmd=stockfish proto=uci option.Threads=1 st=10 nodes=1000
echo POOLREST-DONE
