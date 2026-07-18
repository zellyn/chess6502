; Engine driver (M2): the rig pokes BOARD, PIECESQ, SIDE, EPSQ, CASTLE,
; HALFMOVE, and MAXDEPTH, then runs this binary. It searches to MAXDEPTH
; (plus quiescence), prints the best move in UCI form ("e2e4", "e7e8q")
; + LF via the COUT trap, and exits 0. With no legal moves it prints
; "none" and exits 2 (SCORE distinguishes mate from stalemate; the rig
; reads SCORE/BESTFROM/etc. from zero page).

        .include "defs.inc"

        .segment "CODE"

entry:
        ldx #$FF
        txs
        lda #<MOVESTACK
        sta MSP
        lda #>MOVESTACK
        sta MSP+1
        lda #0
        sta PLY
        sta PSP0
        sta PSP1
        sta GENCAPS
        jsr evalinit
        lda #<(-INF)
        sta ALPHALO
        lda #>(-INF)
        sta ALPHAHI
        lda #<INF
        sta BETALO
        lda #>INF
        sta BETAHI
        lda #NOSQ
        sta BESTFROM
        jsr search
        lda BESTFROM
        cmp #NOSQ
        beq nomove
        jsr printsq
        lda BESTTO
        jsr printsq2
        lda BESTFLAGS
        and #FL_PROMO
        beq endline
        tax
        lda promochar,x
        sta COUT_TRAP
endline:
        lda #$0A
        sta COUT_TRAP
        lda #0
        sta EXIT_TRAP
        brk

nomove: ldx #0
:       lda nonetxt,x
        beq :+
        sta COUT_TRAP
        inx
        bne :-
:       lda #2
        sta EXIT_TRAP
        brk

; printsq: A = 0x88 square of BESTFROM; printsq2 same for any square.
printsq:
        lda BESTFROM
printsq2:
        pha
        and #$0F
        clc
        adc #'a'
        sta COUT_TRAP
        pla
        lsr
        lsr
        lsr
        lsr
        clc
        adc #'1'
        sta COUT_TRAP
        rts

promochar:
        .byte 0, 0, 'n', 'b', 'r', 'q', 0, 0
nonetxt:
        .byte "none", $0A, 0

        .include "search.s"
        .include "eval.s"
        .include "board.s"
        .include "movegen.s"
        .include "tables.s"
