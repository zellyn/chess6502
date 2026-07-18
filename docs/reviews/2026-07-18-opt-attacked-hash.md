# Deep optimization review: attacked, hash/eval updates, taper, TT

2026-07-18. Read-only cycle-hunting review by a Fable agent role-playing
a fanatical 6502 optimizer. Companion doc:
2026-07-18-opt-movegen-make.md. Interrupts are off and code is in
RAM, so self-modifying code is legal everywhere. ZP $2B-$3F verified
free. All table changes are cmd/gentables regenerations.

## Ranking (cycles/node, typical midgame)

| # | Change | /call | /node | Status |
|---|--------|-------|-------|--------|
| 1 | movepiece fusion (+ kind-major Zobrist layout, hashpiece v2, takepiece) | ~230/make + 80/capture | ~235 | open |
| 2 | attacked rewrite (SMC slot base, carry-trick diff, TYPEATK2) | ~295 | ~118 | open |
| 3 | eval 5-bit right-shift multiply | ~190 | ~75 (0-150 phase-dep.) | open |
| 4 | tt: table ttaddr + unrolled read/store | 17-59 | ~40 | open |
| 5 | hashpiece v2 alone (if #1 deferred) | 17 | ~37 | subsumed by #1 |
| 6 | king-square ZP cache | 4 | ~3-8 | rejected (net wash) |

## 1. movepiece fusion — the big one (~235/node)

Current mover work in make = hashpiece(FROM) + rempiece + hashpiece(TO)
+ addpiece + glue ≈ 562 cycles, and rem+add of the *same* piece
recomputes kind, phase (net zero!), PDIRTY, both PSQT page setups, and
the color flip, staging PSQT values through T0-T3.

**Layout change (gentables):** replace the 4×1536 plane-major Zobrist
tables with kind-major 512-byte blocks: `ZKEYS + kind*512` =
plane0[128] plane1[128] plane2[128] plane3[128]. Same key values, same
6KB total, every block starts on an even page ⇒ ZPTR lo is permanently
0 (set once in evalinit). New 16-entry tables indexed by piece&$0F:
`ZKHI0` (block page), `DIRTYTAB` (1 at 1,6,9,14), `TYPEPG0X`/`TYPEPG1X`
(PSQT pages), `PHASEV16`.

**hashpiece v2** (drop-in, same contract, Y preserved, 78 vs 95
cycles): index blocks via `(ZPTR),y` with Y toggled through sq/sq|$80
and one `inc ZPTR+1` for planes 2/3. Beats SMC'ing four `lda abs,y`
high bytes (74 cycles but 8 patch stores make it a wash).

**movepiece** (A ≈ 330 incl. jsr vs 562 ⇒ −230/make on non-promo
moves): one kind/page setup; PDIRTY via DIRTYTAB; PHASE untouched
(provably cancels); hash = xor key[kind][FROM]^key[kind][TO] across 4
planes in one pointer chain; PSQT applied as a from/to delta straight
into MGSCORE/EGSCORE (black via sq^$70 swap of add/sub squares).

**takepiece** (optional, −75-90/capture ≈ 26/node): same skeleton for
the victim — unconditional `sbc PHASEV16,x`, one hash chain, one-square
PSQT adjust; replaces the pha/jsr hashpiece/pla/pha/jsr rempiece
juggling.

Make restructure: board writes first, then `beq mknopromo → jsr
movepiece`; FL_PROMO and castlerook keep the old
hashpiece/rempiece/addpiece sequence (piece byte / phase change; castles
rare).

Caveats: (i) internal/chesstest reads ZPLANE0/KINDTAB symbols — update
to ZKEYS/ZKHI0 (key values unchanged, only rearranged, so stored hashes
stay comparable). (ii) ZPTR/PSP0/PSP1 lo == 0 becomes a hard evalinit
invariant. (iii) Gate on the incremental-vs-evalinit cross-check at
every node + full perft.

## 2. attacked rewrite (~118/node)

Current: ~734 cycles/typical call (14 live misses + 2 tombstones).
Rewrite ≈ 439 (−295/call, 0.4 calls/node):

1. X as slot counter (`dex/bpl`), side via **SMC on the PIECESQ operand
   low byte** ($00/$10; PIECESQ page-aligned, X≤$0F ⇒ no cross).
2. Diff via `eor #$FF / adc ATT78` where ATT78 = ATSQ+$78 precomputed —
   after `cmp #NOSQ` falls through, carry is provably clear, absorbing
   the missing +1. **The adc depends on the preceding cmp/beq — never
   reorder.**
3. 16-entry `TYPEATK2[piece&$0F]` (wp=$10 bp=$20 N=$01 B=$04 R=$08
   Q=$0C K=$02) kills the pawn special-case entirely — wrong-direction
   pawns fast-reject through the same `and ATBITS`.

Live-miss slot 47→27 cycles, tombstone 25→14. Slot scan order reverses
(king last) — irrelevant to OR semantics. TYPEATKTAB stays for make's
ckfast/ckdwalk (candidate to migrate later). Full code in the agent
transcript. Gate: perft (NOEVAL build) exercises it exhaustively.

## 3. eval taper multiply (~75/node avg)

On the multiply path w ∈ 1..31 (w=32/w=0 fast-pathed) ⇒ only 5
multiplier bits. A **right-shift multiply** of exactly 5 iterations
produces `(D*w)>>5` directly: no 24-bit accumulator, no post-shift
loop, and truncation is bit-identical (shifted-out bits are exactly the
product's low 5 bits). ~191 cycles vs ~380-400. Accumulator provably
fits 16 bits. The `clc` after `bcc` is required; the bit-clear path
relies on bcc-taken ⇒ carry clear feeding `ror`. Gate: diff old-vs-new
SCORE over a FEN suite (must be bit-identical).

## 4. TT routines (~40/node)

- ttaddr via tables `SHL3TAB`/`SHR5TAB`/`TTHITAB` (768 bytes): 57→40
  cycles. Final `adc #>TTBASE` must stay adc (base $02 overlaps bit 1).
- ttread unrolled (stays in LCCODE, +~24 bytes): 8-aligned entries ⇒
  `(TTPTR),y` never crosses; 142→~100. Mirror in ttstore (−24/call).
- If bytes are tight: take only the unrolls (−26/node, zero table
  cost).

## Rejected

- King-square ZP cache: saves 4-5/use × 1-2 uses/node minus make-side
  update cost — net wash.
- SMC plane-pointer high bytes: loses to the kind-major layout.

## Top-3 shortlist

1. **movepiece fusion + Zobrist relayout** — risk: Go harness symbol
   changes; promo/castle must provably stay on the old path.
2. **attacked rewrite** — risk: carry trick + SMC fragile under future
   edits; comment aggressively; perft covers it exhaustively.
3. **eval right-shift multiply** — lowest risk; provably bit-identical.
