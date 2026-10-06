// Package grid tem a matemática do minWinM: retângulos, frações da tela,
// gaps, direções, o formato do grid e as proporções de colunas e linhas.
//
// Não depende do Windows: tudo aqui é determinístico e testável em qualquer
// sistema. Quem posiciona janelas de verdade é o pacote wm.
package grid

import "math"

// Rect é um retângulo em pixels de tela.
//
// Tem o mesmo layout de memória do RECT do Win32 — o pacote win passa
// ponteiros para ele direto à API. Não adicionar campos.
type Rect struct{ Left, Top, Right, Bottom int32 }

func (r Rect) W() int32 { return r.Right - r.Left }
func (r Rect) H() int32 { return r.Bottom - r.Top }

// Center devolve o ponto central do retângulo.
func (r Rect) Center() (x, y float64) {
	return float64(r.Left+r.Right) / 2, float64(r.Top+r.Bottom) / 2
}

// Contains diz se o ponto (x, y) está dentro do retângulo.
func (r Rect) Contains(x, y int32) bool {
	return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

// Frac descreve uma área como fração (0 a 1) da área útil do monitor.
type Frac struct{ X, Y, W, H float64 }

// Gaps: Inner = espaço entre janelas vizinhas, Outer = margem nas bordas da
// tela. Zero = janelas encostadas.
type Gaps struct{ Inner, Outer int32 }

const eps = 1e-9

// TileRect converte a fração em pixels dentro de wa, aplicando os gaps.
// Entre dois tiles cada lado recua metade de Inner (esquerda/cima arredondam
// para cima, para a soma dar exatamente Inner mesmo com valor ímpar).
func TileRect(wa Rect, f Frac, g Gaps) Rect {
	w, h := float64(wa.W()), float64(wa.H())
	r := Rect{
		Left:   wa.Left + int32(math.Round(f.X*w)),
		Top:    wa.Top + int32(math.Round(f.Y*h)),
		Right:  wa.Left + int32(math.Round((f.X+f.W)*w)),
		Bottom: wa.Top + int32(math.Round((f.Y+f.H)*h)),
	}
	edge := func(touches bool, half int32) int32 {
		if touches {
			return g.Outer
		}
		return half
	}
	r.Left += edge(f.X < eps, leadGap(g))
	r.Top += edge(f.Y < eps, leadGap(g))
	r.Right -= edge(f.X+f.W > 1-eps, trailGap(g))
	r.Bottom -= edge(f.Y+f.H > 1-eps, trailGap(g))
	return r
}

// leadGap e trailGap são quanto um tile recua à esquerda/em cima e à
// direita/embaixo de uma divisa interna do grid.
func leadGap(g Gaps) int32  { return g.Inner - g.Inner/2 }
func trailGap(g Gaps) int32 { return g.Inner / 2 }

// RelativeFrac calcula onde r está dentro de wa, em frações.
func RelativeFrac(r, wa Rect) Frac {
	w, h := float64(wa.W()), float64(wa.H())
	return Frac{
		X: float64(r.Left-wa.Left) / w,
		Y: float64(r.Top-wa.Top) / h,
		W: float64(r.W()) / w,
		H: float64(r.H()) / h,
	}
}
