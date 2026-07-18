# tools/ — chess-engine gauntlet toolchain

Everything in this directory except `README.md` and `.gitignore` is
**untracked** (`tools/.gitignore` is `*` plus explicit `!` exceptions for
those two files). Nothing here is committed to git; this file documents
exactly how to reproduce the contents from scratch on macOS (Apple
Silicon, Homebrew at `/opt/homebrew`).

Built/verified on: macOS Tahoe 26.5.2, arm64, Apple clang 21.0.0
(clang-2100.1.1.101), cmake 4.4.0, Qt 6.11.1 (Homebrew `qtbase`+`qtsvg`).
Date: 2026-07-17.

## Layout

```
tools/
  bin/                 actual built binaries
  build/                source checkouts / working dirs (gitignored, keep or wipe freely)
  share/fairymax/       fairymax's fmax.ini / *.hash data files
  pgn/                  PGN output from sanity matches
  cutechess-cli, tscp, umax, fairymax, neg, minnow   -> symlinks into bin/
  README.md, .gitignore
```

To rebuild everything from scratch: `rm -rf tools/build tools/bin tools/share tools/pgn tools/cutechess-cli tools/tscp tools/umax tools/fairymax tools/neg tools/minnow` and replay the commands below.

## 0. Prerequisites (Homebrew)

```
brew install cmake qtbase qtsvg
```

`qtbase` (6.11.1) pulls in Core/Gui/Widgets/Concurrent/PrintSupport/Network/etc
(14 lightweight deps: brotli, dbus, double-conversion, freetype, glib,
harfbuzz, icu4c, jpeg-turbo, libb2, libpng, md4c, openssl@3, pcre2, zstd).
`qtsvg` adds just the SVG module. This is **much** lighter than `brew install
qt` (the full meta-formula, which additionally pulls in QtWebEngine/Chromium
and dozens of other modules — not needed just to build `cutechess-cli`).

No cask/formula named `cutechess` exists in Homebrew — it must be built from
source (see below).

## 1. cutechess-cli 1.5.1

Homebrew has no `cutechess` formula or cask. Built from source:

```
cd tools/build
git clone --depth 1 https://github.com/cutechess/cutechess.git
cd cutechess
# commit: 5e84232be4546aaedc9d87a96c91867a1da06ada (2026-07-14)

cmake -S . -B build \
  -DCMAKE_PREFIX_PATH="/opt/homebrew/opt/qtbase" \
  -DQT_ADDITIONAL_PACKAGES_PREFIX_PATH="/opt/homebrew/opt/qtsvg" \
  -DWITH_TESTS=OFF -DCMAKE_BUILD_TYPE=Release

cmake --build build --target cli -j"$(sysctl -n hw.ncpu)"
```

Notes:
- Only the `cli` target is built (not the GUI `cutechess` app or unit tests),
  which is why `-DWITH_TESTS=OFF` and `--target cli` are used — this avoids
  needing `Qt::Test` and keeps the build fast.
- Top-level `CMakeLists.txt` does one `find_package(Qt6 REQUIRED COMPONENTS
  Core Gui Widgets Concurrent Svg PrintSupport ...)` for the whole project
  (GUI + CLI together), so Svg is required even though the CLI binary itself
  only links `Qt::Core`.
- Homebrew installs each Qt6 module (`qtbase`, `qtsvg`, ...) under its own
  `/opt/homebrew/opt/<module>` prefix rather than one unified prefix. Qt6's
  own `Qt6Config.cmake` component-search logic does **not** consult
  `CMAKE_PREFIX_PATH` again when it internally calls `find_package(Qt6Svg
  ...)` — it only searches paths derived from where `Qt6Config.cmake` itself
  lives, plus the CMake cache variable `QT_ADDITIONAL_PACKAGES_PREFIX_PATH`.
  That variable is the documented mechanism for exactly this
  split-installation scenario, hence passing
  `-DQT_ADDITIONAL_PACKAGES_PREFIX_PATH="/opt/homebrew/opt/qtsvg"` above.
  (Plain `-DCMAKE_PREFIX_PATH="qtbase;qtsvg"` alone does **not** work and
  fails with `Failed to find required Qt component "Svg"`.)

Binary produced at `tools/build/cutechess/build/cutechess-cli`, copied to
`tools/bin/cutechess-cli`, symlinked at `tools/cutechess-cli`.

```
cp tools/build/cutechess/build/cutechess-cli tools/bin/cutechess-cli
ln -sf bin/cutechess-cli tools/cutechess-cli
```

Verify:

```
$ tools/cutechess-cli --version
cutechess-cli 1.5.1
Using Qt version 6.11.1
Running on macOS Tahoe (26.5.2)/arm64
```

The binary only links `QtCore` dynamically (`otool -L`), so `qtbase` must
stay installed via Homebrew for it to run (it is not statically linked /
bundled).

## 2. TSCP 1.81 (Tom Kerrigan's Simple Chess Program, CCRL ~1700, xboard protocol v1)

Source URL (still live as of 2026-07-17, HTTP 200):
`http://www.tckerrigan.com/Chess/TSCP/tscp181.zip`

```
cd tools/build
curl -sL -o tscp181.zip http://www.tckerrigan.com/Chess/TSCP/tscp181.zip
unzip -o tscp181.zip -d .
cd tscp181
clang -O2 -o tscp board.c book.c data.c eval.c main.c search.c
```

Builds clean, no warnings, no Makefile needed (the zip only ships a Windows
`.exe` plus source — build manually as above).

```
cp tools/build/tscp181/tscp tools/bin/tscp
ln -sf bin/tscp tools/tscp
```

**Important protocol note**: TSCP implements old-style xboard protocol
version 1 — it recognizes `xboard`, `new`, `force`, `white`, `black`, `st`,
`sd`, `time`, `otim`, `go`, `hint`, `undo`, `remove`, `post`, `nopost`, but
**does not** respond to `protover`/send `feature` lines (it just prints
`Error (unknown command): protover` and falls through — this is harmless).
`cutechess-cli`'s xboard adapter handles this correctly by timing out on the
`feature done=1` handshake and falling back to protocol-version-1 behavior.
Confirmed working end-to-end (see sanity match below).

Verify handshake manually:

```
$ printf 'xboard\nnew\nst 1\ngo\nquit\n' | tools/tscp
...
move g1f3
```

## 3. micro-Max 4.8 (H.G. Muller, CCRL ~1890 nominally, but see caveat below)

Source URL (live): `http://home.hccnet.nl/h.g.muller/umax4_8.c`

```
cd tools/build
curl -sL -o umax4_8.c http://home.hccnet.nl/h.g.muller/umax4_8.c
clang -O2 -std=gnu89 -w umax4_8.c -o umax
```

`-std=gnu89` is required: the source uses 1990s-vintage K&R-style implicit-int
function declarations (e.g. `TC() { ... }`, `D(k,q,l,e,E,z,n) int k,q,...
{ ... }` with no return type), which modern clang treats as a hard error
(`-Wimplicit-int` is an error by default under C17/C23). `-w` silences the
~26 (harmless, expected — this is famously terse/obfuscated code by design)
operator-precedence warnings.

```
cp tools/build/umax tools/bin/umax
ln -sf bin/umax tools/umax
```

**Caveat — raw micro-Max 4.8 does NOT speak xboard protocol.** This exact
4.8 single-file version has its own minimal ASCII I/O loop: it reprints the
whole board after every move and expects moves typed as raw `e2e4`-style
tokens with no `xboard`/`protover`/`feature` handshake and no `move e2e4`
output line — cutechess-cli's `proto=xboard` adapter cannot drive it
directly (confirmed: it doesn't reply to `xboard`/`protover` and doesn't
emit a parseable `move ...` line). It is built and kept here for reference
and for anyone who wants to write a custom protocol shim, but **Fairy-Max
(below) is what actually gets used with cutechess-cli** for gauntlet play,
per the task's own fallback note ("At least one of umax/fairymax must
work").

## 4. Fairy-Max 5.0b (H.G. Muller, CCRL ~1890, full xboard protocol v2)

The canonical `fmax4_8u.zip` URL from the old hccnet mirror
(`http://home.hccnet.nl/h.g.muller/fmax4_8u.zip`) is dead (404). Used the
actively-maintained GitHub mirror instead:

```
cd tools/build
git clone --depth 1 https://github.com/RMKirkpatrick/fairymax.git
cd fairymax
# commit: e5d8db39234e6cd20834acf68a589092e868bc89 (2025-01-25)
```

This package also contains ShaMax and MaxQi (Shatranj/Xiangqi variants) —
only `fairymax.c` (standard chess + variants) is built.

```
clang -O2 -std=gnu89 -w \
  -DFAIRYDIR="\"/Users/zellyn/gh/chess6502/tools/share/fairymax\"" \
  fairymax.c -o fairymax
```

Notes:
- Same `-std=gnu89` requirement as micro-Max (same K&R-style author idiom).
- `FAIRYDIR` (not `INI_FILE`!) is the macro to override: the source has
  `#define INI_FILE FAIRYDIR "/fmax.ini"` with **no** `#ifndef` guard around
  `INI_FILE` itself (only `FAIRYDIR` is guarded), so passing `-DINI_FILE=...`
  on the command line gets silently overridden back by the source's own
  `#define`. Passing `-DFAIRYDIR=...` works because *that* macro genuinely is
  `#ifndef`-guarded.
- Baking in an absolute `FAIRYDIR` means `fairymax` finds its `fmax.ini` (the
  piece/variant definition file — without it, Fairy-Max refuses to run real
  games and just prints `piece-description file './fmax.ini' not found`)
  regardless of what directory cutechess-cli launches it from.

Data files (needed at the `FAIRYDIR` path baked into the binary):

```
mkdir -p tools/share/fairymax
cp tools/build/fairymax/data/fmax.ini tools/build/fairymax/data/qmax.ini \
   tools/build/fairymax/data/*.hash tools/share/fairymax/
```

```
cp tools/build/fairymax/fairymax tools/bin/fairymax
ln -sf bin/fairymax tools/fairymax
```

Verify handshake:

```
$ printf 'xboard\nprotover 2\nquit\n' | tools/fairymax
tellics say     Fairy-Max 5.0b
tellics say     by H.G. Muller
feature myname="Fairy-Max 5.0b"
feature memory=1 exclude=1
feature setboard=0 xedit=1 ping=1 done=0
feature variants="normal,nocastle,shatranj,asean,makruk,...,fairy"
feature option="Resign -check 0"
...
feature done=1
```

Full `feature ... done=1` negotiation confirms proper xboard protocol
version 2 support — this is the "XBoard-complete descendant" the task asked
for, and it's the one used in the sanity match below.

## 5. Two weaker engines (CCRL below TSCP/Fairy-Max)

Research done via web search of CCRL-adjacent sources and TalkChess/GitHub
(see below); the two picked both come from authors already represented here
(H.G. Muller) or are small enough to vet by reading the full source.
`Usurpator II` (no obtainable modern source — only a wrapped 1980s 6502
binary exists) and `POS` (Java, no fit for this native-clang toolchain) were
investigated and are dead ends, not pursued further.

### 5a. N.E.G. 1.1 (H.G. Muller — "Non-Evaluating/Extremely-weak" test engine, no tree search, xboard protocol v2)

No stable single-file download URL exists for N.E.G.; the source was posted
inline (as a `<pre>` code block) by H.G. Muller in a TalkChess forum thread:
`https://www.talkchess.com/forum3/viewtopic.php?t=54757`. Because forum
posts can be edited/deleted (link rot risk), the exact v1.1 source (the
later post in that thread, which fixes a pawn-promotion bug present in the
first v1.0 post — it adds the `promo` suffix so `move ...q` is emitted
correctly) is reproduced verbatim below and also saved at
`tools/build/neg.c`.

N.E.G. is a **non-searching** engine (no minimax/alpha-beta at all — it
picks the best *one-ply* heuristic move directly). H.G. Muller's own
benchmark in that thread: "Fairy-Max with a depth limit of 1 ply scores
about 72% against N.E.G." — i.e. even a 1-ply-capped Fairy-Max beats it
roughly 3:1. No official CCRL rating was found; treat it as a very weak,
sub-1200-ish bottom-of-the-pool engine and re-calibrate empirically once
it's in a real gauntlet.

<details>
<summary>tools/build/neg.c (N.E.G. 1.1 full source, from TalkChess t=54757)</summary>

```c
#include <stdio.h>

#ifdef WIN32 
#    include <windows.h>
#else
#    include <sys/time.h>
#    include <sys/times.h>
#    include <unistd.h>
     int GetTickCount()
     {	struct timeval t;
	gettimeofday(&t, NULL);
	return t.tv_sec*1000 + t.tv_usec/1000;
     }
#endif

#define WHITE 8
#define BLACK 16
#define COLOR (WHITE|BLACK)


typedef void Func(int stm, int from, int to, void *closure);
typedef int Board[128];

int value[8] = { 0, 100, 100, 10000, 325, 350, 500, 950 };
int firstDir[] = { 0, 0, 27, 4, 18, 8, 13, 4 };
int steps[] = { -16, -15, -17, 0, 1, -1, 16, -16, 15, -15, 17, -17, 0, 1, -1, 16, -16, 0, 18, 31, 33, 14, -18, -31, -33, -14, 0, 16, 15, 17, 0 };
Board PST = {
 0, 2, 4, 6, 6, 4, 2, 0,   0,0,0,0,0,0,0,0,
 2, 8,10,12,12,10, 8, 2,   0,0,0,0,0,0,0,0,
 6,12,16,18,18,16,12, 6,   0,0,0,0,0,0,0,0,
 8,14,18,20,20,18,14, 8,   0,0,0,0,0,0,0,0,
 8,14,18,20,20,18,14, 8,   0,0,0,0,0,0,0,0,
 6,12,16,18,18,16,12, 6,   0,0,0,0,0,0,0,0,
 2, 8,10,12,12,10, 8, 2,   0,0,0,0,0,0,0,0,
 0, 2, 4, 6, 6, 4, 2, 0,   0,0,0,0,0,0,0,0
};


//              "abcdefghijklmnopqrstuvwxyz"
char pieces[] = ".5........3..4.176........";
Board board, attacks[2], lva[2];
int bestScore, bestFrom, bestTo, lastMover, lastChecker, randomize, post;

void
MoveGen (Board board, int stm, Func proc, void *cl)
{
  int from, to, piece, victim, type, dir, step;
  for(from=0; from<128; from = from + 9 & ~8) {
    piece = board[from];
    if(piece & stm) {
      type = piece & 7;
      dir = firstDir[type];
      while((step = steps[dir++])) {
        to = from;
        do {
          to += step;
          if(to & 0x88) break;
          victim = board[to];
          (*proc)(piece & COLOR, from, to, cl);
          victim += type < 5;
          if(!(to - from & 7) && type < 3 && (type == 1 ? to > 79 : to < 48)) victim--;
        } while(!victim);
      }
    }
  }
}

int InCheck (int stm)
{
  int k, xstm = COLOR - stm;
  for(k=0; k<128; k++) if( board[k] == stm + 3 ) {
    int dir=4, step;
    int f = (stm == WHITE ? -16 : 16); // forward
    int p = (stm == WHITE ? 2 : 1);
    if(!(k + f + 1 & 0x88) && board[k + f + 1] == xstm + p) return k + f + 2;
    if(!(k + f - 1 & 0x88) && board[k + f - 1] == xstm + p) return k + f;
    while((step = steps[dir])) {
	int from = k + steps[dir + 14];
	if(!(from & 0x88) && board[from] == xstm + 4) return from + 1;
	from = k + step;
	if(!(from & 0x88) && board[from] == xstm + 3) return from + 1;
	from = k;
	while(!((from += step) & 0x88)) if(board[from]) { // occupied
	    if(dir < 8 && (board[from] & COLOR + 6) == xstm + 6) return from + 1; // R or Q and orthogonal
	    if(dir > 7 && (board[from] & COLOR + 5) == xstm + 5) return from + 1; // B or Q and diagonal
	    break;
	}
	dir++;
    }
    break;
  }
  return 0;
}

void
Count (int stm, int from, int to, void *cl)
{
  int s = (stm == BLACK);
  if(!(to - from & 7) && (board[from] & 7) < 3) return; // ignore Pawn non-captures
  attacks[s][to]++;
  if(lva[s][to] > value[board[from] & 7]) lva[s][to] = value[board[from] & 7];
}

void
Score (int stm, int from, int to, void *cl)
{
  int score = PST[to] - PST[from];
  int piece = board[from];
  int victim = board[to];
  int myVal = value[piece & 7];
  int hisVal = value[victim & 7];
  int push = (piece & 7) < 3 && to - from & 7;// Pawn non-capture
  int s = (stm == BLACK);
  int check;
  if((piece & 7) < 3 && !(to - from & 7) != !victim) return; // weed out illegal pawn modes
  if((piece & 7) == 3) score -= score;        // keep King out of center
  else if(myVal > 400) score = 0;             // no centralization for R, Q
  if((piece & ~7) == (victim & ~7)) return;   // self capture
  board[from] = 0; board[to] = piece;
  if(from != lastChecker && InCheck(COLOR - stm)) score += 50; // bonus for checking with new piece
  check = InCheck(stm);                       // in check after move?
  board[to] = victim; board[from] = piece;
  if(check) return;                           // illegal
  score += ((rand()>>8 & 31) - 16)*randomize; // randomize
  if(from == lastMover) score -= 10;          // discourage moving same piece twice
  if(hisVal && hisVal < 400) score += PST[to];// centralization bonus of victim
  score += hisVal;                            // captured piece
  if(attacks[!s][to]) {                       // to-square was attacked
    if(attacks[s][to] - 1 + push < attacks[!s][to] ) score -= myVal; else // not sufficiently protected
    if(myVal > lva[!s][to]) score += lva[!s][to] - myVal; // or protected, but more valuable
  }
  if((piece & 7) != 3 && attacks[!s][from]) { // from-square was attacked (and not King)
    if(attacks[s][from] < attacks[!s][from] ) score += myVal; else // not sufficiently protected
    if(myVal > lva[!s][from]) score -= lva[!s][from] - myVal; // or protected, but more valuable
  }
  if((piece & 7) == 1 && to < 48) score += 50;
  if((piece & 7) == 2 && to > 79) score += 50;

  if(score > bestScore) bestScore = score, bestFrom = from, bestTo = to; // remember best move
  if(post) printf("2 %d 0 1 %c%d%c%d\n", score, (from&7) + 'a', 8 - (from >> 4), (to&7) + 'a', 8 - (to >> 4));
}

int
Setup (char *fen)
{
  char c;
  int i;
  for(i=0; i<128; i++) board[i] = 0;
  i = 0;
  while((c = *fen++)) {
    if(c == 'p') board[i++] = BLACK + 2; else
    if(c >= '0' && c <= '9') i += c - '0'; else
    if(c >= 'a' && c <= 'z') board[i++] = BLACK + pieces[c - 'a'] - '0'; else
    if(c >= 'A' && c <= 'Z') board[i++] = WHITE + pieces[c - 'A'] - '0'; else
    if(c == '/') i = (i | 15) + 1; else break;
  }
  for(i=0; i<128; i = i + 9 & ~8) printf(i&7 ? " %2d" : "\n# %2d", board[i]); printf("\n");
  return (*fen == 'w' ? WHITE : BLACK);
}

int
main ()
{
  int stm = WHITE, engineSide = 0;
  char line[256], command[20];
  srand(GetTickCount());
  while(1) {
    int i, c;
    if(stm == engineSide) {
	char *promo = "";
	for(i=0; i<128; i++) lva[0][i] = lva[1][i] = 30000, attacks[0][i] = attacks[1][i] = 0;
	MoveGen(board, COLOR, &Count, NULL);
	bestScore = -30000;
	MoveGen(board, stm, &Score, NULL);
	board[bestTo] = board[bestFrom]; board[bestFrom] = 0; stm ^= COLOR;
	if((board[bestTo] & 7) < 3 && (stm == BLACK ? bestTo < 16 : bestTo > 111)) board[bestTo] |= 7, promo = "q"; // always promote to Q
	lastMover = bestTo;
	lastChecker = InCheck(stm) ? bestTo : -1 ;
	printf("move %c%d%c%d%s\n", (bestFrom&7) + 'a', 8 - (bestFrom >> 4), (bestTo&7) + 'a', 8 - (bestTo >> 4), promo);
    }
    fflush(stdout); i = 0;
    while((line[i++] = c = getchar()) != '\n') if(c == EOF) printf("# EOF\n"), exit(1); line[i] = '\0';
    if(*line == '\n') continue;
    sscanf(line, "%s", command);
    printf("# command: %s\n", command);
    if(!strcmp(command, "usermove")) {
	int from, to; char c, d, promo, ep;
	sscanf(line, "usermove %c%d%c%d%c", &c, &from, &d, &to, &promo);
	from = (8 - from)*16 + c - 'a'; to = (8 - to)*16 + d - 'a';
	if((board[from] & 7) == 3 && to - from == 2) board[from + 1] = board[to + 1], board[to + 1] = 0; // K-side castling
	if((board[from] & 7) == 3 && from - to == 2) board[from - 1] = board[to - 2], board[to - 2] = 0; // Q-side castling
	ep = ((board[from] & 7) < 3 && !board[to]); // recognize e.p. capture
	board[to] = board[from]; if(ep) board[from & 0x70 | to & 7] = 0; board[from] = 0;
	if(promo == 'q') board[to] = board[to] | 7; // promote
	stm ^= COLOR;	
    }
    else if(!strcmp(command, "protover")) printf("feature myname=\"N.E.G. 1.1\" setboard=1 usermove=1 analyze=0 colors=0 sigint=0 sigterm=0 done=1\n");
    else if(!strcmp(command, "new"))      stm = Setup("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"), randomize = 0, engineSide = BLACK;
    else if(!strcmp(command, "go"))       engineSide = stm;
    else if(!strcmp(command, "result"))   engineSide = 0;
    else if(!strcmp(command, "force"))    engineSide = 0;
    else if(!strcmp(command, "setboard")) stm = Setup(line+9);
    else if(!strcmp(command, "random"))   randomize = !randomize;
    else if(!strcmp(command, "post"))     post = 1;
    else if(!strcmp(command, "nopost"))   post = 0;
    else if(!strcmp(command, "quit"))     break;
  }
  return 0;
}
```

</details>

Build:

```
cd tools/build
# save the block above as neg.c (already saved at tools/build/neg.c)
clang -O2 -std=gnu89 -w neg.c -o neg
```

Same `-std=gnu89` requirement (`main ()`, `MoveGen (...)` etc. use K&R-style
declarations). It prints `# ...` debug lines mixed into stdout (e.g. `#
command: xboard`, a board dump) — these are harmless to cutechess-cli, which
only looks for the lines it recognizes (`feature ...`, `move ...`) and
ignores the rest.

```
cp tools/build/neg tools/bin/neg
ln -sf bin/neg tools/neg
```

Verify: confirmed working end-to-end against TSCP under `cutechess-cli
proto=xboard` (2/2 losses for NEG, as expected for a non-searching engine).

### 5b. minnow (tm512, UCI protocol, small C99 engine, unrated — added for UCI-protocol coverage)

```
cd tools/build
git clone --depth 1 https://github.com/tm512/minnow.git
cd minnow
# commit: 0f9530b5cee610dfc41fc0ab574d3c5aef734329 (2025-05-21)
```

**Local patch required** (`src/main.c`): upstream calls `setbuf(stdin,
NULL); setbuf(stdout, NULL);` only inside `uci_main()` (`src/uci.c`), which
runs *after* `main()`'s own top-level command loop has already done an
`fgets()` on stdin with default (buffered) stdio. Changing a stream's
buffering mode after I/O has already occurred on it is undefined behavior
per C11 §7.21.5.6p2. In practice on macOS this is harmless when stdin is a
regular file (queued input is still delivered) but **hangs forever** when
stdin is a pipe — exactly cutechess-cli's engine-communication mechanism.
Fix: move the two `setbuf` calls to the very top of `main()`, before the
first `printf`/`fgets`:

```diff
 int main (void)
 {
+	/* Local patch (chess6502/tools): unbuffer stdio here, before any I/O
+	   happens. Upstream called setbuf(stdin, NULL) only inside uci_main(),
+	   i.e. after main()'s own fgets() had already buffered from stdin.
+	   Resetting a stream's buffering after I/O has occurred is undefined
+	   behavior (C11 7.21.5.6p2) and reliably hangs when stdin is a pipe
+	   (e.g. under cutechess-cli), even though it happens to work when
+	   stdin is a regular file. */
+	setbuf (stdin, NULL);
+	setbuf (stdout, NULL);
+
 	printf ("minnow " GIT_VERSION "\n");
 	printf (BUILDTYPE " build, built with " CCIDENT "\n");
 	printf ("[c] 2014-2024 Kyle Davis (tm512)\n\n");
```

Build:

```
make CC=clang
```

(Ordinary `gcc`/`cc`/`clang` all resolve to Apple clang on this machine;
`CC=clang` just makes that explicit. Produces a handful of harmless
`-Wformat`/`-Wswitch` warnings, no errors.)

```
cp tools/build/minnow/minnow tools/bin/minnow
ln -sf bin/minnow tools/minnow
```

Verify: confirmed working end-to-end against TSCP under `cutechess-cli
proto=uci` (see sanity matches below) — one draw by repetition, one loss for
minnow, consistent with it being clearly weaker than TSCP.

No published CCRL rating was found for minnow; treat its strength as
empirically unknown/likely weak until it's run through a real gauntlet with
a fixed time control against known-rated opponents.

### Dead ends (investigated, not pursued)

- **Usurpator II**: no obtainable modern/portable source — only exists as an
  original 1980s 6502 binary wrapped by third-party emulator hacks; nothing
  buildable with clang.
- **POS** (vanheusden.com): written in Java, not C — doesn't fit this
  native-clang toolchain, and no CCRL rating was confirmed either.
- CCRL's own rating-list sites (computerchess.org.uk, ccrl.chessdom.com)
  return HTTP 403 to automated fetches, so exact numeric CCRL Elo for NEG /
  minnow could not be directly confirmed — strength claims above are from
  engine-author statements / relative sanity-match results instead.

## 6. Sanity matches (cutechess-cli end-to-end verification)

All engines were launched from `tools/` as the working directory. PGNs saved
under `tools/pgn/`.

### TSCP vs Fairy-Max (the task's required check — 2 games, fast TC)

```
cd tools
./cutechess-cli \
  -engine name=TSCP cmd=./tscp proto=xboard \
  -engine name=FairyMax cmd=./fairymax proto=xboard \
  -each tc=40/10 \
  -rounds 1 -games 2 \
  -pgnout pgn/sanity_tscp_vs_fairymax.pgn
```

Result (both games completed cleanly, decisive):

```
Started game 1 of 2 (TSCP vs FairyMax)
Finished game 1 (TSCP vs FairyMax): 1-0 {White mates}
Score of TSCP vs FairyMax: 1 - 0 - 0  [1.000] 1
Started game 2 of 2 (FairyMax vs TSCP)
Finished game 2 (FairyMax vs TSCP): 1-0 {White mates}
Score of TSCP vs FairyMax: 1 - 1 - 0  [0.500] 2
```

### TSCP vs N.E.G. (xboard vs xboard)

```
./cutechess-cli \
  -engine name=TSCP cmd=./tscp proto=xboard \
  -engine name=NEG cmd=./neg proto=xboard \
  -each tc=40/10 -rounds 1 -games 2 \
  -pgnout pgn/sanity_tscp_vs_neg.pgn
```

Result: TSCP 2 - 0 - 0 vs NEG (both games decisive wins for TSCP, consistent
with NEG being a non-searching, much weaker engine).

### TSCP vs minnow (xboard vs UCI — cross-protocol check)

```
./cutechess-cli \
  -engine name=TSCP cmd=./tscp proto=xboard \
  -engine name=minnow cmd=./minnow proto=uci \
  -each tc=40/10 -rounds 1 -games 2 \
  -pgnout pgn/sanity_tscp_vs_minnow.pgn
```

Result: TSCP 1 - 0 - 1 vs minnow (one draw by 3-fold repetition, one TSCP
win). No hangs, no protocol errors — confirms the minnow stdio patch (§5b)
actually fixes real pipe-based play, not just the isolated repro.

## Summary of what's usable in a gauntlet today

| Engine    | Protocol | Approx. CCRL | Status |
|-----------|----------|--------------|--------|
| TSCP 1.81 | xboard (v1, no feature negotiation) | ~1700 | Working |
| Fairy-Max 5.0b | xboard (v2, full feature negotiation) | ~1890 | Working |
| micro-Max 4.8 (raw) | none (custom ASCII I/O, not xboard) | ~1890 nominal | Builds, but **not usable via cutechess-cli** — use Fairy-Max instead |
| N.E.G. 1.1 | xboard (v2) | unrated, very weak (non-searching) | Working |
| minnow | UCI | unrated | Working (after local patch, see §5b) |
| cutechess-cli | — | — | 1.5.1, built from source |

Nothing was committed to git. All engine binaries, Qt runtime library
dependency (Homebrew `qtbase`), and downloaded/cloned source trees live
under `tools/build/`, `tools/bin/`, and Homebrew's Cellar — none of it is
tracked by this repo except this README and `tools/.gitignore`.
