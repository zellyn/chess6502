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
        sta INCHK,y
        sta FUTILE,y
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
snodej: jmp sprep               ; otherwise ordering only
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
:       bpl snodej              ; score > alpha: not usable
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
:       bmi snodej              ; score < beta: not usable
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
        bcc :+
        ldy PLY                 ; in check: full evasion node
        lda #1
        sta INCHK,y
        jmp snode
:
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
:       bmi :+
        jmp snode               ; no improvement
:       lda SCORE
        sta ALPHALO,y
        lda SCORE+1
        sta ALPHAHI,y
        jmp snode               ; qs nodes skip sprep (no null/futility)

; ---------------------------------------------------------------
; sprep: full-width-node pruning, before move generation.
; ---------------------------------------------------------------
sprep:  ldy PLY
        jsr curincheck
        ldy PLY
        lda #0
        rol                     ; carry (in check) -> A
        sta INCHK,y
        beq :+
        jmp snode               ; in check: no null, no RFP, no futility
:       ; ---- null move: FT_NULL, remaining >= 2, phase >= 3, beta
        ; below the mate zone
        lda FEATURES
        and #FT_NULL
        bne :+
        jmp snonull
:       lda MAXDEPTH
        sec
        sbc PLY
        cmp #2
        bcc snonull
        lda PHASE
        cmp #3
        bcc snonull
        lda BETAHI,y
        cmp #$74
        bcs snonull
        jsr makenull
        ldy PLY                 ; child ply: zero window around -beta
        sec
        lda #0
        sbc BETALO-1,y
        sta ALPHALO,y
        sta BETALO,y
        lda #0
        sbc BETAHI-1,y
        sta ALPHAHI,y
        sta BETAHI,y
        lda BETALO,y
        clc
        adc #1
        sta BETALO,y
        bcc :+
        lda BETAHI,y
        adc #0                  ; carry set: +1
        sta BETAHI,y
:       lda MAXDEPTH            ; reduce by R=2 for the null subtree
        pha
        sec
        sbc #2
        sta MAXDEPTH
        jsr search
        pla
        sta MAXDEPTH
        jsr unmakenull
        sec                     ; SCORE = -SCORE
        lda #0
        sbc SCORE
        sta SCORE
        lda #0
        sbc SCORE+1
        sta SCORE+1
        ldy PLY
        sec
        lda SCORE
        sbc BETALO,y
        lda SCORE+1
        sbc BETAHI,y
        bvc :+
        eor #$80
:       bmi snonull             ; below beta: search normally
        lda BETALO,y            ; null cutoff: fail hard
        sta SCORE
        lda BETAHI,y
        sta SCORE+1
        rts
snonull:
        ; ---- RFP + futility: FT_FUTIL, remaining <= 2
        lda FEATURES
        and #FT_FUTIL
        beq sprepj
        lda MAXDEPTH
        sec
        sbc PLY
        sta REMDEPTH
        cmp #3
        bcs sprepj
        jsr eval
        ldy PLY
        lda #120                ; margin: 120 at depth 1, 250 at depth 2
        ldx REMDEPTH
        cpx #2
        bcc :+
        lda #250
:       sta FUTMARG
        ; reverse futility: eval - margin >= beta -> fail high
        sec
        lda SCORE
        sbc FUTMARG
        sta T0
        lda SCORE+1
        sbc #0
        sta T1
        sec
        lda T0
        sbc BETALO,y
        lda T1
        sbc BETAHI,y
        bvc :+
        eor #$80
:       bmi srfpno
        lda BETALO,y
        sta SCORE
        lda BETAHI,y
        sta SCORE+1
        rts
srfpno: ; futility (depth 1): eval + margin <= alpha -> quiets can't help
        lda REMDEPTH
        cmp #1
        bne sprepj
        clc
        lda SCORE
        adc FUTMARG
        sta T0
        lda SCORE+1
        adc #0
        sta T1
        sec
        lda ALPHALO,y
        sbc T0
        lda ALPHAHI,y
        sbc T1
        bvc :+
        eor #$80
:       bmi sprepj              ; alpha < eval+margin: quiets may matter
        lda #1
        sta FUTILE,y
sprepj: jmp snode

; makenull / unmakenull: pass the move. Only ep, the hash, the halfmove
; clock, and the side flip change; accumulators are untouched.
makenull:
        ldx PLY
        lda EPSQ
        sta UNDOEP,x
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
        lda EPSQ
        cmp #NOSQ
        beq :+
        jsr hashep              ; xor out the ep file
        lda #NOSQ
        sta EPSQ
:       jsr hashstm
        lda SIDE
        eor #COLORMASK
        sta SIDE
        inc HALFMOVE
        inc PLY
        rts
unmakenull:
        dec PLY
        ldx PLY
        lda SIDE
        eor #COLORMASK
        sta SIDE
        lda UNDOEP,x
        sta EPSQ
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
        rts

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
        ; end of list: 0 (TT move) -> 1 (captures) -> 2 (killers) ->
        ; 3 (quiets) -> done; qs and futility-pruned nodes stop after 1
        lda PASSNO,y
        cmp #3
        bcc :+
        jmp sdone
:       cmp #1
        bcc spass1              ; pass 0 done -> captures
        bne spass3              ; pass 2 done -> rest of quiets
        lda QSKIND,y            ; pass 1 done:
        beq :+
        jmp sdone               ; qs: captures only
:       lda FUTILE,y
        beq :+
        jmp sdone               ; futility: quiets can't raise alpha
:       lda FEATURES
        and #FT_KILLER
        bne spass2
        beq spass3              ; killers off: skip their pass
spass1: lda #1
        sta PASSNO,y
        bne spassgo             ; always
spass2: lda #2
        sta PASSNO,y
        bne spassgo             ; always
spass3: lda #3
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
        beq :+
        jmp sloop
:       lda TO
        cmp TTTOA,y
        beq :+
        jmp sloop
:       jmp sdomove
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
        ; captures/promotions in pass 1; quiets in pass 2 (killers) and
        ; pass 3 (the rest)
        ldx TO
        lda BOARD,x
        bne siscap
        lda MVFLAGS
        and #FL_EP|FL_PROMO
        bne siscap
        lda PASSNO,y
        cmp #2
        beq squietk
        cmp #3
        beq squietr
        jmp sloop               ; passes 0/1: no quiets
squietk:                        ; killer pass: only killer matches
        lda KILLER1F,y
        cmp FROM
        bne :+
        lda KILLER1T,y
        cmp TO
        beq sdomovej
:       lda KILLER2F,y
        cmp FROM
        bne :+
        lda KILLER2T,y
        cmp TO
        beq sdomovej
:       jmp sloop
squietr:                        ; final pass: skip killers (already done)
        lda FEATURES
        and #FT_KILLER
        beq sdomovej
        lda KILLER1F,y
        cmp FROM
        bne :+
        lda KILLER1T,y
        cmp TO
        beq skskip
:       lda KILLER2F,y
        cmp FROM
        bne sdomovej
        lda KILLER2T,y
        cmp TO
        bne sdomovej
skskip: jmp sloop
sdomovej:
        jmp sdomove
siscap: lda PASSNO,y
        cmp #2
        bcc :+
        jmp sloop               ; captures were pass 1
:       ; qs nodes: queen promotions only
        lda QSKIND,y
        beq sdomove
        lda MVFLAGS
        and #FL_PROMO
        beq sdomove
        cmp #QUEEN
        bne skskip
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
        ; killers: remember quiet cutoff moves
        lda FEATURES
        and #FT_KILLER
        beq snokupd
        ldx TTENTRY+4
        lda BOARD,x             ; board is restored: nonzero = capture
        bne snokupd
        ldy #2
        lda (CURPTR),y          ; setmove3 left CURPTR at the move
        and #FL_EP|FL_PROMO
        bne snokupd
        ldy PLY
        lda TTENTRY+3
        cmp KILLER1F,y
        bne skupd
        lda TTENTRY+4
        cmp KILLER1T,y
        beq snokupd             ; already killer 1
skupd:  lda KILLER1F,y
        sta KILLER2F,y
        lda KILLER1T,y
        sta KILLER2T,y
        lda TTENTRY+3
        sta KILLER1F,y
        lda TTENTRY+4
        sta KILLER1T,y
snokupd:
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
        lda FUTILE,y
        bne sretqs              ; quiets were pruned: can't claim mate
        lda INCHK,y             ; computed at node entry
        bne smated
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
