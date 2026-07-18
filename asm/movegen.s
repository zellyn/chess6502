; Move generator: pseudo-legal moves for SIDE, appended to the move stack
; at MSP as (from,to,flags) triples. Castling emits only if the king's
; start and through squares are safe; the landing square is covered by the
; caller's make/in-check filter like every other move.

; ---------------------------------------------------------------
; emitmove: A = flags; GFROM/GTO = squares. Advances MSP.
; ---------------------------------------------------------------
emitmove:
        ldy #2
        sta (MSP),y
        dey
        lda GTO
        sta (MSP),y
        dey
        lda GFROM
        sta (MSP),y
        lda MSP
        clc
        adc #3
        sta MSP
        bcc emok
        inc MSP+1
emok:   lda MSP+1
        cmp #>MOVESTACKTOP
        bcc emok2
        lda #100                ; move-stack overflow: abort the run
        sta EXIT_TRAP
emok2:  rts

; ---------------------------------------------------------------
; generate: all pseudo-legal moves for SIDE.
; ---------------------------------------------------------------
generate:
        lda SIDE
        asl
        sta GSLOT
genloop:
        ldy GSLOT
        lda PIECESQ,y
        cmp #NOSQ
        beq gennext
        sta GFROM
        tax
        lda BOARD,x
        sta GPIECE
        and #TYPEMASK
        cmp #PAWN
        bne :+
        jmp genpawn
:       cmp #KNIGHT
        bne :+
        jmp genknight
:       cmp #KING
        bne :+
        jmp genking
:       cmp #BISHOP
        beq genbishop
        cmp #ROOK
        beq genrook
        ; queen: all 8 directions
        lda #<KINGOFF
        sta CURPTR
        lda #>KINGOFF
        sta CURPTR+1
        lda #8
        bne genslide            ; always
genbishop:
        lda #<DIAGOFF
        sta CURPTR
        lda #>DIAGOFF
        sta CURPTR+1
        lda #4
        bne genslide
genrook:
        lda #<ORTHOOFF
        sta CURPTR
        lda #>ORTHOOFF
        sta CURPTR+1
        lda #4
        bne genslide
gennext:
        inc GSLOT
        lda GSLOT
        and #$0F
        bne genloop
        rts

; ---- sliders: A = direction count; CURPTR -> offset table ----
genslide:
        sta GCOUNT
        lda #0
        sta GDIR
gsdirloop:
        ldy GDIR
        lda (CURPTR),y
        sta GDELTA
        lda GFROM
gswalk: clc
        adc GDELTA
        sta GTO
        and #$88
        bne gsnextdir           ; off board
        ldx GTO
        lda BOARD,x
        beq gsempty
        eor SIDE                ; enemy iff color bit differs
        and #COLORMASK
        beq gsnextdir           ; own piece: stop
        lda #0
        jsr emitmove            ; capture: emit, then stop
        jmp gsnextdir
gsempty:
        lda #0
        jsr emitmove
        lda GTO
        jmp gswalk
gsnextdir:
        inc GDIR
        lda GDIR
        cmp GCOUNT
        bne gsdirloop
        jmp gennext

; ---- knight / king: 8 single steps ----
genknight:
        lda #<KNIGHTOFF
        sta CURPTR
        lda #>KNIGHTOFF
        sta CURPTR+1
        bne genstep             ; always
genking:
        lda #<KINGOFF
        sta CURPTR
        lda #>KINGOFF
        sta CURPTR+1
genstep:
        lda #0
        sta GDIR
gstloop:
        ldy GDIR
        lda (CURPTR),y
        clc
        adc GFROM
        sta GTO
        and #$88
        bne gstnext
        ldx GTO
        lda BOARD,x
        beq gstemit
        eor SIDE
        and #COLORMASK
        beq gstnext             ; own piece
gstemit:
        lda #0
        jsr emitmove
gstnext:
        inc GDIR
        lda GDIR
        cmp #8
        bne gstloop
        ; kings also try castling
        lda GPIECE
        and #TYPEMASK
        cmp #KING
        beq gencastle
        jmp gennext

; ---- castling ----
; Emits if: rights bit set, between-squares empty, king square and the
; square the king passes through not attacked by the enemy.
gencastle:
        lda SIDE
        bne gcblack
        ; white kingside: e1(4) f1 g1
        lda CASTLE
        and #CR_WK
        beq gcwq
        ldx #$05
        lda BOARD,x
        bne gcwq
        ldx #$06
        lda BOARD,x
        bne gcwq
        lda #$04
        ldy #$05
        jsr gcsafe2
        bcs gcwq
        lda #$04
        sta GFROM
        lda #$06
        sta GTO
        lda #FL_CASTLE
        jsr emitmove
gcwq:   ; white queenside: e1 d1 c1 b1
        lda CASTLE
        and #CR_WQ
        beq gcdone
        ldx #$03
        lda BOARD,x
        bne gcdone
        ldx #$02
        lda BOARD,x
        bne gcdone
        ldx #$01
        lda BOARD,x
        bne gcdone
        lda #$04
        ldy #$03
        jsr gcsafe2
        bcs gcdone
        lda #$04
        sta GFROM
        lda #$02
        sta GTO
        lda #FL_CASTLE
        jsr emitmove
gcdone: jmp gennext

gcblack:
        ; black kingside: e8($74) f8 g8
        lda CASTLE
        and #CR_BK
        beq gcbq
        ldx #$75
        lda BOARD,x
        bne gcbq
        ldx #$76
        lda BOARD,x
        bne gcbq
        lda #$74
        ldy #$75
        jsr gcsafe2
        bcs gcbq
        lda #$74
        sta GFROM
        lda #$76
        sta GTO
        lda #FL_CASTLE
        jsr emitmove
gcbq:   ; black queenside: e8 d8 c8 b8
        lda CASTLE
        and #CR_BQ
        beq gcbdone
        ldx #$73
        lda BOARD,x
        bne gcbdone
        ldx #$72
        lda BOARD,x
        bne gcbdone
        ldx #$71
        lda BOARD,x
        bne gcbdone
        lda #$74
        ldy #$73
        jsr gcsafe2
        bcs gcbdone
        lda #$74
        sta GFROM
        lda #$72
        sta GTO
        lda #FL_CASTLE
        jsr emitmove
gcbdone:
        jmp gennext

; gcsafe2: carry set if either square A or square Y is attacked by the
; side NOT to move. Clobbers attacked()'s scratch.
gcsafe2:
        sta ATSQ
        sty GTMP
        lda SIDE
        eor #COLORMASK
        sta ATSIDE
        jsr attacked
        bcs gcs2out
        lda GTMP
        sta ATSQ
        jsr attacked
gcs2out:
        rts

; ---- pawns ----
genpawn:
        lda GPIECE
        and #COLORMASK
        bne genbpawn

        ; ---- white pawn: moves up (+$10) ----
        lda GFROM
        clc
        adc #$10
        sta GTO
        tax
        lda BOARD,x
        bne gwcaps              ; blocked: no pushes
        lda GFROM
        and #$F0
        cmp #$60
        beq gwpromopush
        lda #0
        jsr emitmove
        ; double push from rank 2
        lda GFROM
        and #$F0
        cmp #$10
        bne gwcaps
        lda GTO
        clc
        adc #$10
        tax
        lda BOARD,x
        bne gwcaps
        stx GTO
        lda #FL_DOUBLE
        jsr emitmove
        jmp gwcaps
gwpromopush:
        jsr promoloop
gwcaps: lda #$0F
        jsr gwcap1
        lda #$11
        jsr gwcap1
        jmp gennext

; gwcap1: A = capture offset for a white pawn.
gwcap1: clc
        adc GFROM
        sta GTO
        and #$88
        bne gwc1out
        ldx GTO
        lda BOARD,x
        beq gwc1ep
        eor SIDE
        and #COLORMASK
        beq gwc1out             ; own piece
        lda GTO
        and #$F0
        cmp #$70
        beq gwc1promo
        lda #0
        jmp emitmove
gwc1promo:
        jmp promoloop
gwc1ep: lda GTO
        cmp EPSQ
        bne gwc1out
        lda #FL_EP
        jmp emitmove
gwc1out:
        rts

genbpawn:
        ; ---- black pawn: moves down (-$10) ----
        lda GFROM
        sec
        sbc #$10
        sta GTO
        tax
        lda BOARD,x
        bne gbcaps
        lda GFROM
        and #$F0
        cmp #$10
        beq gbpromopush
        lda #0
        jsr emitmove
        ; double push from rank 7
        lda GFROM
        and #$F0
        cmp #$60
        bne gbcaps
        lda GTO
        sec
        sbc #$10
        tax
        lda BOARD,x
        bne gbcaps
        stx GTO
        lda #FL_DOUBLE
        jsr emitmove
        jmp gbcaps
gbpromopush:
        jsr promoloop
gbcaps: lda #$F1                ; -$0F
        jsr gbcap1
        lda #$EF                ; -$11
        jsr gbcap1
        jmp gennext

gbcap1: clc
        adc GFROM
        sta GTO
        and #$88
        bne gbc1out
        ldx GTO
        lda BOARD,x
        beq gbc1ep
        eor SIDE
        and #COLORMASK
        beq gbc1out
        lda GTO
        and #$F0
        bne gbc1nopromo         ; promotion only on rank 1 ($00-$07)
        jmp promoloop
gbc1nopromo:
        lda #0
        jmp emitmove
gbc1ep: lda GTO
        cmp EPSQ
        bne gbc1out
        lda #FL_EP
        jmp emitmove
gbc1out:
        rts

; promoloop: emit N/B/R/Q promotions for GFROM/GTO.
promoloop:
        lda #KNIGHT
        sta PROMTYPE
prloop: lda PROMTYPE
        jsr emitmove
        inc PROMTYPE
        lda PROMTYPE
        cmp #QUEEN+1
        bne prloop
        rts
