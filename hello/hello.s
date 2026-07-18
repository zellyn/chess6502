; Toolchain smoke test: compute 3+4, store at $0300, write "OK\n" to the
; harness character-output trap, then hit the success trap.
;
; Harness conventions (see docs/testing.md):
;   $BFF0  COUT trap  - a store here emits the byte to stdout
;   $BFFF  exit trap  - a store here exits the harness; value = exit code

COUT_TRAP = $BFF0
EXIT_TRAP = $BFFF

        .org $2000

start:  lda #3
        clc
        adc #4
        sta $0300       ; result visible to harness memory dump

        ldx #0
msg:    lda text,x
        beq done
        sta COUT_TRAP
        inx
        bne msg

done:   lda #0          ; exit code 0 = success
        sta EXIT_TRAP
        brk             ; unreachable

text:   .byte "OK", $0A, 0
