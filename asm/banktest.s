; Banked-build proof (M1): a multi-segment image whose loader installs a
; code segment into Language Card RAM (write-enable dance) and a data
; segment into aux memory (RAMWRT-only copy, per D4), then calls the LC
; routine, which reads the aux data back via RAMRD (legal only because it
; executes from LC RAM, outside $0200-$BFFF).
;
; Prints "BANKS OK" and exits 0 on success; exits nonzero on mismatch.

COUT_TRAP = $BFF0
EXIT_TRAP = $BFFF

SRC  = $06              ; copy source pointer
DST  = $08              ; copy dest pointer

        .import __LCCODE_LOAD__, __LCCODE_RUN__, __LCCODE_SIZE__
        .import __AUXDATA_LOAD__, __AUXDATA_RUN__, __AUXDATA_SIZE__

        .segment "CODE"

entry:
        ldx #$FF
        txs

        ; -- install LC segment: bank 1 RAM, write-enabled via double read
        lda $C08B
        lda $C08B
        lda #<__LCCODE_LOAD__
        sta SRC
        lda #>__LCCODE_LOAD__
        sta SRC+1
        lda #<__LCCODE_RUN__
        sta DST
        lda #>__LCCODE_RUN__
        sta DST+1
        ldx #>(__LCCODE_SIZE__ + 255)
        jsr copypages

        ; -- install aux segment: writes land in aux while RAMWRT is on;
        ; reads (fetches + source) still come from main, so this is safe
        ; to run from ordinary code.
        sta $C005            ; RAMWRT on
        lda #<__AUXDATA_LOAD__
        sta SRC
        lda #>__AUXDATA_LOAD__
        sta SRC+1
        lda #<__AUXDATA_RUN__
        sta DST
        lda #>__AUXDATA_RUN__
        sta DST+1
        ldx #>(__AUXDATA_SIZE__ + 255)
        jsr copypages
        sta $C004            ; RAMWRT off

        ; -- main-bank shadow at the aux address must be untouched ($00)
        lda __AUXDATA_RUN__
        bne fail
        ; -- call into LC RAM; returns the aux marker byte in A
        jsr __LCCODE_RUN__
        cmp #$A5
        bne fail

        ldx #0
msg:    lda text,x
        beq done
        sta COUT_TRAP
        inx
        bne msg
done:   lda #0
        sta EXIT_TRAP
fail:   lda #1
        sta EXIT_TRAP
        brk

text:   .byte "BANKS OK", $0A, 0

; copypages: copy X 256-byte pages from (SRC) to (DST). Sloppy (copies
; whole pages), fine for a loader.
copypages:
        ldy #0
cploop: lda (SRC),y
        sta (DST),y
        iny
        bne cploop
        inc SRC+1
        inc DST+1
        dex
        bne cploop
        rts

        .segment "LCCODE"

; Runs at $D000 (LC RAM). Reads the aux marker byte: RAMRD switches all
; reads in $0200-$BFFF to aux, including instruction fetches -- legal
; here only because this code executes above $BFFF.
lcprobe:
        sta $C003            ; RAMRD on
        lda __AUXDATA_RUN__  ; aux byte
        sta $C002            ; RAMRD off
        rts

        .segment "AUXDATA"

auxmark:
        .byte $A5
        .res  255, $5A
