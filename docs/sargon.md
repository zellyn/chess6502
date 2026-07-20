# Driving Sargon III (1983) as a headless opponent

Sargon III (Hayden, 1983; Dan & Kathe Spracklen) is the canonical strong
1 MHz-6502 chess program (~1550-1600 strength) and our benchmark opponent. This
documents how we boot and drive it headlessly inside the goapple2 emulator.

Code: `internal/sargon` (driver library) + `cmd/sargon` (CLI/demo). Disk image:
`assets/sargon-iii.dsk`. Manual: `assets/sargon-iii.pdf`.

## Harness

`internal/sargon` constructs a headless Apple ][+ (48K + language card, Disk II
in slot 6) by driving goapple2's `Apple2` struct directly with a no-op plotter —
no GUI. ROMs are read from the sibling goapple2 checkout
(`/Users/zellyn/gh/goapple2/data/roms`, override with `$GOAPPLE2_ROMS`).

Key methods (`Machine`):
- `NewMachine(dsk)` — mount disk, no stepping yet.
- `BootToPrompt()` — step until the board is set up (piece list shows the start
  position **and** "SARGON III" is on screen), then a ~35M-cycle settle so the
  program finishes loading before the first move (entering a move too early
  corrupts input).
- `EasyMode()` — CTRL-E (see below).
- `Level(n)` — set level 1-9 via the shifted digit.
- `SubmitMove("E2-E4", budget)` — inject a move and return Sargon's reply.
- `ReadPieceList()` / `Screen()` / `TextRow()` — RAM + text-screen introspection.

Emulation runs ~50M CPU cycles/sec wall (≈ 50x realtime); boot-to-prompt is
~120M cycles ≈ 2-3 s wall.

## Interfaces to Sargon

**Input — keyboard text entry.** Moves are typed as `FROM-TO`, e.g. `E2-E4`,
then Return. Captures may be typed with `-` or `X`. Keystrokes must be **paced**
(step the CPU ~200k cycles between characters) — injecting a whole move at once
races Sargon's keyboard poll and drops characters. `SubmitMove` handles pacing
and retries a dropped/garbled entry.

**Output — two channels:**

1. **Text move-list (authoritative, always reliable).** Text page 1
   (`$400-$7FF`, standard interleaved layout) shows a running move list. Columns:
   move number at cols 4-7, PLAYER move at cols 10-15, SARGON move at cols 22-27.
   Sargon's reply is read directly as algebraic (`E7-E5`, `E5XD4`, `0-0`,
   `PXPEP`, `.../Q`). This works regardless of pondering.
2. **Zero-page piece list (RAM, reliable only in Easy Mode).** See below.

## Board in RAM: the zero-page piece-square list

Sargon keeps the live position as a **32-entry piece-square list** in zero page:

- `$60-$6F` — 16 **white** pieces
- `$70-$7F` — 16 **black** pieces

Each byte is the piece's square in **0x88-style coordinates**:
`square = rank*16 + file`, with rank 0 = rank 1 and file 0 = file a. Valid
squares are `$00-$77` (both nibbles 0-7; bit 7 always clear). Algebraic:
`file = 'a' + (sq & 0x0F)`, `rank = '1' + (sq >> 4)`. Examples: e2=`$14`,
e4=`$34`, e7=`$64`, a1=`$00`, h8=`$77`.

The index within each 16-entry half is a **fixed piece slot** (type implied by
slot):

| index | 0-7  | 8 | 9 | 10 | 11 | 12 | 13 | 14 | 15 |
|-------|------|---|---|----|----|----|----|----|----|
| type  | pawn a-h | N | N | B | B | R | R | Q | K |

(Home files of slots 8-15: b, g, c, f, a, h, d, e.) A **captured** piece has its
square byte set to **`$80`** (bit 7 set). A promoted pawn keeps its slot but
changes type — detect promotions from the move text (`/Q`), not the slot.

Verified by diffing RAM around known moves: our `1.e4` set white e-pawn `$14`→
`$34` (e2→e4) at `$64`, and Sargon's `1...e6` set black e-pawn `$64`→`$54`
(e7→e6) at `$74`; `exd5` set the captured black d-pawn's byte to `$80`. There is a matching static
template of this list at `$1120-$113F` (used to reset a new game) and a
square→hi-res-coordinate table at `$1E00`. The board is drawn in hi-res on page
2 (`$4000-$5FFF`).

### Easy Mode is required for reliable RAM reads

At its normal levels Sargon **ponders on the player's time**, continuously
running its search and **overwriting the `$60-$7F` list as scratch**. So the
list is only coherent while Sargon is truly idle. Enabling **Easy Mode
(CTRL-E)** stops pondering; then the list is stable between moves and both our
own move confirmation and Sargon's reply decode cleanly from RAM — cross-checked
against the text move-list, they agree on normal moves, captures, and castling.

Easy Mode also makes per-move timing ponder-free and reproducible, which
**matches our non-pondering engine** and is what we want for fair, repeatable
matches. Per the manual it roughly halves effective strength at a given level,
so pick the level with that in mind. The driver defaults Easy Mode on.

## Fair-match timing: infinite mode + CTRL-T at our cycle budget (PRIMARY)

Sargon's levels are **average** per-move targets and it **banks time** (instant
opening-book moves fund longer thinks later — the same idea as our BankedClock),
and it only **estimates** seconds (the Apple has no real-time clock, so it
counts a software loop). Matching its self-managed timer is therefore fragile.

The robust, reproducible mechanism is to **take the clock away from Sargon**:
put it on the **Infinite** level (SHIFT-9) and force the move ourselves with
**CTRL-T** after exactly the number of 6502 cycles *we* choose — symmetric with
our own cycle-budgeted engine and fully deterministic in the cycle-accurate
emulator. This is `Machine.RequestMove(move, budgetCycles)` (set the level once
with `InfiniteLevel()`).

Cycle timing uses a new cycle-accurate clock added to goapple2
(`Apple2.Cycles()`, one tick per 6502 cycle; **note our `Machine.Steps`/`Run`
count instructions, not cycles**). `MoveResult.ThinkCycles` reports the cost
Sargon spent on each reply. `CyclesPerSecond = 1_020_500` (≈1.0205 MHz), so a
~30 s / ~30M-cycle budget matches our engine's standing budget.

**Verified empirically (the make-or-break unknowns):**
1. **CTRL-T plays a legal best-move-so-far**, not an abort: budgeting 30M cycles,
   Sargon thought 31.65M then CTRL-T → legal capture `D5xC4`, board consistent.
2. **Commit latency after CTRL-T ≈ 1.6M cycles** (~1.6 s); we detect the
   committed move by the text move-list token changing, not a fixed delay.
3. **Opening-book moves are handled**: in infinite mode a book reply appears on
   its own before the budget elapses (e.g. `d7d5` at 1.29M cycles); `RequestMove`
   returns it as-is (ThinkCycles ≈ 0) without needing CTRL-T.

### Alternative mode: real tournament clock (cross-check)

Set Sargon to SHIFT-3 (30 s/move avg = 60 moves / 30 min whole-game budget) and
give our engine a matching 60-moves/30-min banked control, letting each side
manage its own clock (exercises Sargon's own time management). Use once as a
cross-check; the infinite + CTRL-T path stays primary.

### Empirical shift-3 cost (Sargon self-estimates; measure in cycles)

Level 3 ("30 s/move") in the emulator: an opening-book reply cost ~1.3M cycles;
an out-of-book recapture cost ~10.3M cycles (~10 s) — **not** the nominal 30 s,
confirming Sargon banks/averages rather than spending a fixed budget per move.
So shift-3 is a rough quality anchor (an infinite + CTRL-T-at-~30M-cycle move
should be ≈ shift-3 strength), but for a controlled match use the cycle budget.

## Levels and per-move time (manual p.18)

Levels are selected with **SHIFT + digit** (ASCII `! @ # $ % ^ & * (` for 1-9),
changeable on any turn. Times are Sargon's internal per-move budget; the Apple
has no real-time clock, so Sargon estimates seconds by counting toggles of its
"thinking" asterisk — these track wall-clock closely on the original 1 MHz Apple.

| Level (SHIFT-n) | Avg response | Time control |
|-----------------|--------------|--------------|
| 1 | **5 s/move** (blitz) | 60 moves / 5 min |
| 2 | 15 s/move | 60 moves / 15 min |
| **3** | **30 s/move** | 60 moves / 30 min |
| 4 | 1 min/move | 60 moves / 1 hr |
| 5 | 2 min/move | 30 moves / 55 min |
| 6 | 3 min/move | 40 moves / 1h50 |
| 7 | 6 min/move | 30 moves / 3 hr |
| 8 | 10 min/move | 40 moves / 6h40 |
| 9 | infinite | no limit |

**For our ~30 s/emulated-move engine, use Level 3.** Avoid Level 1/2 (blitz) and
6-9 (multi-hour / infinite). Note Easy Mode halves this, so Level 3 + Easy Mode
gives a faster, weaker opponent than Level 3 pondering; when we set up the fair
match we can compare Level 3 (Easy) vs Level 4 (Easy) against our engine's
actual per-move compute.

## Other useful Sargon commands (manual)

CTRL-S change sides (Sargon takes your side / plays White), CTRL-N new game,
CTRL-T force move now, CTRL-B take back, CTRL-A analysis/board-edit mode, CTRL-G
save game, CTRL-L load game, ESC toggle board/text view. Rejected moves show
`ILLEGAL MOVE` / `INVALID MOVE. TRY AGAIN.` on the message line (text row 5).

## CLI

```
# Boot and dump the screen (proves it boots):
go run ./cmd/sargon -boot 110000000

# Sample the screen during boot:
go run ./cmd/sargon -boot 120000000 -sample 20000000

# Scripted keys: wait:N ; type:STR ; key:HH (hex) ; dump ; save:FILE
go run ./cmd/sargon -boot 110000000 -script 'type:E2-E4\r; wait:8000000; dump'

# Play mode: feed player moves, print Sargon's replies (screen + RAM), Easy Mode:
go run ./cmd/sargon -play 'E2-E4,D2-D4,B1-C3' -level 3
```

## xboard adapter (`cmd/sargon-xboard`)

`cmd/sargon-xboard` presents headless Sargon as an xboard (CECP) engine so
cutechess-cli can pit it against another engine. It plays the side that moves
**second** (run it as Black; Sargon's keyboard player is White): opponent moves
arrive as `usermove e2e4`, are typed into Sargon, and Sargon's reply is emitted
as `move e7e6` (coordinate notation from the RAM from/to squares).

Key implementation points, learned the hard way:
- **Boot in a goroutine** and **think in a goroutine** — Sargon's ~3 s boot and
  multi-second search must not block the protocol loop, or cutechess declares a
  stalled connection when its liveness `ping` goes unanswered.
- **force/go handshake**: cutechess starts a Black engine with
  `force; usermove <white1>; go`. Sargon replies as soon as it gets the
  usermove, so the adapter computes the reply but **holds it until `go`**.
- `-budget-cycles N` uses the fair mechanism (Infinite + CTRL-T at N cycles);
  otherwise `-level`/`-easy` timed mode.

**Smoke game played** (our engine White, fast budget, vs Sargon level 1 Black):
a complete 78-ply game via cutechess-cli ending in checkmate (`39...Qh4#`,
Sargon 0-1), with castling, captures and checks all handled correctly.

```
cutechess-cli \
  -engine name=us cmd=./us proto=uci \
  -engine name=SargonIII cmd="./sargon-xboard -budget-cycles 30000000" proto=xboard \
  -each tc=inf -games 1 -pgnout smoke.pgn
```

## Sargon as White (CTRL-S) and matched gauntlet

`StartAsWhite(budgetCycles)` sends CTRL-S so Sargon takes White and plays the
opening move (the keyboard opponent becomes Black); the driver flips which
piece-list half and move-list column belong to Sargon (`Machine.SargonWhite`).
Note the move-list header swaps ("SARGON PLAYER") but the columns don't: White's
move is always the LEFT column (cols 10-15), Black's the RIGHT (22-27). The
xboard adapter detects White automatically (a `go` with no prior `usermove`).

For matches the adapter runs **Infinite level + Easy Mode + CTRL-T**: Easy Mode
stops Sargon pondering after it moves, so the piece list stays stable (reliable
reads) and play is reproducible and ponder-free (matching our non-pondering
engine). The adapter emits Sargon's move parsed from the authoritative on-screen
token (RAM decode is a fallback) and **never claims a game result** — cutechess
adjudicates mate/stalemate/draws from the moves; a wrong claim (our game-over
scrape can false-positive on "CHECK") would lose the game as an "invalid result
claim".

### KNOWN LIMITATION: no setboard / opening pool yet

Games run from the **standard start position** (opening variety comes from our
engine's `-dither`). Setting an arbitrary FEN (e.g. `tools/openings-pool.epd`)
does NOT work: Sargon reconstructs its board from the on-screen move list
(replaying from a fixed template), so the `$60-$7F` piece list is a derived copy
— poking it is reverted on the next move (a fresh game replays zero moves ->
standard start). A search of RAM found no rank-structured master board under any
stride, so proper setboard needs deeper reverse-engineering of Sargon's move-gen
board / move-list state. `SetupPosition` parses the FEN and assigns slots
correctly (unit-tested) but is a no-op on Sargon's actual position; left in with
a clear caveat. This is the top follow-up for reduced-variance rating pools.

## Status / next steps

Done: headless boot; text-screen scraping; paced keyboard move injection; reply
parsing (text + RAM) for normal moves / captures / castling; board RE
(zero-page piece list); level table; **fair-match primitive `RequestMove`
(Infinite + CTRL-T at a chosen cycle budget), verified**; cycle-accurate clock
in goapple2; **xboard adapter with a completed cutechess smoke game**.

Follow-ups: run a real matched match (our engine vs Sargon, ~30M cycles/move
both sides, from `tools/openings-pool.epd` positions) for an anchored strength
estimate; optionally support Sargon-as-White (CTRL-S) and the tournament-clock
cross-check mode; wire an openings book into the adapter for varied games.
