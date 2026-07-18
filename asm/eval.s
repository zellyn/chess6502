; Evaluation: incremental tapered PeSTO piece-square tables + tempo.
; Accumulators (MGSCORE/EGSCORE, white POV, and PHASE) are updated by
; make via addpiece/rempiece and restored wholesale by unmake.
;
; Table layout (see cmd/gentables): per type t, a 512-byte block at
; PSQTBASE+(t-1)*512: MGLO[sq] +0, MGHI +128, EGLO +256, EGHI +384.
; TYPEPAGE0/1[type] give the two page-hi bytes; PSP0/PSP1 lo bytes stay 0.
; Black pieces use sq^$70 and contribute with opposite sign.

; ---------------------------------------------------------------
; addpiece / rempiece: A = piece byte, Y = 0x88 square.
; Updates MGSCORE/EGSCORE/PHASE. Clobbers A,X,Y, T0-T3, EVTMP,
; PSQPIECE, PSQSQ, MULCNT.
; ---------------------------------------------------------------
addpiece:
        ldx #0
        beq psqcom              ; always
rempiece:
        ldx #1
psqcom: sta PSQPIECE
        sty PSQSQ
        stx EVTMP               ; 0 = add piece, 1 = remove piece
        ; phase contribution
        and #TYPEMASK
        tax
        lda PHASEVAL,x
        beq psqnoph
        sta MULCNT
        lda EVTMP
        bne psqphsub
        lda PHASE
        clc
        adc MULCNT
        sta PHASE
        jmp psqnoph
psqphsub:
        lda PHASE
        sec
        sbc MULCNT
        sta PHASE
psqnoph:
        ; table pointers for this type
        lda TYPEPAGE0,x
        sta PSP0+1
        lda TYPEPAGE1,x
        sta PSP1+1
        ; black: flip rank, flip sign
        lda PSQPIECE
        and #COLORMASK
        beq psqwhite
        lda PSQSQ
        eor #$70
        sta PSQSQ
        lda EVTMP
        eor #1
        sta EVTMP
psqwhite:
        ; fetch mg/eg values
        ldy PSQSQ
        lda (PSP0),y
        sta T0
        lda (PSP1),y
        sta T2
        tya
        ora #$80
        tay
        lda (PSP0),y
        sta T1
        lda (PSP1),y
        sta T3
        ; apply: EVTMP 0 = add to white score, 1 = subtract
        lda EVTMP
        bne psqsub
        clc
        lda MGSCORE
        adc T0
        sta MGSCORE
        lda MGSCORE+1
        adc T1
        sta MGSCORE+1
        clc
        lda EGSCORE
        adc T2
        sta EGSCORE
        lda EGSCORE+1
        adc T3
        sta EGSCORE+1
        rts
psqsub: sec
        lda MGSCORE
        sbc T0
        sta MGSCORE
        lda MGSCORE+1
        sbc T1
        sta MGSCORE+1
        sec
        lda EGSCORE
        sbc T2
        sta EGSCORE
        lda EGSCORE+1
        sbc T3
        sta EGSCORE+1
        rts

; ---------------------------------------------------------------
; evalinit: recompute accumulators from the board (root setup, and a
; debug cross-check against the incremental path).
; ---------------------------------------------------------------
evalinit:
        lda #0
        sta MGSCORE
        sta MGSCORE+1
        sta EGSCORE
        sta EGSCORE+1
        sta PHASE
        sta PSP0                ; pointer lo bytes are always 0
        sta PSP1
        sta GSLOT
eviloop:
        ldy GSLOT
        lda PIECESQ,y
        cmp #NOSQ
        beq evinext
        tay
        lda a:BOARD,y
        jsr addpiece
evinext:
        inc GSLOT
        lda GSLOT
        cmp #32
        bne eviloop
        rts

; ---------------------------------------------------------------
; eval: SCORE = tapered eval from the side to move's point of view,
; including tempo. score_w = EG + ((MG-EG) * w) >> 5, w = PHASEW[phase].
; Clobbers A,X,Y, T0-T2, MUL0-2, EVTMP, PSQSQ.
; ---------------------------------------------------------------
eval:
        lda PHASE
        cmp #25
        bcc :+
        lda #24                 ; cap (early promotions can exceed 24)
:       tax
        lda PHASEW,x
        sta EVTMP               ; w, 0..32
        ; D = MG - EG, signed
        sec
        lda MGSCORE
        sbc EGSCORE
        sta T0
        lda MGSCORE+1
        sbc EGSCORE+1
        sta T1
        ; sign-magnitude for the multiply
        ldx #0
        lda T1
        bpl evpos
        ldx #1
        sec
        lda #0
        sbc T0
        sta T0
        lda #0
        sbc T1
        sta T1
evpos:  stx PSQSQ               ; sign flag (scratch reuse)
        ; MUL0-2 = T0T1 * w  (shift-add, 6 bits)
        lda #0
        sta MUL0
        sta MUL1
        sta MUL2
        sta T2                  ; third byte of shifting multiplicand
        ldx #6
evmul:  lsr EVTMP
        bcc evnoadd
        clc
        lda MUL0
        adc T0
        sta MUL0
        lda MUL1
        adc T1
        sta MUL1
        lda MUL2
        adc T2
        sta MUL2
evnoadd:
        asl T0
        rol T1
        rol T2
        dex
        bne evmul
        ; >> 5
        ldx #5
evshr:  lsr MUL2
        ror MUL1
        ror MUL0
        dex
        bne evshr
        ; reapply sign
        lda PSQSQ
        beq evnosgn
        sec
        lda #0
        sbc MUL0
        sta MUL0
        lda #0
        sbc MUL1
        sta MUL1
evnosgn:
        ; white score = EG + product
        clc
        lda EGSCORE
        adc MUL0
        sta SCORE
        lda EGSCORE+1
        adc MUL1
        sta SCORE+1
        ; side-to-move POV
        lda SIDE
        beq evwtm
        sec
        lda #0
        sbc SCORE
        sta SCORE
        lda #0
        sbc SCORE+1
        sta SCORE+1
evwtm:  ; tempo
        clc
        lda SCORE
        adc #TEMPO
        sta SCORE
        bcc :+
        inc SCORE+1
:       rts
