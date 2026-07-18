; chess6502 core definitions: memory layout and encodings.
; The Go test rig reads these addresses from the ld65 label file (-Ln);
; keep symbols exported (no @local) for anything Go needs to poke or read.

; ---- Piece encoding (board byte) ----
; index<<4 | color<<3 | type    (0 = empty square)
; type 1..6 = P N B R Q K; color 0 = white, 8 = black (mask COLORMASK)
; slot in PIECESQ = index + (color ? 16 : 0); king is always index 0.
PAWN    = 1
KNIGHT  = 2
BISHOP  = 3
ROOK    = 4
QUEEN   = 5
KING    = 6
TYPEMASK  = $07
COLORMASK = $08
INDEXMASK = $F0

; ---- Attack table bits (must match cmd/gentables) ----
ATK_KNIGHT = $01
ATK_KING   = $02
ATK_DIAG   = $04
ATK_ORTHO  = $08
ATK_WPAWN  = $10
ATK_BPAWN  = $20

; ---- Move flags ----
; bits 0-2: promotion piece type (0 = none, else N/B/R/Q)
; bit 3: en-passant capture   bit 4: double pawn push   bit 5: castle
FL_PROMO  = $07
FL_EP     = $08
FL_DOUBLE = $10
FL_CASTLE = $20

; ---- Castling rights bits (in CASTLE) ----
CR_WK = $01
CR_WQ = $02
CR_BK = $04
CR_BQ = $08

NOSQ   = $FF            ; "no square": ep-none, tombstoned piece slot
MAXPLY = 32

; ---- Zero page ----
SIDE      = $00         ; side to move: 0 = white, COLORMASK = black
EPSQ      = $01         ; 0x88 square behind a double push, or NOSQ
CASTLE    = $02         ; CR_* bits
PLY       = $03
ROOTDEPTH = $04         ; perft depth (poked by the test rig)
PCOUNT    = $08         ; 32-bit little-endian perft leaf counter
MSP       = $10         ; 16-bit pointer: top of move stack
FROM      = $12
TO        = $13
MVPIECE   = $14
MVFLAGS   = $15
ATSQ      = $16         ; attacked(): target square
ATSIDE    = $17         ; attacked(): attacking side (0 or COLORMASK)
ATTMP     = $18         ; attacked(): scratch (candidate from-square)
ATSLOT    = $19         ; attacked(): scratch (slot loop)
GTMP      = $1A         ; movegen scratch
GSLOT     = $1B         ; movegen: current slot
GFROM     = $1C         ; movegen: current from-square
GPIECE    = $1D         ; movegen: current piece byte
GDIR      = $1E         ; movegen: current direction index
GDELTA    = $1F         ; movegen: current step delta
CAPSLOT   = $20         ; make: captured piece's slot
PROMTYPE  = $21         ; movegen: promotion loop scratch
GTO       = $22         ; movegen scratch target square
ATBITS    = $23         ; attacked(): attack-table bits for current diff
CURPTR    = $24         ; 16-bit: perft move-iteration pointer
DIFF      = $26         ; attacked(): saved difference index
ATDELTA   = $27         ; attacked(): slider step delta
GCOUNT    = $28         ; movegen: offset-loop counter
CRTMP     = $29         ; castle helpers: rook piece byte

BOARD     = $40         ; 128-byte 0x88 board, $40..$BF

; ---- Absolute data ----
PIECESQ    = $0800      ; 32 bytes: slots 0-15 white (0=king), 16-31 black
PLYBASELO  = $0820      ; per-ply move-stack base pointer
PLYBASEHI  = $0840
PLYENDLO   = $0860      ; per-ply move-stack end (after generation)
PLYENDHI   = $0880
CURSORLO   = $08A0      ; per-ply iteration cursor
CURSORHI   = $08C0

UNDOCAP    = $08E0      ; captured piece byte (0 if none)
UNDOCAPSQ  = $0900      ; captured piece square (differs from TO for ep)
UNDOCASTLE = $0920
UNDOEP     = $0940
UNDOPIECE  = $0960      ; moving piece byte before promotion
UNDOFROM   = $0980
UNDOTO     = $09A0
UNDOFLAGS  = $09C0

MOVESTACK  = $0A00      ; 3 bytes/move (from,to,flags); through $17FF
MOVESTACKTOP = $1800    ; exclusive end; generator overflow-checks this

; ---- Harness I/O (plain RAM on real hardware; see docs/testing.md) ----
COUT_TRAP = $BFF0
EXIT_TRAP = $BFFF
