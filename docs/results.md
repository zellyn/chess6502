# Match & measurement log

Newest first. Engine budgets are emulated time (1.0205 MHz); opponent
controls are wall time. See docs/plan.md for the measurement protocol.

## 2026-07-19 — Texel corpus diversification (task #23 remainder)

Folded the 210-game rating-pool gauntlet (non-self-play: vs TSCP-d3,
FairyMax, NEG, minnow, SF-n10/n100/n1000; tools/pgn/pool_c96f604_*.pgn)
into the Texel corpus. New pipeline: `mirror pgnrows` extracts quiet,
non-check, labeled positions from PGNs (mirror.PGNSamples in pgn.go,
honoring [FEN] setup headers from openings-pool.epd); LoadRows now reads
gzip transparently. 210 games → 7,706 quiet rows on top of 101,202
self-play rows = 108,908-row diversified corpus
(testdata/texel-rows-2026-07-19.gz).

Re-tune (K 0.80→0.85, loss 0.1005→0.1037 — pool labels are noisier/more
decisive). Bootstrap B=200 95% CIs on the combined corpus, flagging any
weight whose old self-play value falls outside the CI:

| weight   | self→comb | 95% CI      | moved? |
|----------|-----------|-------------|--------|
| doubled  | 12 → 14   | [11, 18]    | no     |
| isolated | 10 → 7    | [6, 9]      | **yes**|
| passed2  | 18 → 15   | [9, 23]     | no     |
| passed4  | 33 → 21   | [16, 27]    | **yes**|
| passed5  | 62 → 50   | [44, 58]    | **yes**|
| passed6  | 69 → 52   | [46, 63]    | **yes**|
| passed7  | 28 → 20   | [15, 31]    | no     |
| shield   | 3 → 2     | [0, 3]      | no     |
| openfile | 4 → 3     | [0, 9]      | no     |

Real movement: the advanced passed-pawn bonuses (ranks 4/5/6) drop
~20-25% and the isolated penalty drops 10→7. Interpretation: self-play
overvalues passed pawns (both sides push them symmetrically, so the
label correlation inflates the bonus); diverse real-opponent games
correct it downward. Validation A/B (depth 6, 200 games, diversified
weights vs old self-play-tuned): **+21 ± 39** (+71 =70 −59, 53.0%) — the
self-play match environment has home-field bias toward the incumbent, so
neutral-to-positive here is a genuine (if sub-significant) endorsement.

**Adopted** the diversified set as mirror `TunedWeights`
(D14 I7 P[15,0,21,50,52,20] S2 O3; old set kept as
`SelfPlayTunedWeights`). Asm-side port target updated; final
confirmation is a rig-side pool match (deferred, not mirror work).

## 2026-07-19 — mirror verdicts: futility, LMR sweep, QS-shape (3 tasks)

Three mirror A/B campaigns (internal/mirror/ab_test.go), all depth-6,
node counts vs the current-rules base of 922,898 on the mirror bench
set (qs = 819,637 = 89% of nodes). Matches are mirror self-play at
depth 6, ~200-800 games, Elo vs the current-rules baseline. **Bottom
line: two "obvious" improvements are duds, one QS knob is a keeper.**

**1. Futility mate-zone guard fix (task #27) — DO NOT PORT.**
The asm's futility/RFP guard uses an unsigned compare, so futility is
silently disabled in every negative-alpha/beta window (the same bug
class as the old null-move one). Fixing it (signed-aware, futility
active in those windows) cuts −20.7% nodes — but costs strength:
- 800-game match, fix=true vs current: +229 =295 −276, 47.1%,
  **−20.4 ± 19.2 Elo**. Four smaller 200-game runs agree (−31, −23,
  −17, −10). The extra pruning in negative windows is cutting real
  moves. **The "bug" is protective; leave the asm as-is.** This
  retires the held asm-side futility signed-compare fix.

**2. LMR/PVS parameter sweep (task #28) — no porting winner.**
Swept lateness {2,3,4}×{5,6,8}, R floors, reduce-killers, evasion-PVS.
Node cuts are large; strength is flat-to-negative everywhere:
- `2,6,2,4+killers` −53.4% nodes; `3,6,2,4` −48.6%; `rem1=2` −40.5%.
- But matches (±38 Elo, ~200 games each): `4,8,3,5,0,1` −0,
  `4,7,3,4,0,1` −2 / −14, `3,6,3,5,1,1` −14 / −37, `4,7,3,5,0,0` −26,
  `4,7,2,5,0,1` −44. Nothing beats current rules; the deep node-cutters
  lose Elo. **Current LMR is already well-tuned — no change to port.**

**3. QS-shape experiments (task #29, COMPLETE) — recap2 is the keeper.**
Deep-QS knobs: PlyCap (force stand-pat past N qs plies, evasions
exempt) and RecapAfter (past N qs plies, captures only onto the
previous move's TO square). Node cuts (total / qs):
- `recap1` −35.5% / −39.4% · `cap2` −29.3% / −32.7% · `recap2`
  −21.4% / −24.4% · `cap4` −20.0% · `cap6` −13.2%.
- Matches, depth 6, 200 games (100 color-swapped pairs), Elo vs the
  current-rules (uncapped-QS) baseline, all seed 6502:
  - `recap1` (qs 0,1) = **−123 ± 42, catastrophic** (chops the
    recapture tree one ply too early).
  - `recap2` (qs 0,2) = **−12 ± 39** (+61 =71 −68, 48.2%) — confirms
    the prior −10 ± 38 run; statistically neutral at −21%/−24% nodes.
  - `cap2` (qs 2,0) = **−113 ± 40** (+34 =69 −97, 34.2%) — a dud,
    nearly as bad as recap1. Forcing stand-pat at qs ply 2 blinds QS
    to deeper captures/recaptures; the PlyCap knob only pays off much
    deeper (cap6/cap8 barely cut nodes, so there's no useful window).
- **Winner: recap2 (qs=0,2), no combo.** A cap+recap blend is not
  worth pursuing — the only cap shallow enough to save real nodes
  (cap2) is catastrophic, and recap2 already delivers the full −24% QS
  saving at neutral strength on its own. recap2 is the one portable
  lever of the three campaigns. Asm port (a RecapAfter gate in
  generateq/qsearch — capture onto undo[ply-1].to only, past 2 qs
  plies) deferred to a careful inner-loop QS-generation pass.

## 2026-07-19 — first full rating-pool gauntlet (standing scoreboard)

runs/pool.sh @c96f604: 30 games per opponent, 30s emulated/move with
-dither -bank, paired colors from tools/openings-pool.epd (TSCP runs
bookless — xboard v1, no setboard, forfeits on book positions; its
variety comes from -dither). PGNs in tools/pgn/pool_c96f604_*.pgn.

| Opponent  | Result (W-L-D) | Score | Elo diff (logistic) |
|-----------|----------------|-------|---------------------|
| NEG       | 30-0-0         | 100%  | (unbounded +)       |
| minnow    | 30-0-0         | 100%  | (unbounded +)       |
| SF-n10    | 6-23-1         | 21.7% | −223                |
| SF-n100   | 2-26-2         | 10.0% | −382                |
| FairyMax  | 1-27-2         | 6.7%  | −458                |
| TSCP-d3   | 1-29-0         | 3.3%  | −585                |
| SF-n1000  | 0-30-0         | 0%    | (unbounded −)       |

Anchoring, with caveats stacked high: Fairy-Max is CCRL ~1890, so the
−458 puts us at ≈**1430** — but Fairy-Max here plays st=2 wall (below
its CCRL time control, so the true gap is larger than the CCRL number
implies), and n=30 gives roughly ±120 Elo bars per match. TSCP-d3
implies lower still, but depth-3-capped TSCP has no published rating
(full TSCP is ~1700). Treat ≈1400±150 as the first honest anchor and
the SF node ladder (21.7% / 10% / 0%) as the sensitive internal
yardstick — node-limited Stockfish is perfectly reproducible, so
future builds move that ladder or they didn't get stronger.

Notes: TSCP-d3 3.3% vs the banked-time 11.7% two entries down is n=30
noise plus opening-variety differences (both bookless, different
dither seeds) — the pool number is the standing-conditions one going
forward. The 180 pool PGNs are the first non-self-play Texel corpus
material (diversification queued, task #23 remainder).

## 2026-07-19 — banked time: first real move in the TSCP needle

Chess-clock banking rig-side (chesstest.BankedClock, bridge -bank
flag): unused per-move cycles carry forward, each move spends
base + bank/8, bank capped at 8x base, total game time conserved
(protocol (c) comparability). Predictive iteration gating is the
enabler — honest early stops on easy moves now fund extra iterations
on hard ones. 6502 driver port noted (24-bit zp bank, /8 = shifts).

- Diagnostic 60-ply carryover game, realized depth: unbanked
  d1:11 d2:30 d3:16 d4:3 → banked d1:3 d2:20 d3:22 d4:7 d5:2.
  Depth-1 emergency stops nearly gone; depth >= 4 tripled.
- **TSCP-d3, 30 games, -dither -bank: 1-24-5 = 11.7%** — the best
  score yet against it (historical band 3.3-6.7%), and the first
  post-change match to move in the predicted direction. n=30 keeps
  the error bar wide (~±6%); the queued rating-pool gauntlet will
  firm it up. Next depth levers: the mirror's LMR-parameter ranked
  table, then its QS-shape verdicts.

## 2026-07-19 — pawnterm rank-bitmask + two latent bugs + Texel weights

The restructure (per-file rank-occupancy bytes + gentables lookup
tables; scratch $0200-$020F) was gated on exact parity — and the gate
earned its keep twice before passing:

- **h-file isolated bug (old code)**: ptneighw/b returned the flags of
  `cpx #7` (Z set) on the file-h path, so h-file pawns with a g-file
  neighbor were scored isolated anyway. Symmetric ±12 wash in balanced
  positions, real error otherwise. Fixed (`ora #0`, commented
  load-bearing).
- **king-shield scratch clobber (old code)**: ptsuba stomped EVTMP,
  which the shield loops use for the king file, so the black shield
  read past the array (into stale PWMAX bytes) after its first term.
  Fixed (ptsuba uses MULCNT).

With buggy-old off the table as a reference, the gate is now a Go
model of the intended semantics: TestPStructParity, asm == model over
4,045 random-game positions, exact. QS profile: pawnterm's share of
total cycles 15.7% → 12.9% (the 32-slot piece-list scan still
dominates its cost).

**Texel-tuned weights** (from the mirror, task #20: +18 ± 17 vs
no-pstruct over 800 mirror games): isolated 12→10 (split from doubled,
which stays 12), PASSEDBONUS → [0,18,0,33,62,69,28,0], shield 8→3,
open king file 10→4. Asm-side 50-pair SPRT 0x1F vs 0x17: −7 ± 50 —
under-powered at 100 games, consistent with the mirror's CI; the
mirror number is the load-bearing one.

## 2026-07-18 — time-management campaign, items 1-2 (gating, generateq)

1. **Predictive iteration gating + mate-stop** (driver): start iteration
   N+1 only if now + 2x(last iteration's cost) fits the budget; stop
   deepening once a winning mate is exact. Diagnostic game: hard aborts
   13 → 11 of 60 moves, avg think 37M → 32M cycles at the same realized
   depths; mate-in-2 budget search 20M → 2.6M cycles. TSCP-d3 at 30
   games: 0-28-2 (3.3%) vs the 5.0-6.7% band of the previous three
   matches — within noise, and as predicted the honest stops don't gain
   strength at a fixed per-move budget: the savings must be BANKED
   (carry unspent cycles into later moves) to convert into Elo. Banked
   time is the queued follow-up.
2. **generateq** (compile-time captures-only movegen copy, GENCAPS
   retired): behavior-identical (perft/WAC/torture/ckverify green),
   honest measurement only ~0.5% at fixed depth on capture-dense
   positions — the win is structural plus a few percent on quiet ones.
3. **QS profile** (new TestQSProfile): qs = 85-93% of all cycles;
   pawnterm ≈ 13-14% of total (retriggered by every qs pawn capture);
   movegen walks 9-23%. pawnterm rank-bitmask restructure is item 3.

## 2026-07-18 — the "LMR depth collapse" that wasn't; honest abort reporting

The first post-LMR TSCP-d3 match (2-28-0, pgn/…_lmr.pgn) looked like a
regression: PGN depths showed 2-3 where the pre-LMR match had shown 4.
Diagnosis with the new PC-cycle profiler (harness.RunProfile /
chesstest.RunProfiled) and a per-move depth-logging carryover game
(internal/sprt TestDebugDepthGame) found **no regression**:

- Head-to-head on identical positions/seeds, the f2-era build realizes
  depth 1-3 in middlegames at 30 emulated s — same as current; the
  current build hard-aborts *less* (4 vs 9 times in the 60-ply
  diagnostic game). Cold budget searches: 0x1F ≈ 0x0F, sometimes
  cheaper. Dither: no effect (seed 0 vs 17 within noise). No re-search
  storms in the profile.
- The "depth 4" belief came from two reporting bugs: on hard abort the
  driver reported CURDEPTH = the *aborted* iteration (one more than
  completed) and left the abort dummy score in SCORE — hence PGN lines
  like "0.00/5" while dead lost. Both fixed: the driver now restores
  the last completed iteration's score (PREVSC0/1) and decrements
  CURDEPTH on the abort-fallback path.
- Match scores agree: f2-era 0-18-2 (5.0%), first LMR match 2-28-0
  (6.7%), post-fix rerun 0-27-3 (5.0%, pgn/…_lmr2.pgn). All the same
  within noise — TSCP-d3 simply outclasses us at this budget.
- −87% fixed-depth win re-confirmed post-fix: 0x0F 5,502M vs 0x1F
  718M at depth 6.

What the diagnosis exposed as the REAL costs (next levers):
1. **QS is the iteration floor**: at CURDEPTH 2-3, 60-95% of cycles sit
   at plies 5-14 — quiescence trees, which LMR cannot touch. Iteration
   costs are QS-dominated and erratic; that's why realized depth stalls
   at 2-3 regardless of full-width improvements. Candidates: qs ply
   cap, deep-qs recapture-only, SEE-ish pruning.
2. **Hard aborts waste ~45M cycles** on ~25% of middlegame moves (2x
   budget spent, aborted iteration discarded). Predictive iteration
   gating (spent + est(next) vs limit) would reclaim most of it.
3. **pawnterm is ~9-12% of total cycles** in real games — promotes the
   rank-bitmask restructure from "nice to have" to "next batch".

## 2026-07-18 — PVS + LMR (FT_LMR): −87% depth-6 tree

PVS zero-window scouts after the first legal move, with LMR on late
quiets (depth−1 at >= 3 moves searched and remaining >= 3; depth−2 at
>= 6 and remaining >= 5; never in check, never for checking moves,
never at the root). Fail-high scouts re-search: reduced → unreduced
zero-window → full window when open. Depth-6 fixed-tree cycles:

| features | cycles |
|----------|--------|
| 0x00     | 6,509M |
| 0x07     | 3,757M |
| 0x0F     | 5,498M |
| 0x1F     | **718M** |

−87% vs 0x0F; masks without FT_LMR moved <1% (mode-select overhead
only — gated-off behavior is bit-identical). Self-play 0x1F vs 0x0F,
50 pairs at 30 emulated s: +37 =31 −32, **+17 ± 57 Elo** — positive,
inside noise, as expected where the budget sits between depth
thresholds; the tree cut is the durable win (roughly two extra plies
at fixed cycles, EBF ~6.5 → the same budget now reaches depth 5-6
where it reached 4). WAC stays 6/7 at depth 4 (b3b2 remains the
historical miss); root reductions were tried and reverted — WAC.001's
quiet mate move g3g6 was reduced into a fail-low at the root, which is
why the root scouts but never reduces. Mate suite exact throughout
(re-searches rescue reduced mates). Next: rematch TSCP-d3 at the new
realized depth; tune reduction thresholds (the 3/6-move, R=1/2
boundaries) in the Go mirror rather than 30-minute asm SPRTs.

## 2026-07-18 — deep-optimization batch: −15% cycles at identical trees

Six items from the optimization review docs applied (taper right-shift
multiply, SLOTTAB, emitmove filter removal, TT table-addr + unrolls,
movepiece/takepiece fusion with kind-major Zobrist relayout, attacked()
SMC rewrite). All behavior-identical — same trees, same scores, full
suite green after every step. Depth-6 fixed-tree cycles:

| features | before | after | delta |
|----------|--------|-------|-------|
| 0x00     | 7,604M | 6,480M | −14.8% |
| 0x01     | 6,708M | 5,724M | −14.7% |
| 0x07     | 4,404M | 3,742M | −15.0% |

The uniform ratio across configs confirms the cuts are in the
per-node constant factor, not tree shape. Cumulative since the perf
campaign began: 0x00 8,575M → 6,480M (−24%), all-features 4,912M →
3,742M (−24%). Still open from the review: two-ended emit
(~200-600/node, needs node-count A/B), two-copy generate, pawnterm
bitmask (lands with the Texel-tuned weights). Next depth lever: PVS/
LMR (F4b restructure).

## 2026-07-18 — long battery: gates at 100 games, features-vs-budget verdict, first TSCP wins

Full battery in tmux (runs/battery1.log), post give-check-propagation
build; full test suite green first.

- Depth-6 fixed-tree cycles: 0x00 = 7,604M, null-only = 6,708M,
  null+killers+futility (0x07) = 4,404M. The 0x00 baseline itself
  dropped from 8,243M — give-check propagation pays even with pruning
  off.
- Feature gates, 100 games each at 30 emulated s/move: null −3 ± 53,
  killers +17 ± 53, futility +10 ± 53, pstruct −35 ± 61. All still
  inside noise; accumulation continues (task #17).
- **All-features (0x0F) vs baseline, 200 games: −7 ± 39.** The
  features cut the depth-6 tree ~42% but buy no Elo at this budget —
  and the arithmetic says why: 30 emulated s ≈ 31M cycles, while even
  the 0x07 depth-5 tree costs hundreds of millions. Both configs
  realize depth 4; the saved cycles can't cross the next depth
  threshold, so they're wasted. The pruning features are levers that
  only cash out once constant-factor cuts (deep optimization review,
  ~750-850 cycles/node identified) + PVS/LMR bring depth 5 in range.
  pstruct's −35 drag is separately fixable: weights are untuned
  (Texel tuning in progress on the Go mirror, task #20).
- TSCP-d3 rematch, 30 games, dither on: **2-27-1 (8.3%)** — the first
  outright WINS against TSCP-d3 (previous best 0-18-2 = 5%). Still
  decisively outgunned at realized depth 4 vs its depth 3 + better
  eval + faster wall clock.

## 2026-07-18 — perf batch 1 (lazy legality, attacked() micro, hashstm unroll)

Depth-6 fixed-tree cycles: baseline 8,575M -> 8,243M; all features
4,912M -> 4,727M (~4% each). Well under the review's 15-20% model for
lazy legality — that model predated the QS surgery, which had already
removed most per-node legality work. Full suite green throughout
(perft exact; legality torture is the gate that would catch an
over-eager skip). Next ply must come from the structural items:
give-check propagation (kills the second full attacked() scan per
node) and the move-loop restructure.

## 2026-07-18 — M5a: eval terms, dither, and the depth verdict

- Pawn structure + king shield (FT_PSTRUCT) self-play A/B: +14 +/- 57
  over 100 games at 30 emulated s/move — directionally positive,
  unresolved at this sample size (accumulation continues).
- TSCP-d3 rematch with dither: **0-18-2** (from 0-20). First draws, and
  the games are finally distinct (per-move seeded eval dither, the
  simulation of the hardware plan to seed from input timing).
- **The decisive diagnostic**: the bridge now emits depth/score into
  the PGNs — we search **depth 4** at 30 emulated s/move in the
  middlegame. The remaining gap to TSCP-d3 is depth, not sanity.
  Next lever: the open performance items (lazy legality ~15-20%,
  move-loop restructure ~10-15%, make() fusion ~5-7%, give-check
  propagation ~15-20%) — together roughly a node doubling, ~+1 ply,
  compounding with ID/TT. Then re-measure.

## 2026-07-18 — post-fix measurements

Feature gates, self-play at 30 emulated s/move, generated paired
openings (80 games each; the fourth run was cut off by a task limit):
null +0 +/- 47, killers +26 +/- 50, futility +0 +/- 49. The 43%
tree-size win compresses to small Elo in self-play at these depths;
resolving +20-30 needs 400+ games (queued).

TSCP-d3 rematch at 30 emulated s/move: **0-20**. But the game character
changed completely: no more degenerate moves — legal, coherent,
planless chess, ground down positionally (opponent evals creep +0.4 to
+5 over ~25 moves). Diagnosis for next session: (a) instrument realized
search depth per move in games; (b) the eval gap — TSCP has pawn
structure + king safety, we are PSQT-only (M5 terms may now be worth
more than search); (c) deterministic + bookless = the same losing line
repeats every White game (book/variety, M6); (d) A/B the TT aux
carryover, which was never gated.

## 2026-07-18 — M4 debugging night: the pruning stack made real

Fixed-depth tree size on the reference middlegame position (cycles to
complete the search; the honest metric after learning that budget-mode
soft-stops invalidate comparisons):

| Features | Depth 6 cycles | vs baseline |
|---|---|---|
| none | 8,575M | — |
| null move | 7,498M | −13% |
| null + killers + futility | 4,912M | −43% |

What it took to get there (full detail in
docs/reviews/2026-07-18-code-review.md):
1. Adversarial review found null move disabled by an unsigned compare
   on the beta high byte (negative betas always read as "mate zone").
2. Fixing that made trees BIGGER (+73%): single-step instrumentation
   showed the search was QS-dominated (capture chains to ply 30,
   captures in piece-list order, delta pruning documented but never
   implemented). Fixed: two-tier MVV capture passes, per-ply delta
   threshold, capture-only qs generation.
3. Still bigger with null: shallow nulls (remaining 2-3) reduce to bare
   QS sweeps that start fail-low (with the eval>=beta gate, the null
   child's stand-pat can never cut) — all cost, no value when ordering
   already cuts on the first real move. Floor raised to remaining >= 4;
   null cutoffs now also store TT lower bounds, and attempts are gated
   on static eval >= beta.

Also: TT upper-bound cutoffs missed score==alpha (fixed; the baseline
itself dropped ~40% from this), eval got w=32/w=0 taper fast paths,
futility/RFP gained mate-zone window guards, iteration 1 is now
abort-immune.

## 2026-07-18 — M3 complete: first calibration matches

Engine: ID + aux TT + UCI bridge, 30 emulated s/move (~30.6M cycles).

| Opponent | Conditions | Result |
|---|---|---|
| N.E.G. 1.1 (very weak) | st=1 wall | **2-0** (mates both colors) |
| TSCP 1.81 (~1700 CCRL) | st=2 wall (full strength) | 0-10 |
| TSCP depth-limited to 3 | st=2, depth=3 | 0-6 |

Analysis (verified, not guessed):
- Bridge vs cold-engine replay of the loss prefix: identical moves at
  every turn — no TT-carryover corruption, no bridge bug.
- The "hung pawn" moves are PeSTO working as specified: an a-file pawn
  is worth only ~47cp in the mg table (82 - 35 PSQT), so trading it for
  ~20cp of activity is what the eval orders. Verified arithmetically:
  startpos minus the white a-pawn evals -37 = -(82-35) + 10 tempo.
- The real gap is depth: measured EBF is still ~8 (depth 5 on a
  middlegame position cost 5.1B cycles; 30M budget reaches ~depth 4).
  TSCP at depth 3 + its fuller eval outplays that.
- One real bug found and fixed en route: an aborted ID iteration's
  partial "best move" (fail-hard: the first root move always raises
  alpha from -INF) was preferred over the last completed iteration's
  move; with the root TT entry evicted this returned near-arbitrary
  moves, degrading play as the game went on.

This is the recorded pre-pruning baseline. M4 (null move R=2, killers,
futility/RFP) attacks the EBF directly; the plan's model expects those
to buy 2-4 effective plies at the same budget.

Also measured this cycle:
- ID + TT vs cold fixed-depth: WAC.001 to depth 4 in 706M cycles vs
  2,473M (3.5x, including depths 1-3). Suite: 1,537M vs 3,715M.
- WAC subset: 6/7 at depth 4 (both modes).
