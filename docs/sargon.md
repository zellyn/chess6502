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

## Status / next steps

Done: headless boot, screen scraping, keyboard move injection, reply parsing
(text + RAM) for normal moves / captures / castling, board RE, level table.

Remaining stretch (task follow-up): wrap `internal/sargon` as an xboard/UCI
adapter process so cutechess-cli can pit our engine vs Sargon, and play a smoke
game. The adapter maps our engine's coordinates ↔ Sargon's `FROM-TO` text and
uses `SubmitMove` per move. Special-move notation to map: castling (type king
`E1-G1`, Sargon shows `0-0`), en passant (`PXPEP`), promotion (append piece
letter to under-promote; Return promotes to queen).
