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

// columnsOf devolve, para cada coluna do formato, os slots antigos dela de
// cima para baixo.
func columnsOf(shape []int) [][]int {
	cols := make([][]int, len(shape))
	s := 0
	for i, n := range shape {
		for r := 0; r < n; r++ {
			cols[i] = append(cols[i], s)
			s++
		}
	}
	return cols
}

// colOf devolve a coluna do slot no formato, ou -1 se não existe.
func colOf(shape []int, slot int) int {
	s := 0
	for i, n := range shape {
		if slot < s+n {
			return i
		}
		s += n
	}
	return -1
}

// SoloColumn deixa o slot sozinho na coluna dele e passa as outras janelas
// da coluna para a coluna vizinha (a da direita; na última coluna, a da
// esquerda), empilhadas junto com as que já estavam lá. Devolve o novo
// formato e a nova ordem: order[i] é o slot antigo que vai para o slot i.
// ok é false se não há o que fazer (slot já sozinho ou sem vizinha).
func SoloColumn(shape []int, slot int) (newShape, order []int, ok bool) {
	cols := columnsOf(shape)
	c := colOf(shape, slot)
	if c < 0 || len(cols[c]) < 2 || len(shape) < 2 {
		return nil, nil, false
	}
	var rest []int
	for _, o := range cols[c] {
		if o != slot {
			rest = append(rest, o)
		}
	}
	cols[c] = []int{slot}
	// As que saem ficam do lado de onde vieram: no topo da coluna da
	// direita, no pé da coluna da esquerda.
	if t := c + 1; t < len(cols) {
		cols[t] = append(rest, cols[t]...)
	} else {
		cols[c-1] = append(cols[c-1], rest...)
	}
	for _, col := range cols {
		newShape = append(newShape, len(col))
		order = append(order, col...)
	}
	return newShape, order, true
}

// dockable diz se f é uma área que o grid (colunas no topo, linhas só
// dentro de cada coluna) consegue representar sem deixar sobreposição ou
// buraco: uma coluna inteira em qualquer X (metade, terço, dois-terços ou
// centralizada, tipo "center-third") ou um quadrante encostado nos dois
// eixos. "center" (não é coluna inteira nem está encostada) e "maximize"/
// "top"/"bottom" (ocupariam uma linha atravessando todas as colunas, o
// grid não representa isso) não são.
func dockable(f Frac) bool {
	if f.W >= 1-eps {
		return false
	}
	if f.H > 1-eps {
		return true
	}
	onX := f.X < eps || f.X+f.W > 1-eps
	onY := f.Y < eps || f.Y+f.H > 1-eps
	return onX && onY
}

// dockCol é uma coluna montada por Dock: largura, os slots antigos que
// ficam nela (de cima pra baixo) e as linhas deles (nil = iguais).
type dockCol struct {
	w     float64
	rows  []float64
	slots []int
}

// assembleDock monta shape/colW/rowW/order a partir de colunas da esquerda
// pra direita (mesma convenção de order que SoloColumn: order[i] é o slot
// antigo que vai para o slot novo i).
func assembleDock(cols []dockCol) (shape []int, colW []float64, rowW [][]float64, order []int) {
	for _, c := range cols {
		shape = append(shape, len(c.slots))
		colW = append(colW, c.w)
		rows := c.rows
		if rows == nil {
			rows = equalParts(len(c.slots))
		}
		rowW = append(rowW, rows)
		order = append(order, c.slots...)
	}
	return
}

// Dock dá ao slot a área f (ver dockable) e devolve o novo formato e as
// novas proporções para que as outras janelas que ocupam slot agora
// preencham o resto da tela sem sobrepor. ok é false se f não é
// encaixável; quem chama cai de volta pro overlay flutuante de sempre.
func Dock(shape []int, slot int, f Frac) (newShape []int, colW []float64, rowW [][]float64, order []int, ok bool) {
	if !dockable(f) {
		return nil, nil, nil, nil, false
	}
	total := sum(shape)
	if slot < 0 || slot >= total {
		return nil, nil, nil, nil, false
	}
	var rest []int
	for s := 0; s < total; s++ {
		if s != slot {
			rest = append(rest, s)
		}
	}
	if len(rest) == 0 { // janela sozinha: nada pra arrumar
		newShape, colW, rowW, order = assembleDock([]dockCol{{w: 1, slots: []int{slot}}})
		return newShape, colW, rowW, order, true
	}

	if f.H > 1-eps { // coluna inteira: left/right/terço/dois-terços/centralizada
		own := dockCol{w: f.W, slots: []int{slot}}
		leftAnchored, rightAnchored := f.X < eps, f.X+f.W > 1-eps
		switch {
		case leftAnchored:
			newShape, colW, rowW, order = assembleDock([]dockCol{own, {w: 1 - f.W, slots: rest}})
		case rightAnchored:
			newShape, colW, rowW, order = assembleDock([]dockCol{{w: 1 - f.W, slots: rest}, own})
		default:
			// Centralizada (ex.: center-third): divide o resto pela coluna
			// original (não pelo número do slot — um colega empilhado na
			// MESMA coluna, acima ou abaixo, não é "da esquerda" só por ter
			// um índice menor; fica do lado "depois", por convenção). Se um
			// dos lados fica vazio, a largura dele some com a do preset —
			// sem isso haveria um vão sem nenhuma janela.
			sc := colOf(shape, slot)
			var before, after []int
			for _, s := range rest {
				if colOf(shape, s) < sc {
					before = append(before, s)
				} else {
					after = append(after, s)
				}
			}
			switch {
			case len(before) == 0:
				own.w = f.X + f.W
				newShape, colW, rowW, order = assembleDock([]dockCol{own, {w: 1 - own.w, slots: after}})
			case len(after) == 0:
				own.w = 1 - f.X
				newShape, colW, rowW, order = assembleDock([]dockCol{{w: 1 - own.w, slots: before}, own})
			default:
				newShape, colW, rowW, order = assembleDock([]dockCol{
					{w: f.X, slots: before},
					own,
					{w: 1 - f.X - f.W, slots: after},
				})
			}
		}
		return newShape, colW, rowW, order, true
	}

	// Quadrante: a coluna do slot fica com ele + os colegas da coluna
	// original (empresta 1 dos demais se não havia nenhum), a oposta leva
	// o resto.
	leftAnchored := f.X < eps
	c := colOf(shape, slot)
	var mates []int
	for _, s := range columnsOf(shape)[c] {
		if s != slot {
			mates = append(mates, s)
		}
	}
	var others []int
	for _, s := range rest {
		keep := true
		for _, m := range mates {
			if m == s {
				keep = false
			}
		}
		if keep {
			others = append(others, s)
		}
	}
	if len(mates) == 0 {
		mates, others = others[:1], others[1:]
	}
	if len(others) == 0 { // só 2 janelas no total: não dá pra fazer 4 partes
		own := dockCol{w: f.W, slots: []int{slot}}
		if leftAnchored {
			newShape, colW, rowW, order = assembleDock([]dockCol{own, {w: 1 - f.W, slots: mates}})
		} else {
			newShape, colW, rowW, order = assembleDock([]dockCol{{w: 1 - f.W, slots: mates}, own})
		}
		return newShape, colW, rowW, order, true
	}

	topAnchored := f.Y < eps
	mateRows := equalParts(len(mates))
	for i := range mateRows {
		mateRows[i] *= 1 - f.H
	}
	var ownSlots []int
	var rows []float64
	if topAnchored {
		ownSlots = append([]int{slot}, mates...)
		rows = append([]float64{f.H}, mateRows...)
	} else {
		ownSlots = append(append([]int(nil), mates...), slot)
		rows = append(mateRows, f.H)
	}
	own := dockCol{w: f.W, rows: rows, slots: ownSlots}
	other := dockCol{w: 1 - f.W, slots: others}
	if leftAnchored {
		newShape, colW, rowW, order = assembleDock([]dockCol{own, other})
	} else {
		newShape, colW, rowW, order = assembleDock([]dockCol{other, own})
	}
	return newShape, colW, rowW, order, true
}

// Layout é o grid de um monitor: quantas janelas há em cada coluna e as
// proporções das colunas e das linhas de cada coluna (cada lista soma 1).
//
// Os slots são numerados coluna a coluna, de cima para baixo. As
// proporções pertencem aos slots; quando o formato do grid muda (uma janela
// entra ou sai), FitKeep as recalcula a partir do tamanho que cada janela
// tinha (Fit simplesmente as iguala). O valor zero é um grid vazio.
//
// O formato normalmente é Shape(n); SetShape fixa outro até o número de
// janelas mudar.
type Layout struct {
	shape  []int
	colW   []float64
	rowW   [][]float64
	custom bool // formato fixado por SetShape
}

// Fit ajusta o grid para n janelas; se o formato mudar, zera as proporções.
// Um formato fixado vale enquanto continuar sendo para n janelas.
func (l *Layout) Fit(n int) { l.FitKeep(make([]Frac, n)) }

// FitKeep é Fit(len(prev)), mas, se o formato mudar, as proporções novas
// partem do tamanho que cada janela tinha: prev[i] é a fração antiga da
// janela que fica no slot i (W == 0 = desconhecida, ex.: janela nova). Ver
// keepSizes.
func (l *Layout) FitKeep(prev []Frac) {
	n := len(prev)
	if l.custom && sum(l.shape) == n {
		return
	}
	l.custom = false
	if shape := Shape(n); !equalInts(shape, l.shape) {
		l.shape = shape
		l.colW, l.rowW = keepSizes(shape, prev)
	}
}

// keepSizes calcula as proporções de um formato novo a partir do tamanho
// antigo das janelas: a largura de uma coluna é a média das larguras que
// suas janelas tinham, e as linhas mantêm a altura relativa de cada uma;
// depois tudo é reescalado para fechar a tela. Tamanho desconhecido vale a
// média das conhecidas (iguais, se nenhuma é conhecida).
func keepSizes(shape []int, prev []Frac) ([]float64, [][]float64) {
	cols := make([]float64, len(shape))
	rows := make([][]float64, len(shape))
	s := 0
	for c, k := range shape {
		var ws []float64
		hs := make([]float64, k)
		for r := range hs {
			if f := prev[s+r]; f.W > 0 && f.H > 0 {
				ws = append(ws, f.W)
				hs[r] = f.H
			}
		}
		cols[c] = mean(ws)
		rows[c] = normalize(fillUnknown(hs))
		s += k
	}
	return normalize(fillUnknown(cols)), rows
}

// mean é a média de v, ou 0 (desconhecido) se v está vazio.
func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	t := 0.0
	for _, x := range v {
		t += x
	}
	return t / float64(len(v))
}

// fillUnknown troca os zeros pela média dos outros valores (ou por 1, se
// todos são zero).
func fillUnknown(parts []float64) []float64 {
	var known []float64
	for _, p := range parts {
		if p > 0 {
			known = append(known, p)
		}
	}
	m := mean(known)
	if m == 0 {
		m = 1
	}
	for i, p := range parts {
		if p <= 0 {
			parts[i] = m
		}
	}
	return parts
}

// normalize reescala parts para somar 1, com cada parte ≥ MinPart (o que
// falta às pequenas sai das outras, na proporção do que elas têm acima do
// mínimo).
func normalize(parts []float64) []float64 {
	if float64(len(parts))*MinPart > 1 {
		return equalParts(len(parts))
	}
	total := 0.0
	for _, p := range parts {
		total += p
	}
	for i := range parts {
		parts[i] /= total
	}
	deficit, spare := 0.0, 0.0
	for _, p := range parts {
		if p < MinPart {
			deficit += MinPart - p
		} else {
			spare += p - MinPart
		}
	}
	if deficit == 0 {
		return parts
	}
	for i, p := range parts {
		if p < MinPart {
			parts[i] = MinPart
		} else {
			parts[i] = p - (p-MinPart)*deficit/spare
		}
	}
	return parts
}

// SetShape fixa o formato (janelas por coluna). Mantém as larguras das
// colunas se o número delas não mudou; as linhas ficam iguais.
func (l *Layout) SetShape(shape []int) {
	keepCols := len(shape) == len(l.colW)
	l.shape, l.custom = append([]int(nil), shape...), true
	if !keepCols {
		l.colW = equalParts(len(shape))
	}
	l.rowW = make([][]float64, len(shape))
	for c, n := range shape {
		l.rowW[c] = equalParts(n)
	}
}

// SetLayout é SetShape, mas com colW/rowW já calculados (ex.: por Dock) em
// vez de reiniciar tudo igual.
func (l *Layout) SetLayout(shape []int, colW []float64, rowW [][]float64) {
	l.shape, l.custom = append([]int(nil), shape...), true
	l.colW = append([]float64(nil), colW...)
	l.rowW = make([][]float64, len(rowW))
	for c, r := range rowW {
		l.rowW[c] = append([]float64(nil), r...)
	}
}

// ClearShape volta ao formato automático no próximo Fit.
func (l *Layout) ClearShape() { l.shape, l.custom = nil, false }

// Custom diz se o formato atual foi fixado por SetShape.
func (l *Layout) Custom() bool { return l.custom }

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

// ResetExcept é Reset, mas mantém a largura e as linhas da coluna keep como
// estão — usada por "balance" pra não desfazer a largura de um preset
// dockado (ela é o valor do preset, não um redimensionamento manual pra
// desfazer). O resto da largura (1 - a da coluna keep) é dividida em partes
// iguais entre as outras colunas.
func (l *Layout) ResetExcept(keep int) {
	if keep < 0 || keep >= len(l.shape) {
		l.Reset()
		return
	}
	keptW, keptRows := l.colW[keep], l.rowW[keep]
	others := equalParts(len(l.shape) - 1)
	colW := make([]float64, len(l.shape))
	rowW := make([][]float64, len(l.shape))
	j := 0
	for c, n := range l.shape {
		if c == keep {
			colW[c], rowW[c] = keptW, keptRows
			continue
		}
		colW[c] = others[j] * (1 - keptW)
		rowW[c] = equalParts(n)
		j++
	}
	l.colW, l.rowW = colW, rowW
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

func sum(a []int) int {
	t := 0
	for _, v := range a {
		t += v
	}
	return t
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
