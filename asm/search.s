; Negamax alpha-beta search with quiescence.
;
; Node protocol: caller sets ALPHA/BETA at the child's ply, then calls
; search with PLY already advanced by make. SCORE returns the fail-hard
; result from the side-to-move's POV. Full-width until PLY >= MAXDEPTH,
; then quiescence: stand pat + captures + queen promotions; when in
; check, a full evasion node instead (no stand pat, all moves, mate
; detection) — which is exactly a full-width node, so it reuses one.
;
; Move loop runs two passes over the generated list: pass 0 processes
; captures/promotions, pass 1 the quiets. QS capture-only nodes stop
; after pass 0.

; ---------------------------------------------------------------
; gennode: set base/cursor from MSP, generate, set end.
; ---------------------------------------------------------------
gennode:
        ldy PLY
        lda MSP
        sta PLYBASELO,y
        sta CURSORLO,y
        lda MSP+1
        sta PLYBASEHI,y
        sta CURSORHI,y
        jsr generate
        ldy PLY
        lda MSP
        sta PLYENDLO,y
        lda MSP+1
        sta PLYENDHI,y
        rts

; ---------------------------------------------------------------
; curincheck: carry set if the side to move is in check.
; ---------------------------------------------------------------
curincheck:
        lda SIDE
        eor #COLORMASK
        sta ATSIDE
        lda SIDE
        asl
        tay
        lda PIECESQ,y           ; own king (slot 0 of own base)
        sta ATSQ
        jmp attacked

; ---------------------------------------------------------------
; search
; ---------------------------------------------------------------
search:
        lda PLY
        cmp #MAXPLY-1
        bcc :+
        jmp eval                ; hard ply cap: static eval
:       ldy PLY
        lda #0
        sta LEGALCNT,y
        sta PASSNO,y
        sta QSKIND,y
        lda PLY
        cmp MAXDEPTH
        bcc snode               ; full-width node
        ; quiescence entry
        jsr curincheck
        bcs snode               ; in check: full evasion node
        ldy PLY
        lda #1
        sta QSKIND,y
        jsr eval
        ldy PLY
        ; stand pat: if SCORE >= BETA return BETA
        sec
        lda SCORE
        sbc BETALO,y
        lda SCORE+1
        sbc BETAHI,y
        bvc :+
        eor #$80
:       bmi qsnofh
        lda BETALO,y
        sta SCORE
        lda BETAHI,y
        sta SCORE+1
        rts
qsnofh: ; if SCORE > ALPHA: ALPHA = SCORE
        sec
        lda ALPHALO,y
        sbc SCORE
        lda ALPHAHI,y
        sbc SCORE+1
        bvc :+
        eor #$80
:       bpl snode
        lda SCORE
        sta ALPHALO,y
        lda SCORE+1
        sta ALPHAHI,y

snode:  ldy PLY
        lda QSKIND,y            ; qs capture-only nodes generate only captures
        sta GENCAPS
        jsr gennode
        lda #0
        sta GENCAPS
sloop:  ldy PLY
        lda CURSORLO,y
        cmp PLYENDLO,y
        bne sfetch
        lda CURSORHI,y
        cmp PLYENDHI,y
        bne sfetch
        ; end of list: run pass 1 unless done or qs-captures-only
        lda PASSNO,y
        beq :+
        jmp sdone
:       lda QSKIND,y
        beq :+
        jmp sdone
:       lda #1
        sta PASSNO,y
        lda PLYBASELO,y
        sta CURSORLO,y
        lda PLYBASEHI,y
        sta CURSORHI,y
        jmp sloop
sfetch: lda CURSORLO,y
        sta CURPTR
        lda CURSORHI,y
        sta CURPTR+1
        ; advance cursor now, so skipping a move is just "jmp sloop"
        lda CURSORLO,y
        clc
        adc #3
        sta CURSORLO,y
        bcc :+
        lda CURSORHI,y
        adc #0
        sta CURSORHI,y
:       ldy #0
        lda (CURPTR),y
        sta FROM
        iny
        lda (CURPTR),y
        sta TO
        iny
        lda (CURPTR),y
        sta MVFLAGS
        ; classify: capture/promotion vs quiet
        ldx TO
        lda BOARD,x
        bne siscap
        lda MVFLAGS
        and #FL_EP|FL_PROMO
        bne siscap
        ; quiet: only in pass 1
        ldy PLY
        lda PASSNO,y
        beq sloopj              ; pass 0: skip
        bne sdomove             ; always
siscap: ldy PLY
        lda PASSNO,y
        bne sloopj              ; pass 1: already searched
        ; qs nodes: queen promotions only
        lda QSKIND,y
        beq sdomove
        lda MVFLAGS
        and #FL_PROMO
        beq sdomove
        cmp #QUEEN
        bne sloopj
sdomove:
        jsr make
        ; legality: mover must not leave their king attacked
        lda SIDE
        sta ATSIDE
        eor #COLORMASK
        asl
        tay
        lda PIECESQ,y
        sta ATSQ
        jsr attacked
        bcc slegal
        jsr unmake
sloopj: jmp sloop
slegal: ldy PLY                 ; PLY = child here
        dey
        lda LEGALCNT,y
        clc
        adc #1
        sta LEGALCNT,y
        iny
        ; child window: ALPHA[c] = -BETA[p], BETA[c] = -ALPHA[p]
        sec
        lda #0
        sbc BETALO-1,y
        sta ALPHALO,y
        lda #0
        sbc BETAHI-1,y
        sta ALPHAHI,y
        sec
        lda #0
        sbc ALPHALO-1,y
        sta BETALO,y
        lda #0
        sbc ALPHAHI-1,y
        sta BETAHI,y
        jsr search
        sec                     ; SCORE = -SCORE
        lda #0
        sbc SCORE
        sta SCORE
        lda #0
        sbc SCORE+1
        sta SCORE+1
        jsr unmake
        ; beta cutoff?
        ldy PLY
        sec
        lda SCORE
        sbc BETALO,y
        lda SCORE+1
        sbc BETAHI,y
        bvc :+
        eor #$80
:       bmi snocut              ; SCORE < BETA
        lda BETALO,y            ; fail-hard: return BETA
        sta SCORE
        lda BETAHI,y
        sta SCORE+1
        jmp spop
snocut: ; alpha improvement? (strict >)
        sec
        lda ALPHALO,y
        sbc SCORE
        lda ALPHAHI,y
        sbc SCORE+1
        bvc :+
        eor #$80
:       bpl sloopj              ; ALPHA >= SCORE
        lda SCORE
        sta ALPHALO,y
        lda SCORE+1
        sta ALPHAHI,y
        cpy #0
        beq :+
        jmp sloop
:       ; root: remember the move (cursor was already advanced by 3)
        lda CURSORLO,y
        sec
        sbc #3
        sta CURPTR
        lda CURSORHI,y
        sbc #0
        sta CURPTR+1
        ldy #0
        lda (CURPTR),y
        sta BESTFROM
        iny
        lda (CURPTR),y
        sta BESTTO
        iny
        lda (CURPTR),y
        sta BESTFLAGS
        jmp sloop

sdone:  ; return alpha; full-width nodes with no legal moves: mate/stalemate
        ldy PLY
        lda QSKIND,y
        bne sret
        lda LEGALCNT,y
        bne sret
        jsr curincheck
        bcs smated
        lda #0                  ; stalemate
        sta SCORE
        sta SCORE+1
        jmp spop
smated: lda PLY                 ; SCORE = PLY - MATE (mated here)
        sec
        sbc #<MATE
        sta SCORE
        lda #0
        sbc #>MATE
        sta SCORE+1
        jmp spop
sret:   ldy PLY
        lda ALPHALO,y
        sta SCORE
        lda ALPHAHI,y
        sta SCORE+1
spop:   ldy PLY
        lda PLYBASELO,y
        sta MSP
        lda PLYBASEHI,y
        sta MSP+1
        rts
