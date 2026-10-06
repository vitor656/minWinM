package icon

import "testing"

func TestPixels(t *testing.T) {
	for _, size := range []int{16, 20, 24, 32} {
		px := Pixels(size, ColorActive)
		at := func(x, y int) uint32 { return px[y*size+x] }
		pad := max(1, size/8)

		if len(px) != size*size {
			t.Fatalf("%d: %d pixels", size, len(px))
		}
		// Borda externa transparente.
		for i := 0; i < size; i++ {
			if at(i, 0) != 0 || at(0, i) != 0 || at(i, size-1) != 0 || at(size-1, i) != 0 {
				t.Fatalf("%d: borda não transparente", size)
			}
		}
		// Janela da esquerda pintada na altura do meio; as da direita em
		// cima e embaixo; e uma faixa vazia separando esquerda e direita.
		mid := size / 2
		if at(pad+1, mid) != ColorActive {
			t.Errorf("%d: janela da esquerda vazia", size)
		}
		if at(size-pad-2, pad+2) != ColorActive || at(size-pad-2, size-pad-3) != ColorActive {
			t.Errorf("%d: janelas da direita vazias", size)
		}
		gapCol := -1
		for x := pad + 1; x < size-pad; x++ {
			if at(x, pad+2) == 0 {
				gapCol = x
				break
			}
		}
		if gapCol < 0 {
			t.Errorf("%d: sem espaço entre as colunas", size)
		}
	}
	if px := Pixels(4, ColorActive); px[0] != 0 {
		t.Error("tamanho minúsculo deveria ficar vazio")
	}
	if Pixels(16, ColorInactive)[3*16+3] != ColorInactive {
		t.Error("cor inativa não aplicada")
	}
}
