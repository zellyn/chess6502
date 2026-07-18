; Board primitives: attacked, make, unmake. See defs.inc for layout.

; TYPEATKTAB[type] = attack-table bits that mean "this piece type attacks
; across this difference (geometrically)". Pawns are special-cased.
TYPEATKTAB:
        .byte 0, 0, ATK_KNIGHT, ATK_DIAG, ATK_ORTHO, ATK_DIAG|ATK_ORTHO, ATK_KING

; ---------------------------------------------------------------
; attacked: is ATSQ attacked by any piece of side ATSIDE (0/$08)?
; Out: carry set if attacked. Clobbers A,X,Y, ATSLOT/ATTMP/ATBITS/DIFF/ATDELTA.
; ---------------------------------------------------------------
attacked:
        lda ATSQ
        clc
        adc #$77
        sta ATT77               ; diff = ATT77 - from, computed per slot
        lda ATSIDE
        asl                     ; slot base: 0 or $10
        sta ATSLOT
atloop: ldy ATSLOT
        lda PIECESQ,y
        cmp #NOSQ
        beq atnext
        sta ATTMP               ; candidate attacker square
        lda ATT77
        sec
        sbc ATTMP
        sta DIFF
        tay
        lda ATTACKTAB,y
        beq atnext              ; no geometric relation at all
        sta ATBITS
        ldx ATTMP
        lda BOARD,x
        and #TYPEMASK
        tax
        cpx #PAWN
        bne atnotpawn
        ; pawn: direction depends on attacker color
        lda ATSIDE
        bne atbpawn
        lda ATBITS
        and #ATK_WPAWN
        bne athit
        beq atnext              ; always
atbpawn:
        lda ATBITS
        and #ATK_BPAWN
        bne athit
        beq atnext              ; always
atnotpawn:
        lda TYPEATKTAB,x
        and ATBITS
        beq atnext
        cpx #KNIGHT
        beq athit
        cpx #KING
        beq athit
        ; slider: walk the ray from attacker toward ATSQ checking blockers
        ldy DIFF
        lda DELTATAB,y
        sta ATDELTA
        lda ATTMP
atwalk: clc
        adc ATDELTA
        cmp ATSQ
        beq athit
        tax
        lda BOARD,x
        bne atnext              ; blocked
        txa
        jmp atwalk
atnext: inc ATSLOT
        lda ATSLOT
        and #$0F
        bne atloop
        clc
        rts
athit:  sec
        rts

; ---------------------------------------------------------------
; slotof: A = piece byte -> Y = slot in PIECESQ. Clobbers A, GTMP.
; slot = index | (color ? 16 : 0)
; ---------------------------------------------------------------
slotof: sta GTMP
        and #COLORMASK
        asl                     ; 0 or $10
        sta CAPSLOT
        lda GTMP
        lsr
        lsr
        lsr
        lsr
        ora CAPSLOT
        tay
        rts

; ---------------------------------------------------------------
; make: play FROM/TO/MVFLAGS. Saves undo state indexed by PLY,
; updates board, piece lists, castling rights, ep, side; PLY++.
; Clobbers A,X,Y and most scratch.
; ---------------------------------------------------------------
make:
        ldx PLY
        lda CASTLE
        sta UNDOCASTLE,x
        lda EPSQ
        sta UNDOEP,x
        lda FROM
        sta UNDOFROM,x
        lda TO
        sta UNDOTO,x
        lda MVFLAGS
        sta UNDOFLAGS,x
.ifndef NOEVAL
        lda MGSCORE
        sta UNDOMGLO,x
        lda MGSCORE+1
        sta UNDOMGHI,x
        lda EGSCORE
        sta UNDOEGLO,x
        lda EGSCORE+1
        sta UNDOEGHI,x
        lda PHASE
        sta UNDOPHASE,x
        lda HALFMOVE
        sta UNDOHALF,x
        lda HASH0
        sta HASHSTK0,x
        lda HASH1
        sta HASHSTK1,x
        lda HASH2
        sta HASHSTK2,x
        lda HASH3
        sta HASHSTK3,x
        lda PSTRUCT
        sta UNDOPSL,x
        lda PSTRUCT+1
        sta UNDOPSH,x
.endif
        ldy FROM
        lda a:BOARD,y           ; force absolute: no lda zp,y mode exists
        sta MVPIECE
        sta UNDOPIECE,x

        ; capture square: TO, or the pushed-past square for en passant
        lda MVFLAGS
        and #FL_EP
        beq mknotep
        lda MVPIECE
        and #COLORMASK
        bne mkepb
        lda TO                  ; white captures: victim is below TO
        sec
        sbc #$10
        jmp mkhavecap
mkepb:  lda TO                  ; black captures: victim is above TO
        clc
        adc #$10
        jmp mkhavecap
mknotep:
        lda TO
mkhavecap:
        sta UNDOCAPSQ,x
        tay
        lda a:BOARD,y           ; victim byte (0 if quiet move)
        sta UNDOCAP,x
        beq mknocap
        ; remove victim: hash+eval out, clear square, tombstone list slot
        pha
.ifndef NOEVAL
        jsr hashpiece           ; A = victim, Y = capture square (kept)
        pla
        pha
        jsr rempiece
        ldx PLY
.endif
        ldy UNDOCAPSQ,x
        lda #0
        sta a:BOARD,y
        pla
        jsr slotof
        lda #NOSQ
        sta PIECESQ,y
mknocap:
.ifndef NOEVAL
        ; 50-move clock: reset on capture or pawn move
        ldx PLY
        lda UNDOCAP,x
        bne mkhmzero
        lda MVPIECE
        and #TYPEMASK
        cmp #PAWN
        beq mkhmzero
        inc HALFMOVE
        jmp mkhmdone
mkhmzero:
        lda #0
        sta HALFMOVE
mkhmdone:
        ; hash+eval: remove the mover from its origin
        lda MVPIECE
        ldy FROM
        jsr hashpiece
        lda MVPIECE
        jsr rempiece            ; hashpiece preserved Y = FROM
.endif
        ; move the piece (promotion replaces the type bits)
        ldy FROM
        lda #0
        sta a:BOARD,y
        lda MVFLAGS
        and #FL_PROMO
        beq mknopromo
        sta GTMP
        lda MVPIECE
        and #INDEXMASK|COLORMASK
        ora GTMP
        bne mkplace             ; always (piece byte nonzero)
mknopromo:
        lda MVPIECE
mkplace:
        sta CRTMP               ; final piece byte (post-promotion)
        ldy TO
        sta a:BOARD,y
.ifndef NOEVAL
        lda CRTMP
        ldy TO
        jsr hashpiece
        lda CRTMP
        jsr addpiece            ; hashpiece preserved Y = TO
.endif
        lda CRTMP
        jsr slotof
        lda TO
        sta PIECESQ,y

        ; castling: also move the rook
        lda MVFLAGS
        and #FL_CASTLE
        beq mknocastle
        jsr castlerook
mknocastle:
        ; rights: CASTLE &= CASTLEMASK[FROM] & CASTLEMASK[TO]
        ldy FROM
        lda CASTLEMASK,y
        ldy TO
        and CASTLEMASK,y
        and CASTLE
        sta CASTLE
        ; ep square: midpoint of a double push, else none
        lda MVFLAGS
        and #FL_DOUBLE
        beq mknodouble
        lda FROM
        clc
        adc TO                  ; never carries for double-push squares
        lsr
        sta EPSQ
        jmp mkflip
mknodouble:
        lda #NOSQ
        sta EPSQ
mkflip:
.ifndef NOEVAL
        ; hash: castling-rights change, ep change, side to move
        ldx PLY
        lda UNDOCASTLE,x
        cmp CASTLE
        beq mknocch
        jsr hashcastle          ; xor out the old rights
        ldx PLY
        lda CASTLE
        jsr hashcastle          ; xor in the new
mknocch:
        ldx PLY
        lda UNDOEP,x
        cmp EPSQ
        beq mknoech
        cmp #NOSQ
        beq :+
        jsr hashep              ; xor out the old ep file
:       lda EPSQ
        cmp #NOSQ
        beq mknoech
        jsr hashep              ; xor in the new
mknoech:
        jsr hashstm
.endif
        lda SIDE
        eor #COLORMASK
        sta SIDE
        inc PLY
.ifndef NOEVAL
        ; refresh the pawn/king structure term if a pawn or king moved
        lda PDIRTY
        beq :+
        lda FEATURES
        and #FT_PSTRUCT
        beq :+
        jmp pawnterm            ; clears PDIRTY; rts returns to caller
:       lda #0
        sta PDIRTY
.endif
        rts

; ---------------------------------------------------------------
; castlerook: move the rook for the castle move being made; TO tells
; which corner. Uses GTMP/GTO as rook from/to. Clobbers A,Y.
; ---------------------------------------------------------------
castlerook:
        lda TO
        cmp #$06                ; g1
        bne crnot1
        lda #$07
        sta GTMP
        lda #$05
        bne crgo                ; always
crnot1: cmp #$02                ; c1
        bne crnot2
        lda #$00
        sta GTMP
        lda #$03
        bne crgo
crnot2: cmp #$76                ; g8
        bne crnot3
        lda #$77
        sta GTMP
        lda #$75
        bne crgo
crnot3: lda #$70                ; c8
        sta GTMP
        lda #$73
crgo:   sta GTO
        ldy GTMP                ; rook from
        lda a:BOARD,y
        sta CRTMP
.ifndef NOEVAL
        jsr hashpiece           ; A = rook byte, Y = from square (kept)
        lda CRTMP
        jsr rempiece
        lda CRTMP
        ldy GTO
        jsr hashpiece
        lda CRTMP
        jsr addpiece
.endif
        ldy GTMP
        lda #0
        sta a:BOARD,y
        ldy GTO
        lda CRTMP
        sta a:BOARD,y
        lda CRTMP
        jsr slotof              ; A = rook byte -> Y = slot
        lda GTO
        sta PIECESQ,y
        rts

; ---------------------------------------------------------------
; uncastlerook: undo the rook move; UNDOTO,x (x=PLY) tells the corner.
; ---------------------------------------------------------------
uncastlerook:
        lda UNDOTO,x
        cmp #$06
        bne ucnot1
        lda #$05
        sta GTMP
        lda #$07
        bne ucgo
ucnot1: cmp #$02
        bne ucnot2
        lda #$03
        sta GTMP
        lda #$00                ; Z is set: must jmp, not branch-always
        jmp ucgo
ucnot2: cmp #$76
        bne ucnot3
        lda #$75
        sta GTMP
        lda #$77
        bne ucgo
ucnot3: lda #$73
        sta GTMP
        lda #$70
ucgo:   sta GTO
        ldy GTMP                ; rook currently here
        lda a:BOARD,y
        sta CRTMP
        lda #0
        sta a:BOARD,y
        ldy GTO
        lda CRTMP
        sta a:BOARD,y
        jsr slotof              ; A = rook byte -> Y = slot
        lda GTO
        sta PIECESQ,y
        rts

; ---------------------------------------------------------------
; unmake: undo the move recorded at PLY-1. Restores side, castle, ep,
; board, piece lists.
; ---------------------------------------------------------------
unmake:
        dec PLY
        ldx PLY
        lda SIDE
        eor #COLORMASK
        sta SIDE
        lda UNDOCASTLE,x
        sta CASTLE
        lda UNDOEP,x
        sta EPSQ
.ifndef NOEVAL
        lda UNDOMGLO,x
        sta MGSCORE
        lda UNDOMGHI,x
        sta MGSCORE+1
        lda UNDOEGLO,x
        sta EGSCORE
        lda UNDOEGHI,x
        sta EGSCORE+1
        lda UNDOPHASE,x
        sta PHASE
        lda UNDOHALF,x
        sta HALFMOVE
        lda HASHSTK0,x
        sta HASH0
        lda HASHSTK1,x
        sta HASH1
        lda HASHSTK2,x
        sta HASH2
        lda HASHSTK3,x
        sta HASH3
        lda UNDOPSL,x
        sta PSTRUCT
        lda UNDOPSH,x
        sta PSTRUCT+1
.endif
        ; clear TO, put the original piece byte back on FROM
        ldy UNDOTO,x
        lda #0
        sta a:BOARD,y
        lda UNDOPIECE,x
        ldy UNDOFROM,x
        sta a:BOARD,y
        jsr slotof              ; Y = mover's slot
        lda UNDOFROM,x
        sta PIECESQ,y
        ; castle: move the rook back
        lda UNDOFLAGS,x
        and #FL_CASTLE
        beq umnocastle
        jsr uncastlerook
umnocastle:
        ; restore any captured piece
        lda UNDOCAP,x
        beq umnocap
        pha
        ldy UNDOCAPSQ,x
        sta a:BOARD,y
        jsr slotof
        lda UNDOCAPSQ,x
        sta PIECESQ,y
        pla
umnocap:
        rts
