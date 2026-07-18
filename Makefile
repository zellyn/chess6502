# chess6502 build. Requires ca65/ld65 (brew install cc65) and Go.

CA65 := ca65
LD65 := ld65

# Warn (once per make run) if the toolchain drifts from the tested version.
TESTED_CC65 := V2.18
CC65_VERSION := $(shell $(CA65) --version 2>&1 | head -1 | awk '{print $$2}')
ifneq ($(CC65_VERSION),$(TESTED_CC65))
$(warning ca65 is $(CC65_VERSION); this repo was last tested with $(TESTED_CC65))
endif

.PHONY: all hello perft test clean

all: hello perft test

hello: hello/hello.bin
	go run ./cmd/a2run -bin hello/hello.bin -org 0x2000

hello/hello.bin: hello/hello.s hello/raw2000.cfg
	$(CA65) hello/hello.s -o hello/hello.o
	$(LD65) -C hello/raw2000.cfg hello/hello.o -o $@

perft: asm/perft.bin

asm/tables.s: cmd/gentables/main.go
	go run ./cmd/gentables

asm/perft.bin: asm/perft.s asm/board.s asm/movegen.s asm/defs.inc asm/tables.s asm/perft.cfg
	cd asm && $(CA65) perft.s -o perft.o
	cd asm && $(LD65) -C perft.cfg perft.o -o perft.bin -Ln perft.lbl

test:
	go build ./...
	go test ./...
	cd ../goapple2 && go test ./iie/
	cd ../go6502 && go test ./...

clean:
	rm -f hello/hello.o hello/hello.bin
