# chess6502

A maximum-strength chess engine for the Apple IIe (1 MHz 6502, 128K),
built to demonstrate how far chess engine theory has advanced since the
1980s. Target: measurably stronger than anything that ever ran on a
1 MHz 6502.

- **The plan**: [docs/plan.md](docs/plan.md)
- **Decisions**: [docs/decisions.md](docs/decisions.md)
- **Dev loop & testing**: [docs/testing.md](docs/testing.md)
- **Research notes** (with citations): [docs/research/](docs/research/)

## Quick start

Requires: Go and cc65 (`brew install cc65`). Dependencies
([go6502](https://github.com/zellyn/go6502),
[goapple2](https://github.com/zellyn/goapple2)) resolve as normal Go
modules; to hack on them locally, create a `go.work` pointing at sibling
checkouts (`go work init . ../go6502 ../goapple2`).

```sh
make    # assemble + run the smoke tests, build the engine, run all tests
```

`cmd/a2run` runs 6502 binaries against the cycle-accurate go6502 core and
the Apple IIe 128K memory model (`goapple2/iie`, validated against
[a2audit](https://github.com/zellyn/a2audit)'s hardware-verified tests).
