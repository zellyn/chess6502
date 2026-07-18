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
; checkclock: poll the harness clock; set ABORT once cycles reach the
; hard limit (2x budget). No-op in fixed-depth mode (budget 0).
; ---------------------------------------------------------------
checkclock:
        lda BUDGET0
        ora BUDGET1
        ora BUDGET2
        beq ccout
        lda CLOCK_TRAP          ; latches all three bytes
        cmp ABORTL0
        lda CLOCK_TRAP+1
        sbc ABORTL1
        lda CLOCK_TRAP+2
        sbc ABORTL2
        bcc ccout               ; still under the limit
        lda #1
        sta ABORT
ccout:  rts

; ---------------------------------------------------------------
; search
; ---------------------------------------------------------------
search:
        inc NODECNT
        bne :+
        jsr checkclock
:       lda ABORT
        beq :+
        lda #0                  ; aborting: unwind with a dummy score
        sta SCORE
        sta SCORE+1
        rts
:       lda PLY
        cmp #MAXPLY-1
        bcc :+
        jmp eval                ; hard ply cap: static eval
:       lda PLY
        beq sdrawend            ; root: no draw checks; a move is required
        ; 50-move rule. (Nuance accepted: a mate delivered exactly on the
        ; 100th halfmove is scored as a draw here.)
        lda HALFMOVE
        cmp #100
        bcs sdraw
        ; twofold repetition against the search path: only reachable
        ; within the last HALFMOVE plies, same side to move (step 2)
        cmp #4
        bcc snorep
        lda PLY
        sec
        sbc HALFMOVE
        bcs :+
        lda #0
:       sta T2                  ; scan lower bound
        lda PLY
        sec
        sbc #2
        bcc snorep
sreploop:
        cmp T2
        bcc snorep
        tax
        lda HASHSTK0,x
        cmp HASH0
        bne srepnext
        lda HASHSTK1,x
        cmp HASH1
        bne srepnext
        lda HASHSTK2,x
        cmp HASH2
        bne srepnext
        lda HASHSTK3,x
        cmp HASH3
        bne srepnext
        beq sdraw               ; repetition
srepnext:
        txa
        sec
        sbc #2
        bcs sreploop
snorep:
        ; insufficient material: PHASE <= 1 and no pawns (covers KK,
        ; KNK, KBK; same-color-bishops draws are the referee's problem)
        lda PHASE
        cmp #2
        bcs sdrawend
        ldx #31
smdloop:
        lda PIECESQ,x
        cmp #NOSQ
        beq smdnext
        tay
        lda a:BOARD,y
        and #TYPEMASK
        cmp #PAWN
        beq sdrawend            ; a pawn exists: playable
smdnext:
        dex
        bpl smdloop
sdraw:  lda #0
        sta SCORE
        sta SCORE+1
        rts
sdrawend:
        ldy PLY
        lda #0
        sta LEGALCNT,y
        sta QSKIND,y
        sta RAISED,y
        lda #NOSQ
        sta TTFROMA,y
        sta TTBF,y
        lda PLY
        cmp MAXDEPTH
        bcs squiesce            ; quiescence entry below
        ; full-width node: probe the transposition table
        jsr ttprobe
        bcc snodej
        ldy PLY
        lda TTENTRY+3
        sta TTFROMA,y           ; TT move: searched first (pass 0)
        lda TTENTRY+4
        sta TTTOA,y
        ; cutoff allowed if stored depth >= remaining depth, not at root
        lda PLY
        beq snodej
        lda TTENTRY+7
        lsr
        lsr
        sta T0
        lda MAXDEPTH
        sec
        sbc PLY
        cmp T0
        bcc ttcut               ; remaining < stored depth
        beq ttcut               ; equal: cutoff ok
snodej: jmp snode               ; otherwise ordering only
ttcut:  lda TTENTRY+7
        and #$03
        cmp #TT_EXACT
        beq ttexact
        cmp #TT_LOWER
        beq ttlower
        ; upper bound: usable if score <= alpha
        sec
        lda TTENTRY+5
        sbc ALPHALO,y
        lda TTENTRY+6
        sbc ALPHAHI,y
        bvc :+
        eor #$80
:       bpl snode               ; score > alpha: not usable
        lda ALPHALO,y
        sta SCORE
        lda ALPHAHI,y
        sta SCORE+1
        rts
ttlower:                        ; usable if score >= beta
        sec
        lda TTENTRY+5
        sbc BETALO,y
        lda TTENTRY+6
        sbc BETAHI,y
        bvc :+
        eor #$80
:       bmi snode               ; score < beta: not usable
        lda BETALO,y
        sta SCORE
        lda BETAHI,y
        sta SCORE+1
        rts
ttexact:
        lda TTENTRY+5
        sta SCORE
        lda TTENTRY+6
        sta SCORE+1
        rts

squiesce:
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
        ; initial pass: 0 (TT move first) when we have one and this is
        ; not a qs-capture node; else straight to pass 1
        ldy PLY
        ldx #1
        lda QSKIND,y
        bne :+
        lda TTFROMA,y
        cmp #NOSQ
        beq :+
        ldx #0
:       txa
        sta PASSNO,y
sloop:  ldy PLY
        lda CURSORLO,y
        cmp PLYENDLO,y
        bne sfetch
        lda CURSORHI,y
        cmp PLYENDHI,y
        bne sfetch
        ; end of list: pass 0 -> 1; 1 -> 2 (qs-capture nodes stop); 2 -> done
        lda PASSNO,y
        cmp #2
        bcc :+
        jmp sdone
:       cmp #1
        bne spass1
        lda QSKIND,y
        beq spass2
        jmp sdone
spass1: lda #1
        sta PASSNO,y
        bne spassgo             ; always
spass2: lda #2
        sta PASSNO,y
spassgo:
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
        ; pass 0: search only the TT move; passes 1/2 skip it
        ldy PLY
        lda PASSNO,y
        bne snotp0
        lda FROM
        cmp TTFROMA,y
        bne sloopj
        lda TO
        cmp TTTOA,y
        bne sloopj
        jmp sdomove
snotp0: ldx TTFROMA,y
        cpx #NOSQ
        beq snotttm
        cpx FROM
        bne snotttm
        ldx TTTOA,y
        cpx TO
        bne snotttm
        jmp sloop               ; the TT move: already searched in pass 0
snotttm:
        ; captures/promotions in pass 1, quiets in pass 2
        ldx TO
        lda BOARD,x
        bne siscap
        lda MVFLAGS
        and #FL_EP|FL_PROMO
        bne siscap
        lda PASSNO,y
        cmp #2
        bne sloopj              ; quiets only in pass 2
        beq sdomove             ; always
siscap: lda PASSNO,y
        cmp #2
        beq sloopj              ; captures were pass 1
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
        lda QSKIND,y            ; TT: lower bound + the cutting move
        bne sbetapop
        jsr setmove3
        lda SCORE
        sta TTENTRY+5
        lda SCORE+1
        sta TTENTRY+6
        lda #TT_LOWER
        jsr ttstore
sbetapop:
        jmp spop
snocut: ; alpha improvement? (strict >)
        sec
        lda ALPHALO,y
        sbc SCORE
        lda ALPHAHI,y
        sbc SCORE+1
        bvc :+
        eor #$80
:       bmi :+                  ; SCORE > ALPHA: improvement
        jmp sloop
:       lda SCORE
        sta ALPHALO,y
        lda SCORE+1
        sta ALPHAHI,y
        ; record this move (cursor was already advanced by 3)
        lda #1
        sta RAISED,y
        jsr setmove3
        ldy PLY
        lda TTENTRY+3
        sta TTBF,y
        lda TTENTRY+4
        sta TTBT,y
        cpy #0
        beq :+
        jmp sloop
:       lda TTENTRY+3           ; root: also for the driver
        sta BESTFROM
        lda TTENTRY+4
        sta BESTTO
        ldy #2
        lda (CURPTR),y          ; setmove3 left CURPTR at the move
        sta BESTFLAGS
        jmp sloop

sdone:  ; return alpha; full-width nodes with no legal moves: mate/stalemate
        ldy PLY
        lda QSKIND,y
        bne sretqs
        lda LEGALCNT,y
        bne sret
        jsr curincheck
        bcs smated
        lda #0                  ; stalemate
        sta SCORE
        sta SCORE+1
        beq sterm               ; always
smated: lda PLY                 ; SCORE = PLY - MATE (mated here)
        sec
        sbc #<MATE
        sta SCORE
        lda #0
        sbc #>MATE
        sta SCORE+1
sterm:  lda #NOSQ               ; TT: exact, no move
        sta TTENTRY+3
        sta TTENTRY+4
        lda SCORE
        sta TTENTRY+5
        lda SCORE+1
        sta TTENTRY+6
        lda #TT_EXACT
        jsr ttstore
        jmp spop
sret:   ldy PLY
        lda ALPHALO,y
        sta SCORE
        lda ALPHAHI,y
        sta SCORE+1
        ; TT: exact if alpha was raised here, else upper bound
        lda TTBF,y
        sta TTENTRY+3
        lda TTBT,y
        sta TTENTRY+4
        lda SCORE
        sta TTENTRY+5
        lda SCORE+1
        sta TTENTRY+6
        lda #TT_UPPER
        ldx RAISED,y
        beq :+
        lda #TT_EXACT
:       jsr ttstore
        jmp spop
sretqs: ldy PLY
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

; setmove3: TTENTRY+3/4 = the from/to of the move at cursor[PLY] - 3
; (the move just searched; the cursor advances before make).
setmove3:
        ldy PLY
        lda CURSORLO,y
        sec
        sbc #3
        sta CURPTR
        lda CURSORHI,y
        sbc #0
        sta CURPTR+1
        ldy #0
        lda (CURPTR),y
        sta TTENTRY+3
        iny
        lda (CURPTR),y
        sta TTENTRY+4
        rts
