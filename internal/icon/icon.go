// Package icon desenha o ícone do minWinM — um grid com uma janela grande à
// esquerda e duas empilhadas à direita — como pixels, sem depender do
// Windows. O pacote win transforma os pixels num HICON.
package icon

// Cores no formato 0xAARRGGBB.
const (
	ColorActive   = 0xFF3B82F6 // azul: tiling ligado
	ColorInactive = 0xFF9CA3AF // cinza: tiling desligado
)

// Pixels devolve size×size pixels, linha a linha de cima para baixo, no
// formato 0xAARRGGBB (na memória: B, G, R, A — o mesmo de um DIB de 32 bits).
// O fundo é transparente.
func Pixels(size int, color uint32) []uint32 {
	px := make([]uint32, size*size)
	if size < 8 {
		return px
	}
	pad := max(1, size/8)
	gap := max(1, size/16)
	end := size - pad
	midX := pad + (end-pad)/2
	midY := pad + (end-pad)/2

	// Retângulos [x0, x1) × [y0, y1).
	rects := [][4]int{
		{pad, midX - (gap+1)/2, pad, end},                          // esquerda, altura toda
		{midX - (gap+1)/2 + gap, end, pad, midY - (gap+1)/2},       // direita, em cima
		{midX - (gap+1)/2 + gap, end, midY - (gap+1)/2 + gap, end}, // direita, embaixo
	}
	round := size >= 20 // em tamanhos maiores, cantos arredondados (1 px)
	for _, r := range rects {
		x0, x1, y0, y1 := r[0], r[1], r[2], r[3]
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				corner := (x == x0 || x == x1-1) && (y == y0 || y == y1-1)
				if round && corner {
					continue
				}
				px[y*size+x] = color
			}
		}
	}
	return px
}
