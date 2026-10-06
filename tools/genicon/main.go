// Command genicon gera os PNGs do ícone do executável (o mesmo mini-grid do
// ícone da área de notificação, em azul) em vários tamanhos, desenhados um a
// um para ficarem nítidos. Usado por `go generate` (ver main.go).
//
//	go run ./tools/genicon winres
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"minwinm/internal/icon"
)

// Tamanhos que o Windows usa: lista/barra (16–32), Explorer (48) e
// visualizações grandes (256).
var sizes = []int{16, 20, 24, 32, 48, 64, 256}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "uso: genicon <pasta de saída>")
		os.Exit(2)
	}
	dir := os.Args[1]
	for _, size := range sizes {
		path := filepath.Join(dir, fmt.Sprintf("icon_%d.png", size))
		if err := writePNG(path, size); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func writePNG(path string, size int) error {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for i, p := range icon.Pixels(size, icon.ColorActive) {
		img.SetNRGBA(i%size, i/size, color.NRGBA{
			R: uint8(p >> 16), G: uint8(p >> 8), B: uint8(p), A: uint8(p >> 24),
		})
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
