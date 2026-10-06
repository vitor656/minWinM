package grid

import "math"

// MinPart é a menor fração da tela que uma coluna ou linha pode ter.
const MinPart = 0.1

// Shape distribui n janelas em ceil(√n) colunas; as colunas da direita
// recebem as sobras (3 janelas = uma grande à esquerda + duas empilhadas).
// O resultado é o número de janelas em cada coluna.
func Shape(n int) []int {
	if n == 0 {
		return nil
	}
	cols := int(math.Ceil(math.Sqrt(float64(n))))
	shape := make([]int, cols)
	for c := range shape {
		shape[c] = n / cols
		if c >= cols-n%cols {
			shape[c]++
		}
	}
	return shape
}

// Layout é o grid de um monitor: quantas janelas há em cada coluna e as
// proporções das colunas e das linhas de cada coluna (cada lista soma 1).
//
// Os slots são numerados coluna a coluna, de cima para baixo. As
// proporções pertencem aos slots e voltam a ser iguais quando o formato do
// grid muda (uma janela entra ou sai). O valor zero é um grid vazio.
type Layout struct {
	shape []int
	colW  []float64
	rowW  [][]float64
}

// Fit ajusta o grid para n janelas; se o formato mudar, zera as proporções.
func (l *Layout) Fit(n int) {
	if shape := Shape(n); !equalInts(shape, l.shape) {
		l.shape = shape
		l.Reset()
	}
}

// Shape devolve uma cópia do formato atual (janelas por coluna).
func (l *Layout) Shape() []int { return append([]int(nil), l.shape...) }

// SameShape diz se o formato atual é igual a shape.
func (l *Layout) SameShape(shape []int) bool { return equalInts(l.shape, shape) }

// Reset deixa todas as colunas e linhas do mesmo tamanho.
func (l *Layout) Reset() {
	l.colW = equalParts(len(l.shape))
	l.rowW = make([][]float64, len(l.shape))
	for c, n := range l.shape {
		l.rowW[c] = equalParts(n)
	}
}

// Cell devolve a fração do monitor ocupada pelo slot i.
func (l *Layout) Cell(i int) Frac {
	x := 0.0
	for c, n := range l.shape {
		if i >= n {
			i -= n
			x += l.colW[c]
			continue
		}
		y := 0.0
		for r := 0; r < i; r++ {
			y += l.rowW[c][r]
		}
		return Frac{x, y, l.colW[c], l.rowW[c][i]}
	}
	return Frac{}
}

// ColRow converte o índice do slot em (coluna, linha); (-1, -1) se não existe.
func (l *Layout) ColRow(i int) (col, row int) {
	for c, n := range l.shape {
		if i < n {
			return c, i
		}
		i -= n
	}
	return -1, -1
}

// Grow muda a largura (horizontal) ou a altura do slot em delta, tirando o
// espaço proporcionalmente das vizinhas para o grid continuar fechado.
func (l *Layout) Grow(slot int, horizontal bool, delta float64) {
	c, r := l.ColRow(slot)
	if c < 0 {
		return
	}
	if horizontal {
		adjust(l.colW, c, delta)
	} else {
		adjust(l.rowW[c], r, delta)
	}
}

// FollowEdges ajusta as proporções para que as bordas do slot (que estavam
// em start) fiquem onde estão as bordas de got — usado quando o usuário
// estica a janela com o mouse. Só contam as bordas que mudaram e que têm
// vizinha; só a divisa arrastada se move. fullHeight: o slot ocupa a coluna
// inteira, então a altura não tem divisa.
func (l *Layout) FollowEdges(wa Rect, g Gaps, slot int, fullHeight bool, start, got Rect) {
	c, r := l.ColRow(slot)
	if c < 0 {
		return
	}
	x := func(px int32) float64 { return float64(px-wa.Left) / float64(wa.W()) }
	y := func(px int32) float64 { return float64(px-wa.Top) / float64(wa.H()) }

	if got.Left != start.Left && c > 0 {
		setBoundary(l.colW, c-1, x(got.Left-leadGap(g)))
	}
	if got.Right != start.Right && c < len(l.colW)-1 {
		setBoundary(l.colW, c, x(got.Right+trailGap(g)))
	}
	if fullHeight {
		return
	}
	rows := l.rowW[c]
	if got.Top != start.Top && r > 0 {
		setBoundary(rows, r-1, y(got.Top-leadGap(g)))
	}
	if got.Bottom != start.Bottom && r < len(rows)-1 {
		setBoundary(rows, r, y(got.Bottom+trailGap(g)))
	}
}

// Sizes é uma cópia das proporções de um Layout.
type Sizes struct {
	col []float64
	row [][]float64
}

// Snapshot copia as proporções atuais.
func (l *Layout) Snapshot() Sizes {
	s := Sizes{col: append([]float64(nil), l.colW...)}
	for _, r := range l.rowW {
		s.row = append(s.row, append([]float64(nil), r...))
	}
	return s
}

// Restore volta para s; não faz nada se o grid mudou de formato desde a cópia.
func (l *Layout) Restore(s Sizes) {
	if len(s.col) != len(l.colW) || len(s.row) != len(l.rowW) {
		return
	}
	for c := range l.rowW {
		if len(s.row[c]) != len(l.rowW[c]) {
			return
		}
	}
	copy(l.colW, s.col)
	for c := range l.rowW {
		copy(l.rowW[c], s.row[c])
	}
}

// Blend devolve a + (b-a)*k, parte a parte (mantém as somas iguais a 1).
// a e b devem vir do mesmo formato de grid.
func Blend(a, b Sizes, k float64) Sizes {
	out := Sizes{col: make([]float64, len(a.col))}
	for i := range a.col {
		out.col[i] = a.col[i] + (b.col[i]-a.col[i])*k
	}
	for c := range a.row {
		r := make([]float64, len(a.row[c]))
		for i := range r {
			r[i] = a.row[c][i] + (b.row[c][i]-a.row[c][i])*k
		}
		out.row = append(out.row, r)
	}
	return out
}

// adjust soma delta a parts[i] e reescala as demais para manter a soma 1.
func adjust(parts []float64, i int, delta float64) {
	if len(parts) < 2 {
		return
	}
	v := math.Max(MinPart, math.Min(parts[i]+delta, 1-MinPart*float64(len(parts)-1)))
	rest := 1 - parts[i]
	for j := range parts {
		if j != i {
			parts[j] *= (1 - v) / rest
		}
	}
	parts[i] = v
}

// setBoundary move a divisa entre parts[j] e parts[j+1] para a posição b
// (fração da tela), sem mexer nas outras partes.
func setBoundary(parts []float64, j int, b float64) {
	start := 0.0
	for _, p := range parts[:j] {
		start += p
	}
	total := parts[j] + parts[j+1]
	if total < 2*MinPart {
		return
	}
	v := min(max(b-start, MinPart), total-MinPart)
	parts[j], parts[j+1] = v, total-v
}

func equalParts(n int) []float64 {
	p := make([]float64, n)
	for i := range p {
		p[i] = 1 / float64(n)
	}
	return p
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
