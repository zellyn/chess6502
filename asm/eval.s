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
        cpx #PAWN               ; pawn/king changes invalidate PSTRUCT
        beq :+
        cpx #KING
        bne :++
:       stx PDIRTY              ; (nonzero type byte)
:
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
; hashpiece: xor the Zobrist key for (A = piece byte, Y = square) into
; HASH0-3. Y is preserved. Clobbers A,X.
; ---------------------------------------------------------------
hashpiece:
        and #$0F
        tax
        lda KINDTAB,x
        tax                     ; kind 0-11
        lda ZPLLO,x
        sta ZPTR
        lda ZPLHI0,x
        sta ZPTR+1
        lda (ZPTR),y
        eor HASH0
        sta HASH0
        lda ZPLHI1,x
        sta ZPTR+1
        lda (ZPTR),y
        eor HASH1
        sta HASH1
        lda ZPLHI2,x
        sta ZPTR+1
        lda (ZPTR),y
        eor HASH2
        sta HASH2
        lda ZPLHI3,x
        sta ZPTR+1
        lda (ZPTR),y
        eor HASH3
        sta HASH3
        rts

; hashcastle: xor CASTKEYS[A] into HASH0-3. Clobbers A,X,Y.
hashcastle:
        asl
        asl
        tay
        ldx #0
hcloop: lda CASTKEYS,y
        eor HASH0,x
        sta HASH0,x
        iny
        inx
        cpx #4
        bne hcloop
        rts

; hashep: xor EPKEYS[file of A] into HASH0-3 (A = ep square, not NOSQ).
hashep:
        and #$07
        asl
        asl
        tay
        ldx #0
heloop: lda EPKEYS,y
        eor HASH0,x
        sta HASH0,x
        iny
        inx
        cpx #4
        bne heloop
        rts

; hashstm (the side-to-move xor) is emitted unrolled by cmd/gentables,
; since only tables.s knows the key bytes at assembly time.

; ---------------------------------------------------------------
; pawnterm: recompute PSTRUCT (white POV): doubled/isolated/passed
; pawns and a minimal king shield. Called by make when PDIRTY is set
; (a pawn or king changed) and by evalinit; ~800-1200 cycles, but pawn/
; king events are a small fraction of makes. Uses $0200-$022F scratch.
;
; Terms (v1, to be SPRT-tuned): doubled -12 per extra pawn on a file;
; isolated -12; passed bonus by rank; king shield +8 per own pawn on
; the three files around a back-rank king (max 3 counted via files),
; -10 for an open own file under the king.
; ---------------------------------------------------------------
PWCNT  = $0200          ; white pawns per file (8)
PBCNT  = $0208          ; black pawns per file (8)
PWMAX  = $0210          ; highest white pawn rank per file (0 if none)
PBMAX  = $0218          ; highest black pawn rank per file (0 if none)
PWMIN  = $0220          ; lowest white pawn rank per file (15 if none)
PBMIN  = $0228          ; lowest black pawn rank per file (15 if none)

PASSEDBONUS:
        .byte 0, 8, 12, 18, 28, 45, 70, 0

pawnterm:
        lda #0
        sta PDIRTY
        sta PSTRUCT
        sta PSTRUCT+1
        ; clear the per-file tables
        ldx #7
ptclr:  sta PWCNT,x
        sta PBCNT,x
        sta PWMAX,x
        sta PBMAX,x
        lda #15
        sta PWMIN,x
        sta PBMIN,x
        lda #0
        dex
        bpl ptclr
        ; scan the piece lists for pawns
        ldx #31
ptscan: lda PIECESQ,x
        cmp #NOSQ
        beq ptnext
        tay
        lda a:BOARD,y
        and #TYPEMASK
        cmp #PAWN
        bne ptnext
        tya
        and #$07
        sta EVTMP               ; file
        tya
        lsr
        lsr
        lsr
        lsr
        sta MULCNT              ; rank
        cpx #16
        bcs ptblack
        ldy EVTMP
        lda PWCNT,y
        clc
        adc #1
        sta PWCNT,y
        lda MULCNT
        cmp PWMAX,y
        bcc :+
        sta PWMAX,y
:       lda MULCNT
        cmp PWMIN,y
        bcs ptnext
        sta PWMIN,y
        jmp ptnext
ptblack:
        ldy EVTMP
        lda PBCNT,y
        clc
        adc #1
        sta PBCNT,y
        lda MULCNT
        cmp PBMAX,y
        bcc :+
        sta PBMAX,y
:       lda MULCNT
        cmp PBMIN,y
        bcs ptnext
        sta PBMIN,y
ptnext: dex
        bpl ptscan

        ; per-file terms into PSTRUCT (T0/T1 as signed accumulator)
        lda #0
        sta T0
        sta T1
        ldx #7
ptfile: ; doubled: -12 per extra pawn (white subtracts, black adds)
        lda PWCNT,x
        beq ptwd0
        sec
        sbc #1
        beq ptwiso
        jsr ptsub12
ptwiso: ; isolated white pawn(s): neighbors empty
        jsr ptneighw
        bne ptwpass
        jsr ptsub12
ptwpass:
        ; white passed pawn (use the most advanced on the file):
        ; no black pawn strictly ahead on x-1, x, x+1
        lda PWMAX,x
        sta EVTMP               ; r
        jsr ptbmax3             ; A = max black rank on x-1..x+1
        cmp EVTMP
        bcs ptwd0               ; a blocker at rank > r ... (>= r is
                                ;  conservative: adjacent same-rank
                                ;  enemy pawns still guard the path)
        ldy EVTMP
        lda PASSEDBONUS,y
        jsr ptadda
ptwd0:  ; black side, mirrored (ranks flipped: advancement = low rank)
        lda PBCNT,x
        beq ptnextf
        sec
        sbc #1
        beq ptbiso
        jsr ptadd12
ptbiso: jsr ptneighb
        bne ptbpass
        jsr ptadd12
ptbpass:
        lda PBMIN,x
        sta EVTMP
        jsr ptwmin3             ; A = min white rank on x-1..x+1
        cmp EVTMP
        beq ptnextf
        bcc ptnextf             ; a white blocker at rank < r
        lda #7
        sec
        sbc EVTMP               ; black advancement = 7 - rank
        tay
        lda PASSEDBONUS,y
        jsr ptsuba
ptnextf:
        dex
        bmi ptkings
        jmp ptfile

ptkings:
        ; king shield: only for kings on their own back two ranks
        ldy PIECESQ+0           ; white king
        tya
        and #$70
        bne ptbk                ; not on rank 1 (shield only when home-ish)
        tya
        and #$07
        jsr ptshieldw
ptbk:   ldy PIECESQ+16          ; black king
        tya
        and #$70
        cmp #$70
        bne ptdone
        tya
        and #$07
        jsr ptshieldb
ptdone: lda T0
        sta PSTRUCT
        lda T1
        sta PSTRUCT+1
        rts

; helpers: T0/T1 16-bit signed accumulator ---------------------------
ptadda: clc                     ; A (unsigned small) added
        adc T0
        sta T0
        bcc :+
        inc T1
:       rts
ptsuba: sta EVTMP
        sec
        lda T0
        sbc EVTMP
        sta T0
        bcs :+
        dec T1
:       rts
ptadd12:
        lda #12
        bne ptadda              ; always
ptsub12:
        lda #12
        bne ptsuba              ; always

; ptneighw/b: Z set if both neighbor files have no own pawns
ptneighw:
        lda #0
        cpx #0
        beq :+
        ora PWCNT-1,x
:       cpx #7
        beq :+
        ora PWCNT+1,x
:       rts
ptneighb:
        lda #0
        cpx #0
        beq :+
        ora PBCNT-1,x
:       cpx #7
        beq :+
        ora PBCNT+1,x
:       rts

; ptbmax3: A = max(PBMAX[x-1], PBMAX[x], PBMAX[x+1])
ptbmax3:
        lda PBMAX,x
        cpx #0
        beq :+
        cmp PBMAX-1,x
        bcs :+
        lda PBMAX-1,x
:       cpx #7
        beq :+
        cmp PBMAX+1,x
        bcs :+
        lda PBMAX+1,x
:       rts
; ptwmin3: A = min(PWMIN[x-1], PWMIN[x], PWMIN[x+1])
ptwmin3:
        lda PWMIN,x
        cpx #0
        beq :+
        cmp PWMIN-1,x
        bcc :+
        lda PWMIN-1,x
:       cpx #7
        beq :+
        cmp PWMIN+1,x
        bcc :+
        lda PWMIN+1,x
:       rts

; ptshieldw/b: A = king file; +8 per shielded file, -10 for an open
; own file under the king. Clobbers Y, EVTMP.
ptshieldw:
        sta EVTMP
        tay
        lda PWCNT,y
        beq :+
        lda #8
        jsr ptadda
:       ldy EVTMP
        beq :+                  ; file a: no left neighbor
        lda PWCNT-1,y
        beq :+
        lda #8
        jsr ptadda
:       ldy EVTMP
        cpy #7
        beq :+
        lda PWCNT+1,y
        beq :+
        lda #8
        jsr ptadda
:       ldy EVTMP
        lda PWCNT,y
        bne :+
        lda #10                 ; open file under the king
        jsr ptsuba
:       rts
ptshieldb:
        sta EVTMP
        tay
        lda PBCNT,y
        beq :+
        lda #8
        jsr ptsuba
:       ldy EVTMP
        beq :+
        lda PBCNT-1,y
        beq :+
        lda #8
        jsr ptsuba
:       ldy EVTMP
        cpy #7
        beq :+
        lda PBCNT+1,y
        beq :+
        lda #8
        jsr ptsuba
:       ldy EVTMP
        lda PBCNT,y
        bne :+
        lda #10
        jsr ptadda
:       rts

; ---------------------------------------------------------------
; evalinit: recompute accumulators and the Zobrist hash from the board
; (root setup, and a debug cross-check against the incremental path).
; ---------------------------------------------------------------
evalinit:
        lda #0
        sta MGSCORE
        sta MGSCORE+1
        sta EGSCORE
        sta EGSCORE+1
        sta PHASE
        sta HASH0
        sta HASH1
        sta HASH2
        sta HASH3
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
        ldy GSLOT
        lda PIECESQ,y
        tay
        lda a:BOARD,y
        jsr hashpiece
evinext:
        inc GSLOT
        lda GSLOT
        cmp #32
        bne eviloop
        ; side to move, castling rights, ep square
        lda SIDE
        beq :+
        jsr hashstm
:       lda CASTLE
        jsr hashcastle
        lda EPSQ
        cmp #NOSQ
        beq :+
        jsr hashep
:       jmp pawnterm            ; initial PSTRUCT (clears PDIRTY)

; ---------------------------------------------------------------
; eval: SCORE = tapered eval from the side to move's point of view,
; including tempo. score_w = EG + ((MG-EG) * w) >> 5, w = PHASEW[phase].
; Clobbers A,X,Y, T0-T1, MUL0-1, EVTMP, PSQSQ.
; ---------------------------------------------------------------
eval:
        lda PHASE
        cmp #25
        bcc :+
        lda #24                 ; cap (early promotions can exceed 24)
:       tax
        lda PHASEW,x
        sta EVTMP               ; w, 0..32
        ; fast paths: w=32 (full middlegame) is pure MG, w=0 pure EG —
        ; no multiply needed. w=32 covers every opening/middlegame node.
        cmp #32
        bne :+
        lda MGSCORE
        sta SCORE
        lda MGSCORE+1
        sta SCORE+1
        jmp evpov
:       cmp #0
        bne :+
        lda EGSCORE
        sta SCORE
        lda EGSCORE+1
        sta SCORE+1
        jmp evpov
:       ; D = MG - EG, signed
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
        ; MUL1:MUL0 = (T1:T0 * w) >> 5 via right-shift multiply.
        ; w is 1..31 here (w=32/w=0 fast-pathed above), so 5 iterations
        ; produce the >>5 for free: the shifted-out bits are exactly
        ; the product's low 5 bits. T1:T0 is a magnitude < $8000, so
        ; the accumulator never exceeds 17 bits (carry + 16).
        lda #0
        sta MUL0
        sta MUL1
        ldx #5
evmul:  lsr EVTMP
        bcc evskip              ; bit clear: carry clear into the ror
        clc                     ; carry is set here (from the lsr)
        lda MUL0
        adc T0
        sta MUL0
        lda MUL1
        adc T1
        sta MUL1                ; adc carry-out falls into the ror
evskip: ror MUL1
        ror MUL0
        dex
        bne evmul
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
evpov:  ; pawn-structure/king-shield term (white POV, kept current by
        ; make via PDIRTY)
        lda FEATURES
        and #FT_PSTRUCT
        beq :+
        clc
        lda SCORE
        adc PSTRUCT
        sta SCORE
        lda SCORE+1
        adc PSTRUCT+1
        sta SCORE+1
:       ; side-to-move POV
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
:       ; dither: 0-3cp of seeded noise breaks deterministic move
        ; repetition (hardware seeds SEED from input timing; the bridge
        ; pokes a random byte; 0 = off, keeping tests reproducible)
        lda SEED
        beq evdone
        asl
        clc
        adc SEED                ; seed = seed*3 + 29
        adc #29
        sta SEED
        and #$03
        clc
        adc SCORE
        sta SCORE
        bcc evdone
        inc SCORE+1
evdone: rts
