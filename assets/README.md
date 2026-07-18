# Art assets

Hand-drawn in DazzleDraw on the Apple II (well, an emulator) by zellyn.

- `chessboard.*` — the hires chessboard and piece set (monochrome-
  targeted; the striped light squares NTSC-artifact into color on
  composite displays, so a monochrome monitor profile is the intended
  look).

Preferred format: the raw DazzleDraw/hires save (the actual $2000-page
byte layout, 8192 bytes, no conversion loss). A PNG export alongside is
welcome for browsing on GitHub. The M8 display work will slice this
into per-square tiles and pre-shifted piece sprites (pieces occupy 8
fixed columns, so at most 8 bit-phases per sprite — generated at init,
not hand-maintained).
