// Command dhgr2png decodes an Apple II double-hires screen save (A2FC
// layout: 8K aux bank then 8K main bank, as DazzleDraw writes) into a
// PNG for browsing. Usage: dhgr2png in.bin out.png
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: dhgr2png in.bin out.png")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(data) != 16384 {
		fmt.Fprintf(os.Stderr, "expected 16384 bytes (aux+main), got %d\n", len(data))
		os.Exit(1)
	}
	aux, mainb := data[:8192], data[8192:]
	// Double height for roughly square pixels on modern displays.
	img := image.NewGray(image.Rect(0, 0, 560, 384))
	on := color.Gray{0xEE}
	for y := range 192 {
		off := (y&7)*1024 + ((y>>3)&7)*128 + (y>>6)*40
		x := 0
		for col := range 40 {
			for _, b := range []byte{aux[off+col], mainb[off+col]} {
				for bit := range 7 {
					if b>>bit&1 != 0 {
						img.SetGray(x, 2*y, on)
						img.SetGray(x, 2*y+1, on)
					}
					x++
				}
			}
		}
	}
	f, err := os.Create(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
