; Engine driver (M2): the rig pokes BOARD, PIECESQ, SIDE, EPSQ, CASTLE,
; HALFMOVE, and MAXDEPTH, then runs this binary. It searches to MAXDEPTH
; (plus quiescence), prints the best move in UCI form ("e2e4", "e7e8q")
; + LF via the COUT trap, and exits 0. With no legal moves it prints
; "none" and exits 2 (SCORE distinguishes mate from stalemate; the rig
; reads SCORE/BESTFROM/etc. from zero page).

        .include "defs.inc"

        .import __LCCODE_LOAD__, __LCCODE_RUN__, __LCCODE_SIZE__

        .segment "CODE"

entry:
        ldx #$FF
        txs
        ; install the LC-resident code (aux-read primitives): bank 1
        ; RAM, write-enabled by the double read
        lda $C08B
        lda $C08B
        lda #<__LCCODE_LOAD__
        sta ZPTR
        lda #>__LCCODE_LOAD__
        sta ZPTR+1
        lda #<__LCCODE_RUN__
        sta TTPTR
        lda #>__LCCODE_RUN__
        sta TTPTR+1
        ldx #>(__LCCODE_SIZE__ + 255)
        ldy #0
ldloop: lda (ZPTR),y
        sta (TTPTR),y
        iny
        bne ldloop
        inc ZPTR+1
        inc TTPTR+1
        dex
        bne ldloop
        lda #0
        sta PSP0
        sta PSP1
        sta GENCAPS
        sta ABORT
        sta NODECNT
        jsr evalinit
        lda #NOSQ
        sta BESTFROM
        ; depth cap; MAXDEPTH becomes the per-iteration depth
        lda MAXDEPTH
        sta MAXCAP
        ; hard-abort limit = 2x budget
        lda BUDGET0
        asl
        sta ABORTL0
        lda BUDGET1
        rol
        sta ABORTL1
        lda BUDGET2
        rol
        sta ABORTL2
        ; fixed-depth mode (budget 0): one iteration at the cap
        lda BUDGET0
        ora BUDGET1
        ora BUDGET2
        bne idmode
        lda MAXCAP
        sta CURDEPTH
        jsr iterate
        jmp report
idmode: lda #1
        sta CURDEPTH
idloop: jsr iterate
        lda ABORT
        beq idok
        ; aborted mid-iteration: a partial iteration's "best" is just the
        ; first root move it happened to search (fail-hard alpha starts at
        ; -INF), so prefer the last COMPLETED iteration's move whenever
        ; one exists. (D9's improved-on-previous-score refinement needs
        ; score bookkeeping; this is the safe subset.)
        lda PREVFROM
        cmp #NOSQ
        beq report              ; iteration 1 aborted: keep what we have
        sta BESTFROM
        lda PREVTO
        sta BESTTO
        lda PREVFLAGS
        sta BESTFLAGS
        jmp report
idok:   ; don't start another iteration past half the budget
        lda BUDGET2
        lsr
        sta T2
        lda BUDGET1
        ror
        sta T1
        lda BUDGET0
        ror
        sta T0
        lda CLOCK_TRAP
        cmp T0
        lda CLOCK_TRAP+1
        sbc T1
        lda CLOCK_TRAP+2
        sbc T2
        bcs report              ; spent >= budget/2
        inc CURDEPTH
        lda CURDEPTH
        cmp MAXCAP
        bcc idloop
        beq idloop
        jmp report

; iterate: run one full-window search at CURDEPTH, saving the previous
; iteration's best move first.
iterate:
        lda BESTFROM
        sta PREVFROM
        lda BESTTO
        sta PREVTO
        lda BESTFLAGS
        sta PREVFLAGS
        lda #NOSQ
        sta BESTFROM
        lda CURDEPTH
        sta MAXDEPTH
        lda #0
        sta PLY
        lda #<MOVESTACK
        sta MSP
        lda #>MOVESTACK
        sta MSP+1
        lda #<(-INF)
        sta ALPHALO
        lda #>(-INF)
        sta ALPHAHI
        lda #<INF
        sta BETALO
        lda #>INF
        sta BETAHI
        jmp search              ; rts returns to iterate's caller

report: lda BESTFROM
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
        .include "tt.s"
        .include "eval.s"
        .include "board.s"
        .include "movegen.s"
        .include "tables.s"
