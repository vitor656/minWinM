package grid

import "math"

// Dir é uma direção de navegação (h/j/k/l no estilo Vim).
type Dir int

const (
	Left Dir = iota
	Down
	Up
	Right
)

var dirNames = map[string]Dir{"left": Left, "down": Down, "up": Up, "right": Right}

// ParseDir converte "left", "down", "up" ou "right" em Dir.
func ParseDir(s string) (Dir, bool) {
	d, ok := dirNames[s]
	return d, ok
}

// DirNames lista os nomes aceitos por ParseDir.
func DirNames() []string { return []string{"left", "down", "up", "right"} }

// Neighbor escolhe, entre cands, o retângulo mais próximo de from na
// direção d, penalizando o desvio no eixo perpendicular. -1 se não há.
func Neighbor(from Rect, cands []Rect, d Dir) int {
	fx, fy := from.Center()
	best, bestScore := -1, math.Inf(1)
	for i, c := range cands {
		cx, cy := c.Center()
		var along, across float64
		switch d {
		case Left:
			along, across = fx-cx, math.Abs(fy-cy)
		case Right:
			along, across = cx-fx, math.Abs(fy-cy)
		case Up:
			along, across = fy-cy, math.Abs(fx-cx)
		case Down:
			along, across = cy-fy, math.Abs(fx-cx)
		}
		if along <= 0 {
			continue
		}
		if score := along + 2*across; score < bestScore {
			best, bestScore = i, score
		}
	}
	return best
}

// EntryIndex diz em que posição (0..len(cands)) uma janela vinda de from,
// movendo-se na direção d, entra numa lista de retângulos que segue a ordem
// do grid (coluna a coluna, de cima para baixo): toma o lugar do mais
// próximo na direção (Right/Down) ou fica logo depois dele (Left/Up) — ou
// seja, entra pelo lado de onde veio. Sem candidatos na direção, vai para o
// começo (Right/Down) ou o fim (Left/Up).
func EntryIndex(from Rect, cands []Rect, d Dir) int {
	after := d == Left || d == Up
	k := Neighbor(from, cands, d)
	switch {
	case k < 0 && after:
		return len(cands)
	case k < 0:
		return 0
	case after:
		return k + 1
	}
	return k
}
