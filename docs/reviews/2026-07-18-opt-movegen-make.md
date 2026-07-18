# Deep optimization review: make/unmake, movegen/emit, pawnterm

2026-07-18. Read-only cycle-hunting review by a Fable agent role-playing a
fanatical 6502 optimizer. Companion report (attacked/hash/eval-taper/TT)
in a separate doc. Cycle assumptions: make ≈ unmake ≈ 1.4/node, ~40%
captures (qs-heavy tree), emitmove ~12/node amortized, pawnterm ~0.1
calls/node.

ZP verification: defs.inc allocates scratch through `ATT77 = $2A`; BOARD
starts at $40. No `.s` file references $2B-$3F — **21 ZP bytes free**.

## Ranking by estimated cycles/node

| # | Change | Est. cycles/node | Status |
|---|--------|------------------|--------|
| 1 | Swap-partitioned (two-ended) emit + segment passes | ~200-600 | open (widest error bars; A/B required) |
| 2 | emitmove streamline (drop dead GENCAPS filter; overflow check on page-cross only) | ~180-250 | open |
| 3 | SLOTTAB (256-byte table replaces slotof) + dead pha/pla in unmake + VICTIM ZP | ~120-130 | open |
| 4 | Two-copy generate (GENCAPS specialization) | ~60-120 | open (~350 bytes dup code) |
| 5 | pawnterm bitmask restructure | ~60-90 | open (only when FT_PSTRUCT on) |
| 6 | Castle-helper table dispatch; make-prologue trims | <5 | rejected — prologue already optimal |

Top-3 shortlist: **emitmove streamline** (near-zero risk; deleted filter
becomes a caller contract), **SLOTTAB** (essentially riskless, perft
catches divergence instantly), **two-ended emit** (moderate risk: quiet
reordering shifts node counts/tie-breaks — gate with fixed-depth node
A/B + games, not just perft).

---

## 1. make/unmake (asm/board.s)

Costs as-is: undo prologue 17 `lda zp/sta abs,x` pairs = 136 cycles
(mirror restore 98) — **already optimal shape**; alternatives (undo
records via `(ptr),y`, conditional saves, reconstructing UNDOPIECE /
UNDOFROM from the move stack) all lose or wash once the null-move
NOSQ marker (read at search.s:329) and promo reconstruction are
accounted for. `slotof` = 38 cycles/call × ~3.9 calls/node ≈ 150
cycles/node; 17% of perft-build make, 33% of unmake.

### SLOTTAB

gentables addition (page-aligned):

```go
// SLOTTAB[b] = (b>>4) | ((b&8) != 0 ? 16 : 0)  — slot for piece byte b
var slotTab [256]byte
for i := range slotTab {
        s := i >> 4
        if i&8 != 0 { s |= 16 }
        slotTab[i] = byte(s)
}
```

Re-encoding the piece byte as `slot<<3|type` (making slotof three
`lsr`s) was considered and rejected: breaks KINDTAB's `and #$0F` in
hashpiece, INDEXMASK promotion logic, every COLORMASK test, and the Go
parser — the table gets the same 6-9 cycles with zero encoding churn.

Call-site pattern (never touches X, so X=PLY invariants survive):

```asm
        ldy piecebyte
        lda SLOTTAB,y
        tay                     ; 9-10 cycles, was jsr slotof = 38
```

Sites: make capture path (also add `VICTIM = $2B` ZP to kill the
pha/pla juggling, and add it to defs.inc for ParseDefs), make mover
placement (board.s:234-237), unmake mover restore (board.s:561-566),
unmake capture restore (board.s:574-583), castlerook, uncastlerook.
Then delete slotof entirely.

**Dead code found**: the `pha`/`pla` pair in unmake's capture restore is
dead — all three unmake callers (search.s:800, search.s:832,
perft.s:94) immediately `ldy PLY / jmp sloop` and never read A. −7
cycles per capture-unmake just by deleting it.

Savings: ~30/site × 3.9/node ≈ **~120 cycles/node** (+4/node dead
pha/pla, +5/capture-make from VICTIM). Caveats: SLOTTAB must equal
`index | (color?16:0)` exactly (perft catches instantly); page-align;
rempiece/hashpiece verified not to touch $2B.

## 2. generate/emitmove (asm/movegen.s)

Cost as-is: 69 cycles/emit (quiet gen), 88/kept qs capture. Dispatch
chain already pawn-first-optimal; SMC jump table loses (23 flat).

### emitmove streamline

**The GENCAPS filter inside emitmove is provably dead.** Audit of all
20 call sites: every quiet emission (gsempty :126, gstquiet :171, pawn
pushes :322/:389, castle gate :181) is already gated caller-side on
GENCAPS; every capture/ep/promo site always passes the filter (BOARD[GTO]≠0
or flags∩(EP|PROMO)≠0). Promo *pushes* bypass the pawn GENCAPS gate by
design and are always kept. Also: the overflow compare can move onto
the page-cross path (MSP+1 only grows via carry out of the low bump).

Replacement (54 cycles, −15/emit gen, −34/emit qs):

```asm
; CONTRACT: with GENCAPS set, callers must not call this for quiet
; moves (all quiet sites are gated on GENCAPS; captures/ep/promos
; are always kept, so no filter is needed here).
emitmove:
        ldy #2
        sta (MSP),y
        dey
        lda GTO
        sta (MSP),y
        dey
        lda GFROM
        sta (MSP),y
        lda MSP
        clc
        adc #3
        sta MSP
        bcs empage
        rts
empage: inc MSP+1
        lda MSP+1
        cmp #>MOVESTACKTOP
        bcc :+
        lda #100
        sta EXIT_TRAP
:       rts
```

Bonus: a two-byte `emitmove0: lda #0` prefix entry converts the twelve
`lda #0 / jsr emitmove` sites to `jsr emitmove0` (−24 bytes,
cycle-neutral). Savings ≈ **~180-250 cycles/node**. Caveat: the filter
becomes a caller *contract* — keep the comment; any future quiet emit
site must carry its own GENCAPS gate (perft doesn't exercise GENCAPS;
qs is gated only by search regression).

### Two-copy generate (optional)

Separate generate/generateq selected at gennode: removes the 5-6 cycle
`lda GENCAPS/bne` paid per empty candidate square (~25-35/generation)
≈ 150+/generating node. Cost ~350 bytes duplication + drift hazard.
~60-120 cycles/node.

### Two-ended emit — full cost/benefit

Search rescan cost (sloop→sfetch→classify, search.s:568-758) ≈
**~110-126 cycles per rejected entry**; a 35-move ALL-node with TT move
pays ≈13,000 cycles of pure rescan across passes 0-4. Cut-nodes
stopping in pass 0/1 pay far less (which is what keeps the current
scheme alive).

True two-ended (quiets descending from a ceiling) is a non-starter:
fixed per-ply arenas for 32 plies × ~96 moves ≈ 9KB vs the 4.6KB
arena. Workable variant: **swap-partition at emit time** — capture
sites call a new `emitcap` which appends then 3-byte-swaps down to a
boundary pointer `CAPB` ($2C/2D, free ZP; TSWP $2E/2F scratch).
Classification is free (capture and quiet emissions come from
different call sites). Per-ply capture counts in the 32-byte hole
$0DE0-$0DFF (boundary = base + 3×count via regenerable MUL3 table).
Search passes 1-2 iterate base→boundary, 3-4 boundary→end, pass 0
unchanged (full scan for TT move); surviving pass-3/4 rejections also
lose their 16-cycle board-probe classify.

Costs: +8/emit average (+20 no-swap to +60 swap on ~15% of emits).
Benefits: kills quiet rescans in passes 1-2 (≈6,600-7,500 at a
30-quiet node) and capture rescans in passes 3-4 (≈1,100-1,400) at
nodes reaching those passes; qs single-segment nodes save nothing.
Weighted: **~200-600 cycles/node**. Caveats: quiets reorder by
displacement → node counts and tie-breaks shift; perft passes
regardless — validate with fixed-depth node A/B + games; preserve the
spop/MSP restore invariant.

## 3. pawnterm (asm/eval.s:199-346)

Cost as-is ≈ 1,200-1,800 cycles midgame (header's 800-1200 is
optimistic) ≈ 120-180/node with FT_PSTRUCT on.

Full incremental file counts in add/rempiece **rejected**: unmake
restores accumulators wholesale and never calls add/rempiece, so
incremental state needs hooks in every unmake (~20-25 cycles × 1.4/node)
with real undo-symmetry risk.

Chosen shape: keep the pure recompute, but replace six per-file byte
arrays with two 8-byte **rank-bitmask** arrays (PWBITS/PBBITS at
$0200-$020F; $0210-$022F retired) plus regenerable 256-byte tables:
`RANKBIT[128]` (1<<rank by 0x88 sq), `WBLOCKM` ($FF<<hibit),
`WPASSB` (PASSEDBONUS[hibit]), `BBLOCKM` ((1<<(lobit+1))−1),
`BPASSB` (PASSEDBONUS[7−lobit]). Doubled = `bits & (bits−1) ≠ 0`.
Full replacement code in the agent transcript; savings ≈ −600 to −900
per call (~45%) → **~60-90 cycles/node**. Stretch: with bitmasks in
place, xor-RANKBIT incremental in make+unmake (xor is its own undo)
kills scan+clear for another ~+40/node at the symmetry risk above.

Semantic landmines to preserve exactly:
1. White "blocked" uses ≥ (WBLOCKM includes own rank bit); black uses ≤
   (BBLOCKM includes rank r).
2. **Existing discrepancy found**: comment says doubled = "−12 per
   extra pawn" but the code (eval.s:274-279) applies a flat −12 for
   any count ≥ 2 (tripled ≠ −24). Replacement must reproduce the code,
   not the comment. Flag for pstruct weight tuning (task #20 Texel).

Gate: perft doesn't cover eval — the gate is **exact term parity** with
today's code on a position battery, plus fixed-depth node counts.
