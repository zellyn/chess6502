# Testing and the development loop

This project develops a 6502 chess engine without touching disk images or
GUI emulators: assemble on the Mac, run headless in a Go harness built on
the cycle-accurate `go6502` CPU core, assert on results, repeat.

## The pieces

| Piece | Where | Status |
|---|---|---|
| 6502 CPU core | `github.com/zellyn/go6502/cpu` (sibling checkout `../go6502`) | Modernized: `go.mod` added, all tests pass, including Klaus Dormann's functional suite run in lockstep against the `visual/` gate-level (perfect6502) simulation |
| IIe 128K memory model | `github.com/zellyn/goapple2/iie` (sibling checkout `../goapple2`) | New package; see below |
| Headless runner | `cmd/a2run` (this repo) | Working |
| Assembler | ca65/ld65 (cc65 V2.18, via Homebrew) | Working; raw-binary link config in `hello/raw2000.cfg` |
| Hardware-truth memory tests | `github.com/zellyn/a2audit` (sibling checkout `../a2audit`) | Language Card suite passing against `iie`; aux-memory suite blocked on stage 2 |

## The dev loop

```sh
make            # assemble hello, run it in the harness, run all Go tests
# or by hand:
ca65 engine.s -o engine.o
ld65 -C raw2000.cfg engine.o -o engine.bin
go run ./cmd/a2run -bin engine.bin -org 0x2000 [-dump 0300:0320] [-trace]
```

`a2run` loads the raw binary into main RAM (validating org/entry ranges
and overruns), jumps to it, and runs at full host speed (~100-170x real
time depending on workload — an emulated hour takes well under a wall
minute, so self-play gauntlets are practical; a `go test -bench` rig to
pin this number down lands with M1). It reports cycles and emulated time
(1.0205 MHz effective IIe clock) to stderr; program output (COUT trap)
goes to stdout, instruction traces (`-trace`) to stderr, so the two never
interleave. Memory dumps use the side-effect-free `Peek` path — dumping
`$C08x` does not flip Language Card state. The cycle limit is checked
between instructions (overshoot <= 7 cycles). `-rom` loads a 12K
$D000-$FFFF image for runs that need monitor/Applesoft ROM.

### Harness I/O conventions

On real hardware these addresses are plain RAM, so engine binaries that use
them via the swappable I/O module still run there unmodified. Traps fire
only on **main-bank** stores (aux writes to the same addresses via RAMWRT
are ordinary memory), and `$BF00-$BFFF` is reserved in both banks by the
memory map (D8) so engine tables can never collide with them.

| Address | Access | Means |
|---|---|---|
| `$BFF0` | store (main bank) | emit byte to stdout |
| `$BFFF` | store (main bank) | exit; stored value becomes the process exit code |
| `$BFF1`/`$BFF2` | read | planned (D12, at M3): input byte / input-ready status |
| `$C019` | read | VBL status derived from the cycle counter (bit 7 low during VBL, IIe sense) — lets the hardware timing path be tested pre-metal |

The `-cout`/`-exit` flags can relocate the traps for experiments, but
$BFF0/$BFFF are canonical — all checked-in code assumes them.

Planned additions as the engine needs them (mostly at M3, see D12):
input traps + long-lived session mode for the UCI bridge, extraction of
the a2run core into an importable Go package so perft/gauntlet rigs run
in-process instead of scraping CLI output, PC-trap callbacks, and a
symbol-aware trace using the ca65 listing/map files. Perft results come
out via COUT as ASCII (exit codes are 8-bit; counts are 32-bit).

## The IIe 128K memory model (`goapple2/iie`)

goapple2 proper is a ][+ emulator with a single flat 64K array; it had
none of the IIe auxiliary-memory machinery. The new `iie` package is a pure
memory model (no video, no cards) implementing:

- 64K main + 64K aux RAM
- RAMRD/RAMWRT (`$C002-$C005`): read/write banking for `$0200-$BFFF`
- ALTZP (`$C008/$C009`): banking for `$0000-$01FF` and Language Card RAM
- Language Card banking (`$C080-$C08F`), both banks, including the
  double-read write-enable (prewrite) behavior
- Status reads: `$C011-$C016`, `$C018`
- An `Unhandled` counter for accesses to anything it does not implement,
  so the harness warns when code strays outside the supported subset

Deliberately unimplemented in stage 1 (the engine keeps 80STORE off, which
on real hardware makes RAMRD/RAMWRT govern all of `$0200-$BFFF`):
80STORE/PAGE2/HIRES display-coupled banking, INTCXROM/SLOTC3ROM Cxxx ROM
mapping, keyboard. Stage 2 adds these, validated by the rest of a2audit,
and wires the model into the interactive goapple2 machine.

### Validation against a2audit

`iie/audit_test.go` assembles a2audit's test binary with the ACME assembler
pinned inside that repo (note: the pin is a hermit shim — the first-ever
run on a fresh machine fetches ACME over the network), loads it at `$6000`
with the ][+ ROM, runs the monitor init a real boot would perform, calls
test entry points directly (addresses parsed from ACME's `--symbollist`
output at test time, so they can't go stale), and checks the zero-page
result flags:

- `TestA2AuditLangcard`: **passing** — the data-driven Language Card
  suite, verified against real hardware, including WRTCOUNT quirks.
- `TestA2AuditAuxmem`: skipped pending stage 2 (exercises 80STORE/PAGE2/
  HIRES aliasing and Cxxx ROM in its later stages).

A subtlety this loop already caught: calling audit code on a cold machine
hangs in a BRK loop, because monitor output vectors through CSW (`$36/$37`)
and the BRK vector (`$3F0`) is uninitialized — the stub must run
SETKBD/SETVID/INIT/HOME first, like a real boot.

## Engine-specific test rigs (planned; see docs/plan.md milestones)

- **Perft in the emulator**: run the engine's move generator to fixed
  depths from standard positions (startpos, Kiwipete, etc.) inside a2run
  and compare node counts against published values. This is the movegen
  correctness gate before any search work.
- **Search invariants**: fixed-depth, fixed-node searches of tactical
  positions with expected best moves (subset of WAC), run in CI.
- **UCI bridge** (`cmd/uci`): a Go process that speaks UCI to cutechess-cli
  and relays moves to the emulated engine through the harness I/O traps
  (D12 input design; long-lived session). Gauntlets follow the four-part
  protocol in plan.md/D11: paired openings always; opponents bracketing
  our level at their rating-valid conditions; node-odds ladder for the
  headline. cutechess must be run with generous wall margins
  (`timemargin`) — the engine's real clock is its internal cycle budget,
  and its wall usage (~0.2-0.4s per 30-60s emulated move) must never be
  what decides a game.
- **Determinism**: an engine-side property, not a harness feature — the
  engine enforces a fixed *emulated-cycle* budget internally (cycles,
  not nodes: cycle budgets are deterministic AND sensitive to per-node
  cost, which is what half our features change). Same position + same
  budget = same move, regardless of host speed.

## Speed / capacity notes

- Measured harness speed: ~100-170x real time depending on workload
  (M-series Mac, informal; tiny runs are dominated by process startup
  and measure ~1-2x). A benchmark rig lands with M1 so this number is
  monitored, not asserted.
- One 40-move game at 30 s/move ≈ 20 emulated minutes ≈ ~10 wall
  seconds of engine time; opponents at CCRL-valid controls dominate
  gauntlet wall time (~4 min/game at 40/4) — ~35 core-hours per
  500-game gauntlet, parallelizable across cores.

## Known infrastructure gaps (from adversarial review)

- **No CI anywhere** (chess6502, goapple2, go6502, a2audit) — the
  hardware-truth gates currently run only on one machine. GitHub Actions
  for `make test` is the obvious first step once repos are pushed.
- The load-bearing changes in sibling repos (goapple2's `iie/` package
  and go.mod, go6502's go.mod and test fixes) and this repo itself need
  commits/pushes before the documented workflow is reproducible anywhere
  else.
- a2audit assembly writes `audit.o` into the a2audit checkout (gitignored
  there, but still a cross-repo side effect of `go test`).
