# Research: engine design tradeoffs for low-NPS hardware

*Web research conducted 2026-07-17 (agent-assisted). Preserved as input to
docs/plan.md; URLs are the citations. Operating point assumed: ~300-2,000
NPS, depths ~5-9 plus quiescence. Caveat: most published Elo numbers come
from engines at millions of NPS; where possible this anchors to sources
addressing low-depth/minimal engines (chiefly H.G. Muller's micro-Max
documentation and TalkChess posts).*

## 1. Search stack for low NPS

- **Quiescence search** — universally rated the single most important thing after alpha-beta itself. In the TalkChess thread ["Which features offer the best return?"](https://www.talkchess.com/forum/viewtopic.php?p=880859), H.G. Muller calls QS "extremely important" and jdart's priority list starts with QS, then hash table, then recursive null move. Micro-Max's history quantifies this: switching from a recapture-only/SEE-like QS to **all-captures QS with MVV/LVA ordering** was worth **~80 Elo in self-play** ([hccnet.nl mvv.html](https://home.hccnet.nl/h.g.muller/mvv.html)). Micro-Max does a two-pass sweep: pass 1 scans all moves to find the best MVV/LVA capture without searching (no beta cutoff in pass 1), pass 2 searches captures starting with it — a trick that needs no sort buffer, attractive on a 6502.
- **Iterative deepening + previous-best-move-first** — essentially free and mandatory. Micro-Max orders only the hash/previous-iteration best move first, all other moves in generation order ([deepen.html](https://home.hccnet.nl/h.g.muller/deepen.html)). "Trying idiotic moves first can easily lead to an explosion of the search" ([max-src2.html](https://home.hccnet.nl/h.g.muller/max-src2.html)).
- **Transposition table** — worth roughly **130-160 Elo** even when small ([Best practices for transposition tables](https://talkchess.com/viewtopic.php?t=76508)). HGM: a **dead-simple always-replace table performs almost identically to sophisticated replacement schemes** at small sizes. The hash *move* (ordering) is worth more than the hash *score* at these depths.
- **Null move, R=2** — HGM: null-move pruning "will bring you most Elo, and is very simple to implement." Micro-Max 4.4 uses **R=2** and guards zugzwang by *disabling null move when captured material implies ≤2 minor pieces total remain* — a material-count guard, no verification search ([null.html](https://home.hccnet.nl/h.g.muller/null.html)). Side benefit: the null-move result doubles as a check detector (distinguishing mate from stalemate).
- **Killer moves** — cheap (2 bytes/ply) and real: Rustic measured **~56 Elo self-play** (~35 vs. foreign opponents); HGM: "killer always helped my engines a lot. (In contrast to history.)" ([TalkChess t=77734](https://talkchess.com/viewtopic.php?t=77734)).
- **History heuristic** — unreliable at low depth per the same HGM quote; needs a 64x64 or piece×square counter table with aging — poor Elo/byte and Elo/cycle on a 6502. **Skip.**
- **Futility (frontier) + delta pruning in QS** — micro-Max 4.2 added futility with its MVV/LVA QS; "futility pruning prevents most calls that would give a stand-pat cutoff in QS" — mostly saves make/unmake+eval cycles, exactly what matters at 1 MHz ([mvv.html](https://home.hccnet.nl/h.g.muller/mvv.html)). Reverse futility gained ~145 Elo in one modern writeup ([int0x80](https://int0x80.ca/posts/chess-engines/9-rfp-nmp)) but with few applicable depths here expect modest gains. Delta pruning margins ~1 pawn are safe with PSQT-only eval.
- **LMR** — standard condition depth ≥3, move number >4, non-capture, not in check ([CPW LMR](https://www.chessprogramming.org/Late_Move_Reductions)). Works mostly at moderate+ depth; can propagate errors at shallow depth. Micro-Max 4.8 did adopt LMR, so viable even in tiny engines — implement *after* everything above, reduce by 1 only, never in PV nodes.
- **Aspiration windows** — mixed; typically 10-20 Elo when tuned, sometimes negative (MadChess was 9 Elo *stronger without*) ([talkchess t=76115](https://talkchess.com/viewtopic.php?t=76115)). Re-search overhead hurts when branching factor is high. **Low priority.**
- **Internal iterative deepening** — cheap given ID machinery; matters mainly with no hash move. Optional.
- **Checks in qsearch** — micro-Max does not search checks in QS. Generating checking moves is expensive without attack tables; QS explosion is dangerous at low NPS. Skip initially; a cheap substitute is the check *extension* in the main search.

**Ranked by Elo per implementation+cycle cost:** 1) QS with MVV/LVA (captures only, stand pat, delta prune) → 2) iterative deepening + TT-move-first ordering → 3) null move R=2 with material zugzwang guard → 4) small always-replace TT → 5) killers → 6) futility/RFP at depth ≤2-3 → 7) LMR (conservative) → 8) IID → 9) aspiration windows → 10) history heuristic and QS checks (skip).

## 2. Evaluation on a budget

- **PSQT-only is remarkably strong.** [PeSTO's evaluation](https://www.chessprogramming.org/PeSTO%27s_Evaluation_Function) (Texel-tuned tapered mg/eg tables from RofChade) is material + PSQT + side-to-move only, and reached TCEC CPU League 1. **Swapping PeSTO tables into TSCP gave TSCP +200 Elo** (Pawel Koziol, same page). Exact table values are public in the C source there ([RofChade thread](http://www.talkchess.com/forum3/viewtopic.php?f=2&t=68311&start=19)). mg/eg piece values: P 82/94, N 337/281, B 365/297, R 477/512, Q 1025/936; phase = N,B=1, R=2, Q=4, capped at 24; score = (mg·phase + eg·(24−phase))/24.
- **Reference points:** micro-Max 4.8 — almost no eval beyond PST-ish terms + a "magnetic frozen king" king-safety hack — ~1891 CCRL 40/40; TSCP 1.81 with a simple hand eval ~1700. Search quality + tuned PSQT dominates at hobby level.
- **Tapered eval on 6502:** keep running mg-score, eg-score, and phase updated in make/unmake; taper once per evaluated node (the standard PeSTO two-accumulator pattern).
- **Marginal terms by yield per cycle:** (1) tempo (~10 cp, nearly free); (2) **passed pawns** (biggest classical-eval term per jdart); (3) doubled/isolated pawns — cheap with per-file pawn counts; (4) minimal king safety — micro-Max's "magnetic, frozen king" costs almost nothing and PeSTO's king PSQT already encodes stay-castled/centralize-in-endgame; (5) **mobility — not worth it at 1 MHz** (roughly doubles eval cost). Skip.
- **Recommendation:** PeSTO tapered PSQT + tempo, incremental; add passed/doubled/isolated pawns later (ideally cached in a tiny pawn hash — even 256 entries — since pawn structure changes rarely).

## 3. 16-bit arithmetic on an 8-bit CPU

Sources: [CPW 6502](https://www.chessprogramming.org/6502), [talkchess t=20831](https://talkchess.com/viewtopic.php?t=20831) (8/16-bit machines hit 2000+ USCF: Super Constellation 2014 USCF on 5 MHz 6802; Schröder's Rebel via TK20 TurboKit), MyChess, Sargon.

- **Scores:** 16-bit centipawns (two-byte add/sub via carry chain; compare via SBC high-then-low). Don't shrink to 8 bits — PeSTO values exceed ±127 and mate handling gets ugly. Negamax with symmetric bounds (±32000-ish INF, avoiding −32768 asymmetry).
- **Mate scores:** MATE=30000, return MATE−ply at mated nodes, adjust by ±ply when storing/probing TT ([CPW Checkmate](https://www.chessprogramming.org/Checkmate)). Micro-Max's crude alternative is a known weakness (slow mate convergence).
- **Incremental vs. recompute:** with material+PSQT, incremental update in make/unmake is unambiguously right: 2-4 table lookups + 16-bit adds per make (~40-80 cycles) vs. >1000 for a full scan. Keep three accumulators (mg, eg, phase). Full recompute only at root as a debug check.
- **6502 specifics:** zero page for board pointers, accumulators, move-stack frame; 256-byte page-aligned lookup tables; split PSQT into lo/hi byte tables per piece to keep adds 8-bit with carry.

## 4. Move generation on 6502

- **Board representation:** mailbox with out-of-board guard — 0x88 (one-AND off-board test; [0x88 in 6502 assembly demo](https://www.youtube.com/watch?v=q4A8SDaWrtw)) or micro-Max's 16x8 variant / 10x12 with sentinels. Bitboards hopeless on 8-bit.
- **Piece list vs. board scan:** H.G. Muller's ["mailbox trials"](https://www.talkchess.com/forum3/viewtopic.php?t=76773&p=885827) ([code mirror](https://github.com/maksimKorzh/hgm-mailbox-trials)): optimized design is **16x12 mailbox + piece list**, move list grown in two directions — **captures prefixed to the front, non-captures appended to the back** — so captures come out first with no sorting pass; staged generation per victim-value group makes MVV order fall out of generation order. A 16-entry-per-side piece list also makes incremental PSQT trivial.
- **Pseudo-legal vs. legal:** every minimal engine generates **pseudo-legal** and handles legality lazily — "king capture = +INF" (micro-Max, essentially free) or make-then-test-king-attacked. Full legal generation (pin detection) not worth the cycles.
- **Historical:** Sargon used board array + per-position attack/defense lists — heavyweight but instructive; the Spracklens found the 6502 better than the Z80 for chess. No published per-move cycle counts exist; mid-80s 2 MHz units achieved ~500-3000 NPS with assembly mailbox generators.

## 5. Time management and timing on Apple IIe

- **Free hardware clock: the VBL flag.** Frame = exactly **17,030 cycles** (65 x 262); on the **IIe poll $C019 (RDVBLBAR)** — bit 7 indicates VBL state (sense inverted on IIgs; IIc exposes VBL only via interrupts) ([VBL timing](https://rich12345.tripod.com/aiivideo/vbl.html), [no VBL on original II](https://quorten.github.io/quorten-blog1/blog/2020/05/23/no-vbl-orig-a2)). 17,030 cycles ≈ 59.92 Hz tick.
  - **Recommended scheme:** poll every ~256 nodes (INC counter / BNE skip), detect VBL *edges* (VBL lasts 4,550 cycles ≈ many polls), count frames, abort search when frames ≥ budget.
  - Fallback needing no hardware: **node counting** — calibrate NPS once, convert time budget to node budget. Robust, emulator-friendly, works on a clockless ][+.
- **Mockingboard 6522 timers**: any slot card with a 6522 VIA gives programmable timer IRQs (period is N+2 cycles — [AppleWin #701](https://github.com/AppleWin/AppleWin/issues/701)). Support as optional; don't require.
- **Allocation policy:** budget = remaining/30 (or fixed s/move); don't start a new iteration past ~50% budget; abort mid-iteration only on hard limit; always keep previous iteration's best move.

## 6. Testing methodology

- **Tooling:** [cutechess-cli](https://github.com/cutechess/cutechess/blob/master/projects/cli/res/doc/help.txt): round-robin/gauntlet, PGN books, built-in **SPRT** (`-sprt elo0=0 elo1=10 alpha=0.05 beta=0.05` with `-rounds 999999`) ([t=78272](https://talkchess.com/viewtopic.php?t=78272), [CPW SPRT](https://www.chessprogramming.org/Sequential_Probability_Ratio_Test), [Rustic SPRT chapter](https://rustic-chess.org/progress/sprt_testing.html)).
- **Games needed:** error ≈ 630/√N: **±20 Elo at ~1000 games, ±30 at ~400-500**. Self-play exaggerates; expect ~60% of self-play gains vs foreign opponents ([t=77734](https://talkchess.com/viewtopic.php?t=77734)). Absolute rating: gauntlet vs CCRL-rated engines near our level (TSCP 1.81 ≈ 1700, micro-Max 4.8 ≈ 1890).
- **Emulator-bridge prior art (mature):** **MessChess** runs 300+ MAME-emulated dedicated chess computers as UCI engines ([ChessBase overview](https://en.chessbase.com/post/the-wonderful-world-of-chess-machine-emulators)); **MAME-4-PicoChess** ([GitHub](https://github.com/ScallyBag/MAME-4-PicoChess)); dedicated/emulated machines on Lichess via lichess-bot + XBoard shim ([HIARCS thread](https://hiarcs.net/forums/viewtopic.php?t=10577)); **retro-sargon** wraps 1978 Sargon as native UCI ([GitHub](https://github.com/billforsternz/retro-sargon)).
- **Recommended architecture:** engine speaks a trivial serial-style text protocol through the emulated console; a wrapper presents UCI to cutechess-cli/lichess-bot. Same binary can run on real hardware via a Super Serial Card later.

## 7. Opening book + endgame

- **Book:** [Polyglot .bin format](https://hgm.nubati.net/book_format.html) is 16 bytes/entry, key-sorted. On-device, use a *compiled* own format (hash-keyed, 4-6 bytes/entry, sorted, binary-searched from aux) built offline from a Polyglot book: ~10 plies of mainline coverage in 2-8KB ([CPW Opening Book](https://www.chessprogramming.org/Opening_Book)). Modest Elo, big practical value (variety, avoids early traps PSQT-only eval falls into).
- **KPK:** canonical bitbase ~12KB/side; van Kervinck's [pfkpk](https://talkchess.com/viewtopic.php?t=57517) uses 32KB with a clean generator. Sub-2KB exact tables aren't established; TalkChess consensus: **cover KPK with rules instead** ([thread](https://www.talkchess.com/forum/viewtopic.php?p=191242)). Ranking: (1) tapered PSQT endgame tables + passed-pawn bonus by rank (near-free); (2) ~100-byte rule-based KPK recognizer (square-of-the-pawn + key squares); (3) full bitbase in aux only if testing shows dropped half-points.

## Master recommendation list, ranked by Elo per (implementation cost + cycles)

1. Negamax + alpha-beta, pseudo-legal 0x88/16x12 mailbox + piece list, captures-first movegen (HGM pattern)
2. QS: all captures, MVV/LVA by generation order, stand-pat, delta pruning (~1 pawn)
3. Iterative deepening + TT/previous-best move first
4. Incremental PeSTO tapered PSQT + tempo
5. Null move R=2 with micro-Max's material-based zugzwang guard
6. Always-replace TT, 2K-16K entries in aux (~130-160 Elo even small)
7. Killer moves (2/ply) (~35-56 Elo)
8. Futility + reverse-futility at depth ≤2
9. Time mgmt: node-count budget + $C019 VBL frame counting; optional Mockingboard 6522 IRQ
10. Check extension; mate = MATE−ply with TT ply adjustment
11. Compiled 2-8KB opening book; rule-based KPK recognizer
12. Later/optional: conservative LMR, IID, pawn-structure terms with tiny pawn hash
13. Skip: history heuristic, mobility eval, checks in QS, aspiration windows, bitboards, legal movegen
