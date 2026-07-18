# Decision log

Lightweight ADR-style records. Statuses: **accepted**, **proposed**,
**revisit** (accepted but flagged for re-evaluation at a named milestone).

## D1: Cross-assemble with ca65/ld65 — accepted

Merlin32, ACME (which a2audit pins), 64tass, and go6502's own assembler
(Merlin/SCMA flavors) all work, but ca65 has the strongest linker story for
a multi-segment image (main RAM, Language Card RAM, aux RAM segments with
separate load/run addresses), maintained macro tooling, listing/map files
we can feed back into the Go harness for symbolized traces, and the largest
Apple II community usage. go6502's assembler stays useful for its own
tests; a2audit stays on its pinned ACME. Revisit only if ld65 segment
handling proves awkward for the bank-copy loader.

## D2: Target the NMOS 6502, not the 65C02 — accepted (revisit at M4)

The 65C02 (enhanced IIe) offers PHX/PHY/PLX/PLY, BRA, STZ, (zp) indirect,
INC A, JMP (abs,X). Honest ceiling (adversarial review): ~8-15% in hot
paths, which at fixed clock is roughly 12-17 Elo — real, but smaller than
any single search feature on the list. Against it:

- go6502 has no 65C02 opcode support (documented-NMOS only), and its
  gate-level perfect6502 lockstep verification — our strongest correctness
  asset — exists only for the NMOS part. A 65C02 core would be new; it
  could be *functionally* verified (Dormann's 65C02 extended-opcode test
  exists) but not at gate level (no public 65C02 netlist).
- NMOS runs on every IIe including unenhanced ones (and the ][+ for a
  possible 64K build), broadening real-hardware targets.
- The engine's speed lives mostly in algorithmics (pruning, ordering), not
  15% instruction-level wins.

Decision: all engine code is documented-NMOS-6502 clean. If profiling at
M4+ shows hot loops where 65C02 forms would win big, revisit with a
build-flag approach (conditional macros emitting 65C02 forms) — and only
then design 6502/65C02 switching for go6502.

## D3: Own harness importing go6502 + goapple2/iie; no full emulator — accepted

The harness (`cmd/a2run`) is ~150 lines: go6502 CPU + the iie memory model
+ traps. No ROM boot, no video, no disk. goapple2 remains the interactive
emulator; the new `iie` package retrofits the IIe memory system into the
goapple2 repo where it is generally useful (stage 2 wires it into the
full machine).

## D4: Aux memory via RAMRD/RAMWRT with 80STORE off; aux-touching code runs from LC RAM or $0000-$01FF — accepted

With 80STORE off, RAMRD/RAMWRT bank all of `$0200-$BFFF`, giving ~47.5K of
aux for tables with dead-simple rules. Critical correctness details
(confirmed by adversarial review against Sather):

- RAMRD switches *instruction fetches* in `$0200-$BFFF` too, so any
  routine that turns RAMRD on must itself execute from `$0000-$01FF`,
  Language Card RAM (`$D000+`), or must only toggle RAMWRT
  (writes-to-aux while executing from main is safe). Aux-access
  primitives therefore live in LC RAM.
- While RAMRD is on, main `$0200-$BFFF` is unreadable — so aux
  primitives take arguments and return results **in zero page only**
  (hash key in, entry copy out); they cannot touch main-RAM tables
  mid-toggle.
- **ALTZP is never touched** (hard invariant for stage 1): it swaps ZP,
  the stack, and LC RAM simultaneously — code in main LC RAM that sets
  ALTZP pulls its own instruction fetches out from under itself, and any
  code that sets it loses its stack for RTS. The aux-LC "stage-2 ideas"
  in the memory budget are parked behind lifting this invariant safely.

The audit suite validates the underlying switch semantics.

## D5: Board representation: 0x88 board + piece lists — proposed

0x88 gives a one-instruction off-board test (`AND #$88`), signed-byte move
deltas, and difference-indexed attack/direction lookup tables (256-byte,
page-aligned) that prune candidate directions for is-square-attacked
(sliders still ray-walk for blockers — shorter loops, not no loops).
16-entry piece lists per side drive move generation (no 64/128-square
board scans), following H.G. Muller's "mailbox trials" results: captures
generated first (per-victim staging approximates MVV order with no
sorting). Tables indexed directly by 0x88 square (128 bytes/table, 64
wasted) to avoid index conversion in hot paths.

Open sub-decisions the M1 spike must price in BOTH arms (0x88+lists vs
10x12), per adversarial review — the naked movegen inner loop is not the
whole cost:
- Finding a captured piece's list entry: board-byte encodes list index
  (color|type|index fits 8 bits; Kittinger-style) vs a 128-byte
  square-to-index map vs linear scan of 16.
- Removal discipline: tombstone (square=$FF — skipped for free by the
  `AND #$88` test, a genuine 0x88 synergy) vs swap-with-last (breaks
  index stability; grows the undo frame).
- Promotion bookkeeping (piece type changes mid-list).

## D6: 32-bit Zobrist hashing; transposition table in aux RAM — proposed

32-bit keys, incrementally XOR-updated in make/unmake (~160 cycles for
the two square-keys plus side-to-move; see the cycle budgets in plan.md).
Key set: 12 piece-kinds x 128 squares x 4 bytes = 6K in main RAM, 0x88-
indexed, **plus side-to-move, 16 castling-rights, and 8 ep-file keys**
(forgetting these is a classic TT bug); keys generated offline by a
vetted PRNG (bad 32-bit key sets measurably raise collision rates).

TT in aux `$0200-$BFFF`: **4096 entries x 8 bytes = 32K — power-of-2
only** (mod-5888 indexing needs a division the 6502 won't pay per probe,
and 8192x8 = 64K can never fit), leaving ~15K of aux explicitly for the
opening book and optional KPK data. Entry: 20 verify bits (all the
non-index bits — they're free), packed move, 16-bit score, depth, bound
flags. Collision math: ~0.3-0.6 false verify-matches per game at our
node rates — harmless for scores (Hyatt & Cozzie), **fatal for an
unvalidated move on a 6502** (piece-list/accumulator desync). Therefore:
**the TT move (and killer moves — also from sibling nodes) are pseudo-
legality-checked before use**: right piece on from-square, target not own
piece, slider path clear (~60-100 cycles, TT-move nodes only).

Aux probe cost ~180-230 cycles round trip (RAMRD toggles + 8-byte ZP
copy from LC-resident primitive); store ~120 (RAMWRT-only, callable from
main code). A main-LC TT would save ~100 cycles/probe but caps at ~1024
entries — two halvings ≈ ~16 Elo by Muller's rule — aux wins. QS
probing: default OFF (~230 cycles on the ~60% of nodes that are QS, for
ordering MVV staging mostly provides); settle by measurement at M3-M4.

Replacement: always-replace vs two-slot depth-preferred — measured, not
assumed (evidence says always-replace is nearly as good at small sizes).
Repetition detection uses a separate game-history + search-path list of
truncated hashes in main RAM, not the TT; a truncated-hash match is
verified against the full 32-bit key before scoring a draw. See plan.md
for repetition-vs-TT ordering rules.

## D7: Reserve hires page 1 ($2000-$3FFF) for a future board display — accepted

The engine and its tables stay out of `$2000-$3FFF` so a simple hires board
(nothing fancy) can arrive later without a memory-map upheaval. Engine code
loads at `$4000` upward; `$0800-$1FFF` holds low tables/move stack. If we
end up desperate for main RAM, this is the first decision to revisit.

## D8: Harness I/O via store-traps at $BFF0 (COUT) / $BFFF (exit); engine I/O behind a vector table — accepted (amended)

Store-traps are invisible on real hardware (plain RAM writes). The engine
calls I/O only through a small vectored module (put-char, get-line,
move-out), so the same engine core links against a harness I/O module today
and a keyboard/screen (or Super Serial) module later.

Amendments from adversarial review (both critics found the collision):
- **$BF00-$BFFF is reserved in BOTH banks** — the memory-budget tables
  and ld65 segment configs must exclude it. Without this, engine tables
  growing to the top of main RAM, or aux TT/book data crossing $BFF0,
  would spuriously emit protocol bytes or kill the run.
- Traps fire **only on main-bank stores** (the harness checks RAMWRT),
  so aux writes to those addresses are ordinary memory. Implemented.
- The trap addresses are canonical: the `-cout`/`-exit` flags exist for
  experiments, but the checked-in I/O module and all tooling assume
  $BFF0/$BFFF.

## D12: Harness input and long-lived sessions — proposed (implement at M3)

For the UCI bridge the engine must accept input and answer repeatedly
without restarting. Design: memory-mapped **input traps** mirroring
keyboard-strobe hardware shape — read $BFF1 = next input byte (pops),
read $BFF2 = status ($80 when a byte is waiting). The engine polls
status; the harness feeds bytes from stdin (a goroutine). One long-lived
a2run-style process holds the session; the same engine code drives real
keyboard hardware later by swapping the I/O module (D8). Prefer this
over PC-callbacks (less hardware-faithful) and over $C000 keyboard
emulation (drags in ROM/soft-switch surface the trap harness avoids).
Also planned then: extract the a2run core into an importable Go package
(load/run/traps as a library) so perft and gauntlet rigs call it
in-process instead of scraping CLI output.

## D9: Time management: cycle/node budgets; VBL ($C019) polling on real hardware — proposed (amended)

No interrupts: go6502 doesn't emulate IRQ/NMI delivery, unenhanced IIe VBL
polling is cheap anyway, and interrupt-free code is simpler to verify.

The original "poll $C019 every 256 nodes and count VBL edges" scheme was
**killed by adversarial review**: at ~2000 cycles/node, 256 nodes is
26-38 frames between samples, and a 1-bit flag cannot count frames it
never saw — the clock would run 15-30x slow. Corrected scheme:

- Primary through M7 (emulator): **fixed emulated-cycle budgets**. The
  engine's own node counter gives in-engine pacing; the SPRT/gauntlet
  rigs enforce cycles (deterministic AND time-faithful, unlike node
  budgets, which are blind to per-node cost changes — see plan.md
  testing). $C019 is now emulated (cycle-fed) so the hardware path is
  testable in the harness.
- Hardware (M8): poll $C019 every 1-2 nodes (LDA/EOR/BPL ≈ 10-13 cycles
  ≈ 0.5% overhead) so no VBL period (4,550 cycles) is ever missed;
  count frames at 59.92 Hz. Sense: on the IIe, $C019 bit 7 is LOW
  during vertical blanking (inverted on IIgs; IIc has no polled VBL).
- Iteration policy: don't start a new iteration past ~50% of budget
  (EBF ~3 means a new iteration roughly doubles spend); on mid-iteration
  hard abort at 2x, play the best move found so far in the current
  iteration if at least one root move completed and improved on the
  previous iteration's score — else the previous iteration's move.

## D10: Plain docs/ with this decision log, not full ADR directory — accepted

Two-contributor hobby repo; decision granularity above is enough. If the
project grows contributors or the log gets unwieldy, graduate entries to
docs/adr/.

## D11: Elo measurement: emulated engine bridged to UCI; four-part protocol — accepted (amended)

`cmd/uci` (Go) presents the emulated engine as a UCI engine to
cutechess-cli (MessChess/retro-Sargon prior art). Adversarial review
rewrote the measurement design; the four products, in order of rigor:

1. **Feature gates (SPRT self-play)**: both sides at fixed emulated-cycle
   budgets, **paired openings from a standard suite with color reversal**
   (a deterministic engine from startpos plays one photocopied game —
   without openings the ±Elo math is fiction), SPRT elo0=0/elo1=10.
2. **Absolute CCRL-anchored gauntlet**: engine at its real control
   (30-60s x 1.0205 MHz as a cycle budget ≈ 0.2-0.4s wall at ~170x);
   opponents at their CCRL-valid conditions (40/4-style), chosen to
   **bracket the engine's actual level within ±200 Elo** (pool roughly
   1000-1700 — not just TSCP/micro-Max, which may sit 200-500 above).
   Generous cutechess wall margins so the emulated engine never forfeits
   on wall clock. ~500-1000 games; report error bars including anchor
   uncertainty, not just 630/sqrt(N).
3. **Node-odds ladder** (the thesis number): node/cycle-limit micro-Max
   and TSCP to create calibrated opponents near our level, rate them
   against pool 2, and find the crossover: "micro-Max needs Nx our
   nodes to hold 50%." This is the cleanest quantification of how far
   theory has come.
4. **Period showdown (M7)**: emulated period engines at authentic 1 MHz,
   equal per-move emulated time, paired openings, pre-registered win
   criterion (>=65% over >=100 paired games).
