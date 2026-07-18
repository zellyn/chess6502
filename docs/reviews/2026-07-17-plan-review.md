# Adversarial review of the initial plan — 2026-07-17

Three independent critic agents attacked the plan, decision log, testing
doc, and harness/memory-model code, with distinct lenses: (A) 6502/Apple
II systems feasibility, (B) chess-engine theory and measurement
methodology, (C) tooling/process (which verified findings by actually
running the harness). Every decision D1-D11 survived; D4, D6, D8, D9, and
D11 were amended; D12 was added. Dispositions below; "fixed" means code
or docs were changed the same day.

## Blockers

| # | Finding (critic) | Disposition |
|---|---|---|
| 1 | Elo target conflated rating scales: +150-300 over a period-scale ~1600 was promised for verification against CCRL-anchored opponents at full speed. Counter-model (Belle: ~125 Elo/node-doubling at low depth, 12 doublings below micro-Max's conditions) predicts ~1250-1550 CCRL. (B) | **Fixed.** Goals split by scale: beat-the-80s headline; node-odds crossover as primary number; ~1250-1550 CCRL honest prediction; 1700-1900 valid only on the period/node-matched scale (Mephisto MM V 500-600 NPS -> 1883 SSDF proves reachability). |
| 2 | Gauntlet protocol undefined where it matters (opponent clocks); TSCP/micro-Max alone are the wrong pool if we land 200-500 below them. (B) | **Fixed.** Four-part protocol in D11: fixed-cycle SPRT with paired openings; bracketing pool (±200) at rating-valid conditions; node-odds ladder; period matches at equal emulated time. |
| 3 | Hardware stack budget self-contradictory ("~8 bytes/ply" x 64 plies = 512 bytes in a 256-byte page); no MAXPLY; unbounded check extension -> silent stack wrap. (A) | **Fixed.** Return-addresses-only hardware stack; per-ply software frames; MAXPLY 32-48 + extension budget from day one; debug headroom assert; risk-table entry. |
| 4 | Store-traps at $BFF0/$BFFF sit inside the documented engine-table and aux-TT regions and fired on aux writes too. (A and C independently) | **Fixed.** Traps now fire only on main-bank stores (code); $BF00-$BFFF reserved in both banks (memory map + D8). |
| 5 | `-org` overflow panics; overruns silently truncate; `-entry` conflates unset with $0000 and wraps silently. (C, tested) | **Fixed** in a2run: validation with real error messages. |
| 6 | `-dump` reads through `Read`, whose $C08x side effects flip Language Card state while inspecting memory. (C, tested) | **Fixed.** Side-effect-free `iie.Peek`; dump uses it. |
| 7 | Sibling-repo changes (goapple2/iie, go.mods) uncommitted; chess6502 had no commits — documented workflow reproducible on exactly one machine. (C) | **Surfaced to owner** (commits are theirs to make); listed in testing.md known gaps. |

## Majors (selected)

| # | Finding (critic) | Disposition |
|---|---|---|
| 8 | VBL edge-counting at 256-node intervals cannot count frames (poll gap 26-38 frames vs 1-bit flag); clock would run 15-30x slow on hardware. (A) | **Fixed.** D9 rewritten: cycle budgets primary; hardware polls every 1-2 nodes; $C019 sense documented; cycle-fed $C019 added to the iie model so the path is testable pre-metal. |
| 9 | Make/unmake cycle claims ~3x low (honest: quiet make ~330-380, capture ~480-540); M1 "within 2x of budget" gate had no budget. (A) | **Fixed.** Per-primitive cycle budgets published in plan.md; NPS model recomputed (~420-680, anchors intact). |
| 10 | Tapered-eval blend needs multiplies/divide-by-24 the CPU doesn't have (~300-450 cycles/eval, unbudgeted). (A) | **Fixed.** /32 phase rescale + convexity (min/max bounds) shortcut designed in; budgeted. |
| 11 | Fixed-node SPRT is blind to cycle-cost features (futility's whole point) and free-passes per-node cost increases. (B) | **Fixed.** All testing moved to fixed emulated-cycle budgets. |
| 12 | Deterministic engines from startpos make N-game matches one photocopied game; no openings suite anywhere. (B) | **Fixed.** Paired openings with color reversal mandatory, wired at M3. |
| 13 | QS omitted promotions (blind to queening at the horizon); delta pruning unsound at low phase; stand-pat while in check. (B) | **Fixed.** Queen promotions in QS with adjusted margin; delta off at low phase; in-check evasions. |
| 14 | Unvalidated TT move (~0.3-0.6 false verify-matches/game) desyncs piece lists/accumulators on a 6502 — collision risk is crash-shaped, not Elo-shaped. Killers too. (B; A concurred) | **Fixed.** Mandatory pseudo-legality validation (D6); 20 verify bits; power-of-2 sizing only (mod-5888 indexing needs division; 5888 also double-booked the book/KPK space). |
| 15 | Fixed per-ply move lists at "~70 bytes/ply" overflow in routine middlegames (Kiwipete: 48 legal at root). (A and B) | **Fixed.** Contiguous move stack + per-ply base pointers + overflow guard. |
| 16 | PVS silently dropped despite +55 measured evidence; plan's own LMR condition referenced "non-PV" with no PV concept. (B) | **Fixed.** PVS added to M5 measured list. |
| 17 | Repetition semantics unspecified against the TT (probe ordering, storing draw scores, path vs game history, truncated-hash false positives). (B) | **Fixed.** Search #8 spells out: path+history check before TT probe; rep draws not stored; full-key verification. |
| 18 | Trace output interleaved with program stdout mid-line. (C, tested) | **Fixed.** go6502 trace -> stderr. |
| 19 | Hardcoded audit.lst symbol addresses go stale silently. (C) | **Fixed.** Parsed from ACME `--symbollist` at test time. |
| 20 | D4 gaps: aux primitives' data reachability unstated; ALTZP toggling from LC code self-destructs. (A) | **Fixed.** ZP-only calling convention; "ALTZP never touched" stage-1 invariant. |
| 21 | RAMRD-on makes main $0200-$BFFF unreadable (fetches AND data) — confirmed correct in plan; monitor ROM unmapped while LC enabled affects M8 UI. (A) | **Fixed.** M8 milestone drives $C000/$C010 + text page directly. |
| 22 | No Makefile/toolchain pinning; no multi-segment linker story before M1 needs it. (C; A concurred) | **Partially fixed.** Makefile with version warning landed; multi-segment cfg + loader scheduled explicitly in M1. |
| 23 | M2 "beats random mover 100-0" trivial-yet-brittle; M4 aggregate +150 mis-shaped. (B) | **Fixed.** Mate suites + >=97.5% + legality torture; per-feature SPRT gates. |
| 24 | Insufficient-material handling at M6 is too late (shuffle-draws pollute M3-M5 measurements). (B) | **Fixed.** Moved to M2. |
| 25 | Speed claims (168x/170x) single-anecdote, workload-dependent (2x on tiny runs); "500 games under an hour" ignores opponent clocks. (C, tested; B concurred) | **Fixed.** Claims softened and corrected; benchmark rig scheduled for M1. |

## Notable attacks that did not land

- Null move R=2 with material zugzwang guard at depth 6-8: sound.
- Skipping SEE, history heuristic, checks-in-QS, aspiration windows,
  mobility: supported by evidence at these depths.
- 0x88 + piece lists direction: survives; M1 spike must price list
  bookkeeping in both arms (now in D5).
- Aux TT vs main-LC TT: aux wins on the math (~100 cycle/probe tax vs
  ~16 Elo for two capacity halvings).
- D2 (NMOS-only): survives a proper steelman — honest 65C02 ceiling is
  ~8-15% ≈ 12-17 Elo, and functional (Dormann) but not gate-level
  verification would exist; compatibility + no-second-core decides it.
- iie Language Card prewrite semantics: verified correct against Sather
  by critic A (and hardware-verified via a2audit).
