# Research: prior art — 6502 chess engines and Elo baselines

*Web research conducted 2026-07-17 (agent-assisted; 6 of 7 spot-checked
claims confirmed by independent sources). Preserved as input to
docs/plan.md; URLs are the citations.*

## 1. Historical baselines: Sargon and the 1980s 6502 field

### Sargon I (1978)
- Written by Dan & Kathe Spracklen in **Z80 assembly** (Wavemate Jupiter; the Spracklens later called the 6502 "the much better processor" for their program — RISC-like, ~1 µs instructions, fast branches). https://en.wikipedia.org/wiki/Sargon_(chess), https://www.chessprogramming.org/Sargon, https://www.chessprogramming.org/6502
- Internals: fixed-depth **full-width minimax with alpha-beta, no quiescence**, SOMA-like exchange evaluation applied at the horizon; **10x12 mailbox board** (120-byte array, 0xFF borders); selectable up to 6 ply, but practical play was ~2 ply. Full commented listing published as *Sargon: A Computer Chess Program* (Hayden, 1978). https://www.chessprogramming.org/Sargon, https://www.chessprogramming.org/10x12_Board, https://github.com/billforsternz/retro-sargon, https://archive.org/details/sargon_202108
- Strength: **~1200 Elo** (Bill Forster's estimate after resurrecting the exact book code as a UCI engine). His x86 conversion needs ~30 s for a 6-ply middlegame search and runs "two to three orders of magnitude faster" than the original hardware — i.e., 6-ply full-width took hours in 1978. https://github.com/billforsternz/retro-sargon

### Sargon II / 2.5 (1979-80)
- Ported to Apple II and other 6502 machines; BYTE estimated ~1500 at tournament settings. The 6502-based Chafitz ARB "Sargon 2.5" earned **real human-tournament performance ratings of 1641** (Paul Masson, 1979) **and 1736** (San Jose CC Open, 1980) — verified. https://en.wikipedia.org/wiki/Sargon_II_(video_game), https://www.chessprogramming.org/Chafitz_ARB_Sargon_2.5
- Per-level times: ~10 s/move (est. ~1000 Elo level) up to 45 min/move (claimed >1800; independent assessments closer to ~1500). https://www.schach-computer.info/wiki/index.php/Chafitz_ARB_Sargon_2.5
- A fully annotated **6502 disassembly of Sargon II (VIC-20)** exists: https://github.com/jamarju/vic20-sargon-ii-chess

### Sargon III (1983/84)
- Complete rewrite **in 6502 assembly**: replaced the exchange evaluator with **quiescence search**, added a **transposition table with BCH hashing** and an opening book (~68,000 positions advertised). 9 levels, 5 s to 10 min/move. https://www.chessprogramming.org/Sargon, https://en.wikipedia.org/wiki/Sargon_III, https://www.spacious-mind.com/html/commodore_c64_sargon_iii.html
- Strength range (report as a spread): **~1550-1600 measured** on 1 MHz C64 retro round-robins; **~1750 USCF claimed** on faster hardware (unverified forum figure); **2200 was marketing**. https://www.spacious-mind.com/html/commodore_c64_sargon_iii.html, http://chesstroid.blogspot.com/2014/05/commodore-64-revisited-retro-chess.html, https://www.chess.com/forum/view/help-support/sargon-3-vs-fidelity-par-exellence

### Other 80s 6502 programs
- **MicroChess** (Jennings, 1976, KIM-1): 1KB total, **piece lists instead of a board array**, minimax, no castling/e.p./promotion in v1.0. Claimed "~1100"; a modern port measured **573 Elo on CCRL 40/4** — huge claimed-vs-measured gap. http://www.benlo.com/microchess/index.html, https://www.chessprogramming.org/MicroChess, https://www.chess.com/blog/BenRedic/retro-computer-chess-part-3-bringing-it-home
- **MyChess** (Kittinger, 1979, originally Z80): iterative full-width alpha-beta with **killer + capture heuristics**, capture extension at horizon, two 16-byte piece lists. https://www.chessprogramming.org/MyChess
- **Chessmaster 2000** (1986): Kittinger engine derived from his Novag Constellation 6502 line; measured **~1552** on C64. https://www.chessprogramming.org/Chessmaster, http://chesstroid.blogspot.com/2014/05/commodore-64-revisited-retro-chess.html
- **Chess 7.0** (Larry Atkin, 1982, 6502 assembly for Apple II): from the Northwestern Chess 4.x lineage; no published Elo; beat a 1965-rated player in a demo postal game. https://www.chessprogramming.org/Chess_7.0, https://en.wikipedia.org/wiki/Chess_7.0

## 2. The true bar to beat: dedicated 6502 chess computers (SSDF-rated)

Critical context: the **SSDF scale was repeatedly deflated** (~70 pts in 1989, another **-100 in 2000**), so every machine has two canonical numbers: the Dec-1996 list and the current list (~90-100 lower). https://www.chessprogramming.org/SSDF, https://www.oocities.org/marochess/ssdf/1996/ssdf9612.htm, http://ssdf.bosjo.net/long.txt

Attribution corrections vs. common folklore: Richard Lang's championship Mephistos were **68000/68020, not 6502**; the strong 6502 Mephistos are **Ed Schröder's** Rebel line (65C02) plus Ulf Rathsman's MM II; Fidelity's Designer 2325 and Elite Avant Garde 2265/2325 were 68020/68000 — Fidelity's 6502 era ends around 1987-88. https://www.chessprogramming.org/Mephisto, https://www.chessprogramming.org/Ed_Schroder, https://www.chessprogramming.org/Fidelity_Electronics

Headline results (all verified against SSDF lists):
- **Strongest 6502 ever: Mephisto MM IV + 16 MHz Turbo Kit — SSDF 2092 (1996 list) / 2001 (current)**. It was **#1 on the entire SSDF list at end-1988 with 1993** — the last time a 6502 led the world rankings. https://en.wikipedia.org/wiki/Swedish_Chess_Computer_Association, https://www.oocities.org/marochess/ssdf/1996/ssdf9612.htm
- **Strongest standard production unit: Mephisto Polgar 10 MHz (Schröder) — 2042 / 1949** (verified on both lists). https://www.chessprogramming.org/Mephisto_Polgar, http://ssdf.bosjo.net/long.txt
- **Fidelity's best 6502**: Par Excellence (5 MHz, 1986) — SSDF **1835 / 1745**, but **officially rated 2100 by the USCF Computer Rating Agency** (the same source lists FIDE-est. 1963 — the 2100 is the agency number, not SSDF strength). https://www.chessprogramming.org/Par_Excellence, https://www.spacious-mind.com/html/par_excellence.html
- **Novag's best**: Super Expert C (65C02 6 MHz, 1990) — **1961 / 1868** (verified); the Super Constellation (4 MHz, 1984) topped the very first SSDF list at 1631 (1984 scale) and scored a 1981 performance at the 1983 U.S. Open. https://www.schach-computer.info/wiki/index.php/Novag_Super_Expert_C, https://www.chessprogramming.org/Super_Constellation

**Bottom line: the best 5-10 MHz 65C02 machines reached ~1950-2050 (1996 SSDF scale), ~1870-1950 on today's scale. Scaled to 1.023 MHz, no 6502 program at your clock ever exceeded roughly 1550-1700.** That's the realistic bar.

## 3. Nodes-per-second reality check on ~1 MHz 6502

Documented data points (machine, clock, NPS):

| Program / machine | CPU/clock | NPS | Source |
|---|---|---|---|
| Colossus Chess 2.0 (C64) | 6510 ~1 MHz | **520 avg** (official manual; uncorroborated but primary) | https://www.lemon64.com/doc/colossus-chess-2/138 |
| Colossus (ZX Spectrum) | Z80 3.5 MHz | 170 | https://en.wikipedia.org/wiki/Colossus_Chess |
| Mephisto MM V (Schröder) | 65C02 5 MHz | 500-600 | https://www.chessprogramming.org/Mephisto_MM_V |
| Tandy 2150 (Kaplan "B") | 6502 3 MHz | 95 | http://tibono.free.fr/Echiquiers_moyens_eng.html |
| Saitek Turbo King II ("D+") | 6502 5 MHz | ~310 | http://tibono.free.fr/Echiquiers_forts_eng.html |
| Mephisto III (ultra-selective B-strategy) | 6502 3.7 MHz | 1-10 (!) | https://www.hiarcs.net/forums/viewtopic.php?t=3144 |
| Chess-Master (Sargon 2.5 clone, 6502 emulated on Z80) | 2.5 MHz | 12-15 | https://www.chessprogramming.org/Chess-Master |
| Hobbyist rule of thumb | 6502 1 MHz | ~500 | https://stardot.org.uk/forums/viewtopic.php?t=11884 |

Synthesis: at 1.023 MHz expect **~300-600 NPS with a lean assembly engine and material+PST-class eval** (Colossus is the proof point at 520), dropping to **~30-100 NPS with knowledge-heavy evaluation** (Kaplan/Schröder programs ran 30-120 NPS/MHz). Sargon, MicroChess, and the Fidelity units have **no documented NPS anywhere** — the "Super Constellation ~500 nps" figure circulating in forums is folklore with no citable source. Node-counting conventions differ across sources. https://www.chessprogramming.org/Nodes_per_Second

At 500 NPS and 10-60 s/move you get 5K-30K nodes/move — with good pruning that's ~6-9 ply nominal depth, which historical evidence (talkchess "Vintage Chess Programming": >3 ply full-width was hard; 256-entry TTs useful at ~400 nps) says is attainable only with modern selectivity. https://talkchess.com/viewtopic.php?t=40674

## 4. Modern retro engines (post-2000)

The striking finding: **nobody has yet built a serious maximum-strength 6502 engine** — the niche is genuinely open.
- **maksimKorzh/6502-chess** (2022, KIM-1): 909 bytes, depth-3 search, ~1 min/move — a minimalism exercise, not strength. https://github.com/maksimKorzh/6502-chess, https://talkchess.com/viewtopic.php?t=80085
- **Toledo Atomchess-6502** (Atari 2600, 1K ROM, 128 B RAM): 2-ply search, no move validation. https://github.com/nanochess/Atomchess-6502
- **StewBC/cc65-Chess** (C64/Apple II, cc65 C): author says "the AI is not very good." https://github.com/StewBC/cc65-Chess
- **Toledo Nanochess** (C, 1274 chars): alpha-beta + quiescence, self-reported **~1400-1600 Elo** (1429 at ChessWar). https://nanochess.org/chess2.html
- **micro-Max 4.8 / Fairy-Max** (H.G. Muller, ~2000 chars of C): the strength benchmark for minimal engines — **CCRL 1868-1954 depending on list (1891±28 on 40/40, verified)**. Includes: negamax alpha-beta, **recapture-only quiescence, IID with best-move-first, hash table (score+best move), null move, futility, full FIDE rules** (except underpromotion); excludes SEE, killers, book. Board: 0x88. https://home.hccnet.nl/h.g.muller/max-src2.html, http://ccrl.chessdom.com/ccrl/4040/cgi/engine_details.cgi?eng=Micro-Max+4.8
- Best "modern C engine on 8-bit" datapoint: **micro-Max 1.6 compiled via z88dk for 3.5 MHz Z80** — hash table dropped (won't fit 64K), deepening loop truncated for response time; 12.3KB binary. https://github.com/z88dk/z88dk/wiki/umax_chess
- No 6502/Z80 ports of TSCP or Sunfish exist. Z80 scene is byte-golf (ChesSkelet 269-377 bytes; 1K ZX Chess 672 bytes) — no measured Elo, novice-level play. https://www.chessprogramming.org/TSCP, https://spectrumcomputing.co.uk/entry/34814/ZX-Spectrum/ChesSkelet, https://en.wikipedia.org/wiki/1K_ZX_Chess

## 5. Which techniques pay at 4-9 ply and tiny hash

1. **Quiescence + MVV-LVA + PSTs**: MinimalChess 0.2→0.3 jumped **909→1439 CCRL (~+500)** from exactly this. https://github.com/lithander/MinimalChessEngine
2. **Transposition table, even tiny**: Rustic measured **+42 from cutoffs, +103 from TT move ordering (~+145 total)**; in Stockfish, 75% of hash-node cutoffs come from the hash move — the *ordering* value survives tiny tables. Muller's rule: **~8 Elo lost per halving** of hash below tree size; Stockfish fishtest 128MB→4MB cost only ~14 Elo at blitz. A few-KB table on the IIe costs tens of Elo vs. ideal, not hundreds — and 64K of aux RAM is a luxury no 1980s 8-bit engine had. https://rustic-chess.org/progress/playing_strength.html, https://www.chessprogramming.org/Transposition_Table, https://talkchess.com/viewtopic.php?t=54204&start=10
3. **Null-move pruning**: **+100-150 when added first** (one measured +144±64 after fixing a bug that initially made it -16 — very bug-sensitive); Stockfish loses ~800 without it. Guard zugzwang — shallow searches hit endgames. https://talkchess.com/viewtopic.php?t=81949, https://talkchess.com/viewtopic.php?t=82539, https://www.chessprogramming.org/Null_Move_Pruning
4. **Killers**: **+35-56** (Rustic); Muller: "killer always helped my engines a lot. (In contrast to history.)" https://talkchess.com/viewtopic.php?t=77734
5. **Futility pruning**: operates at depth 1-3 — i.e., our entire tree; in micro-Max 4.x. https://www.chessprogramming.org/Futility_Pruning
6. **PVS**: +55 (Rustic). https://rustic-chess.org/progress/playing_strength.html
7. **LMR**: +30-90 but standard conditions disable it below depth 3 and it needs decent quiet-move ordering — marginal but positive at our depths (MinimalChess got ~90 self-play). https://talkchess.com/viewtopic.php?t=78397
8. **History**: mixed — unlocked LMR in some engines, useless in Muller's. **Aspiration windows: ~0-15, sometimes negative — skip.** (MadChess gained +9 by *removing* them.) **SEE: no published Elo data; micro-Max/TSCP/Sunfish all omit it.** https://talkchess.com/viewtopic.php?t=76115
9. Eval quality dominates late: Rustic's tapered+tuned eval was worth **+248** — the single biggest item. https://rustic-chess.org/progress/playing_strength.html

Calibration ladder: TSCP (no TT, no null move) ≈ **1598-1724 CCRL**; micro-Max 4.8 (TT+null+futility, crude eval) ≈ **1868-1954**; MinimalChess 0.6 (all of the above + LMR) ≈ **2443** on fast hardware. https://computerchess.org.uk/ccrl/402.archive/cgi/engine_details.cgi?print=Details+(text)&eng=TSCP+1.81

## 6. Board representation on the 6502

- **0x88 is the consensus modern choice for 6502**: off-board test is one `AND #$88 / BNE` on the accumulator — no memory read, no table; square differences fit a 256-entry (one-page) attack/direction table, perfect for 8-bit indexing. Used by micro-Max and Toledo's assembly Atomchess line. Kittinger used a 0rrr1ccc variant at Novag explicitly to use "1/2 a page of RAM efficiently." https://www.chessprogramming.org/0x88, http://home.hccnet.nl/h.g.muller/board.html
- **Sargon used 10x12 mailbox** (120 bytes, 0xFF borders) — sentinel reads are also cheap on 6502 via indexed addressing, so this remains viable. https://www.chessprogramming.org/10x12_Board
- Pair either with **piece lists** (MicroChess/MyChess tradition) so movegen iterates ≤16 pieces, not 64+ squares. https://www.chessprogramming.org/Piece-Lists
- **Bitboards: confirmed infeasible/pointless on 8-bit.** Every 64-bit op becomes 8+ byte ops; 6502 shifts one bit at a time. Steven Edwards estimated emulated bitboard code would manage "only a few evaluations per second." https://www.talkchess.com/forum3/viewtopic.php?t=14610, https://www.chessprogramming.org/Bitboards

## 7. Assembler choice (2025-26)

- **ca65/cc65 — recommended.** Actively developed (commits through July 2026, though last tagged release is V2.19/2020 — use snapshot builds); ld65 fully supports raw binaries at fixed addresses; stock `apple2-asm.cfg`; Apple II libs maintained by Oliver Schmidt; flagship Apple II projects (a2stuff/a2d Desktop) use it. https://github.com/cc65/cc65, https://cc65.github.io/doc/ld65.html, https://cc65.github.io/doc/apple2.html
- **Merlin32**: vintage-syntax choice, alive but fragmented across forks. https://brutaldeluxe.fr/products/crossdevtools/merlin/
- **64tass**: very actively maintained but C64-centric. **ACME**: last release 2020, still used by 4am's Total Replay.
- Comparison writeup: https://www.lonsteins.com/posts/apple2-choosing-an-assembler/

## Summary table: strongest 6502-family chess implementations

| Implementation | CPU / clock | Elo (source/scale) |
|---|---|---|
| **Mephisto MM IV + Turbo Kit** (Schröder) | 6502 / 16 MHz | **2092** (SSDF '96) / 2001 (current); SSDF world #1, end-1988 (1993) |
| **Mephisto Polgar 10 MHz** (Schröder) | 65C02 / 10 MHz | 2042 (SSDF '96) / 1949 (current) |
| Mephisto MM V (Schröder) | 65C02 / 5 MHz | 1977 / 1883 — at **500-600 NPS** |
| Novag Super Expert C (Kittinger) | 65C02 / 6 MHz | 1961 / 1868 |
| Saitek Leonardo Maestro B | 6502 / 18 MHz | 1928 (SSDF '96) |
| Mephisto MM IV (stock) | 65C02 / 5 MHz | 1903 / 1814 |
| Fidelity Par Excellence (Spracklen) | 6502 / 5 MHz | 1835 / 1745 SSDF; USCF-CRA 2100 |
| Fidelity Excellence | 6502 / 3-4 MHz | 1757-1800 / 1668-1711 |
| Novag Super Constellation (Kittinger) | 6502 / 4 MHz | 1731 ('96) / 1640; 1981 US Open TPR |
| Mephisto MM II (Rathsman) | 65C02 / 3.7 MHz | 1773 / 1682 |
| Sargon 2.5 / Chafitz ARB (Spracklen) | 6502 / ~2 MHz | TPR 1641-1736 (human tournaments) |
| Sargon III (Spracklen), C64/Apple II | 6502 / 1 MHz | ~1550-1600 measured; ~1750 claimed |
| Colossus Chess 2.0-4.0 (Bryant), C64 | 6510 / ~1 MHz | ~1500s class; **520 NPS** documented |
| micro-Max 4.8 (modern minimal benchmark, not 6502) | — | 1868-1954 CCRL (1891±28 on 40/40) |

**Strategic takeaway**: At 1 MHz, period software topped out ~1550-1600 measured; the ~2000-Elo 6502 machines needed 5-16 MHz. Colossus proves ~500 NPS at 1 MHz with iterative deepening + TT + killers + null move + futility already in 1984-85. A modern design targeting micro-Max's technique set but in hand-optimized assembly at ~500 NPS should plausibly land in the **1700-1900 range at 10-60 s/move** — clearly above anything that ever ran at 1 MHz, which would demonstrate exactly the theory-advance thesis. The niche is empty: no post-2000 project has attempted a maximum-strength 6502 engine.
