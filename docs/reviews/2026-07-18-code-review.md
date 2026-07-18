# Adversarial code review — 2026-07-18

Three independent reviewers over the whole codebase: (A) correctness/
logic, (B) performance (cycle arithmetic, validated against the measured
1,750 cycles/leaf perft number), (C) architecture/cleanliness ("would
this survive a skeptical HN thread?"). Dispositions below; "fixed" =
same-day code/doc change, "open" = tracked work.

## The headline findings

| Finding (reviewer) | Disposition |
|---|---|
| **Null move silently disabled for every negative beta**: the "beta below the mate zone" guard compared the signed high byte unsigned, so beta < 0 (hi $80-$FF >= $74) always tripped it — null never ran where the side to move was ahead of the window, which is where null cutoffs live. Explains the Elo-flat feature battery. (A) | **Fixed** (signed-aware zone test + explicit root exclusion). |
| Fresh clone broken three ways: `cmd/a2run` source accidentally gitignored by an unanchored pattern; committed `go.work` requiring sibling checkouts; `make test` cd'ing into sibling repos. (C) | **Fixed**, verified by actually cloning and building. |
| QS tree pathology (found by our own instrumentation, confirmed direction by B's scan-cost math): no MVV ordering (captures ran in piece-list order), no delta pruning (doc'd as planned, never implemented — C flagged the drift), evasion chains to ply 30. Iteration-2 QS alone could eat >1.5B cycles. | **Fixed**: two-tier MVV capture passes (victims >= rook first), always-on delta pruning with per-ply threshold (disabled at phase < 6), quiet generation suppressed inside QS (GENCAPS inline drops). Evasion bounding deferred pending re-measurement. |

## Correctness (A)

| # | Finding | Disposition |
|---|---|---|
| A2 | Futility/RFP lacked mate-zone window guards (wrong bounds / missed frontier mates inside mate-score windows) | Fixed (skip the whole block when alpha or beta is in a mate zone) |
| A3 | TT upper-bound cutoff missed score == alpha | Fixed (compare flipped to alpha - score) |
| A4 | Iteration-1 abort could report an arbitrary root move | Fixed (checkclock never aborts during iteration 1) |
| A5 | 24-bit overflow deriving the 2x hard-abort limit | Fixed (saturate) |
| A6 | Go budget conversions: cycles 1-255 became "fixed-depth mode"; depth-mode run cap too small | Fixed (shared SetBudget rounds up; depth-mode cap raised) |
| A7 | All read traps gated on InAddr != 0 | Fixed (per-trap gating) |
| A8 | Engine can't see pre-root game history (threefold leak in won positions); referee draw-before-mate ordering nuance | Open (needs game-history feed via the session protocol; symmetric in self-play) |
| A9 | Dead INCHECK zp byte; TT-move matching promotions searches all four promos in pass 0 | INCHECK removed; promo nuance accepted (ordering only) |
| — | Verified clean: defs layout, make/unmake symmetry incl. clobber discipline, pass-partition exactness (no move skipped or double-searched), TT mate-ply adjustment, MSP discipline, 0x88 pawn edges, killers implementation | — |

## Performance (B) — NOW list status

| # | Finding (est. impact) | Disposition |
|---|---|---|
| F3 | Eval taper multiply always runs; w=32/w=0 fast paths absent (~350 cycles at most nodes) | Fixed (both tiers); convexity-vs-bounds shortcut still open |
| F4a | GENCAPS dropped quiets inside emitmove (~33 cycles per dropped quiet) | Fixed (inline drops at all quiet emit sites) |
| F1 | Legality via full attacked() after every make; ~570/node avoidable with a king-alignment pre-test | Open (next perf pass; needs perft+torture gates) |
| F2 | curincheck full scan at every node; give-check propagation from make would save ~650/node | Open (later; bug-farm territory) |
| F4b | 4-pass full-list rescans ~13K cycles/node worst case; TT-move-without-generation, two-ended emit | Open (restructure; partially mitigated by capture-only QS generation) |
| F5 | make() ~2.4x its budget; fuse the four per-piece routines, unroll hashstm | Open |
| F6/F7/F8 | attacked() micro, pawn-count byte for material-draw scan, ttread early-verify | Open |
| F9/F10 | Go-side per-move allocs, ID full-window re-search | No action (measured negligible / deferred by design) |

## Architecture/cleanliness (C) — beyond the clone-breakers

Fixed same-day: gofmt, .gitignore anchoring, Makefile engine/tables/
clean targets + guarded sibling tests, README go.work wording, debug
fossils (nulldebug scaffolding) removed after use.

Open, queued as the "doc-truth + dedup pass" (one change): testing.md
trap table (input/clock traps, stale "planned" prose, stale by-hand
example), plan.md milestone statuses + delta-pruning/check-extension
annotations, D6/D9/D12 statuses + the TT/killer list-match design
description, defs.inc header comment, mate-zone constants (defined; two
tt.s call sites still literal), shared SqName/MoveUCI/cyclesPerMs/
StartFEN helpers, test-build single-source-of-truth, replay test made
asserting, banktest fall-through + placement, results.md linked from
README, package chesstest naming/doc.

Explicitly preserved as good: the adversarial-review culture docs,
refchess isolation, aux-memory discipline comments, commit messages,
tools/README.md reproducibility notes.
