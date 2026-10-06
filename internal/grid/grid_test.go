package grid

import (
	"math"
	"testing"
)

func TestShape(t *testing.T) {
	cases := map[int][]int{
		0: nil, 1: {1}, 2: {1, 1}, 3: {1, 2}, 4: {2, 2},
		5: {1, 2, 2}, 7: {2, 2, 3}, 9: {3, 3, 3}, 10: {2, 2, 3, 3},
	}
	for n, want := range cases {
		if got := Shape(n); !equalInts(got, want) {
			t.Errorf("Shape(%d) = %v, want %v", n, got, want)
		}
	}
}

func TestFitResetsOnlyWhenShapeChanges(t *testing.T) {
	var l Layout
	l.Fit(3)
	l.Grow(0, true, 0.2)
	grown := l.Snapshot()
	l.Fit(3) // mesmo formato: mantém
	if !equalFloats(l.colW, grown.col) {
		t.Fatalf("Fit com mesmo formato zerou: %v", l.colW)
	}
	l.Fit(4) // formato novo: zera
	if !equalFloats(l.colW, []float64{0.5, 0.5}) {
		t.Fatalf("Fit com formato novo não zerou: %v", l.colW)
	}
}

func TestCellsCoverScreen(t *testing.T) {
	var l Layout
	l.Fit(5)
	adjust(l.colW, 1, 0.1)
	adjust(l.rowW[2], 0, -0.05)
	area := 0.0
	for i := 0; i < 5; i++ {
		f := l.Cell(i)
		area += f.W * f.H
	}
	if math.Abs(area-1) > 1e-9 {
		t.Fatalf("células somam %v da tela, want 1", area)
	}
	if c, r := l.ColRow(4); c != 2 || r != 1 {
		t.Fatalf("ColRow(4) = %d,%d, want 2,1", c, r)
	}
}

func TestAdjust(t *testing.T) {
	p := equalParts(3)
	adjust(p, 0, 0.2)
	if sum := p[0] + p[1] + p[2]; math.Abs(sum-1) > 1e-9 {
		t.Fatalf("soma %v", sum)
	}
	if p[0] <= 1.0/3 || math.Abs(p[1]-p[2]) > 1e-9 {
		t.Fatalf("parts %v", p)
	}
	for i := 0; i < 50; i++ {
		adjust(p, 0, 0.2)
	}
	if p[1] < MinPart-1e-9 {
		t.Fatalf("vizinha abaixo do mínimo: %v", p)
	}
	one := []float64{1}
	adjust(one, 0, 0.2)
	if one[0] != 1 {
		t.Fatalf("coluna única mudou: %v", one)
	}
}

func TestNeighbor(t *testing.T) {
	// Grid 2 colunas: A ocupa a esquerda inteira, B em cima e C embaixo à direita.
	a := Rect{0, 0, 500, 1000}
	b := Rect{500, 0, 1000, 500}
	c := Rect{500, 500, 1000, 1000}
	all := []Rect{a, b, c}
	if got := Neighbor(c, all, Left); got != 0 {
		t.Errorf("C esquerda = %d, want A", got)
	}
	if got := Neighbor(b, all, Down); got != 2 {
		t.Errorf("B baixo = %d, want C", got)
	}
	if got := Neighbor(c, all, Up); got != 1 {
		t.Errorf("C cima = %d, want B", got)
	}
	if got := Neighbor(b, all, Right); got != -1 {
		t.Errorf("B direita = %d, want nenhum", got)
	}
}

func TestParseDir(t *testing.T) {
	for _, name := range DirNames() {
		if _, ok := ParseDir(name); !ok {
			t.Errorf("ParseDir(%q) falhou", name)
		}
	}
	if _, ok := ParseDir("diagonal"); ok {
		t.Error("ParseDir aceitou direção inválida")
	}
}

func TestTileRectGaps(t *testing.T) {
	wa := Rect{0, 0, 1000, 600}
	left := TileRect(wa, Frac{0, 0, 0.5, 1}, Gaps{Inner: 7, Outer: 3})
	right := TileRect(wa, Frac{0.5, 0, 0.5, 1}, Gaps{Inner: 7, Outer: 3})
	if left.Left != 3 || left.Top != 3 || left.Bottom != 597 || right.Right != 997 {
		t.Fatalf("bordas erradas: %+v %+v", left, right)
	}
	if got := right.Left - left.Right; got != 7 {
		t.Fatalf("espaço entre janelas = %d, want 7", got)
	}
	if r := TileRect(wa, Frac{0, 0, 0.5, 1}, Gaps{}); r != (Rect{0, 0, 500, 600}) {
		t.Fatalf("sem gap: %+v", r)
	}
}

func TestPresets(t *testing.T) {
	if f, ok := Preset("center"); !ok || f != (Frac{0.15, 0.1, 0.7, 0.8}) {
		t.Fatalf("center = %+v, %v", f, ok)
	}
	for _, name := range PresetNames() {
		if _, ok := Preset(name); !ok {
			t.Errorf("PresetNames tem %q mas Preset não acha", name)
		}
	}
	if _, ok := Preset("nenhum"); ok {
		t.Fatal("Preset aceitou nome inexistente")
	}
}

func TestFollowEdges(t *testing.T) {
	wa := Rect{0, 0, 1000, 600}
	g := Gaps{Inner: 4}
	// 3 colunas iguais; arrasta a borda direita da do meio até x=800.
	l := &Layout{shape: []int{1, 2, 1}}
	l.Reset()
	start := TileRect(wa, l.Cell(1), g)
	got := start
	got.Right = 800 - g.Inner/2
	l.FollowEdges(wa, g, 1, false, start, got)
	if math.Abs(l.colW[0]-1.0/3) > 1e-9 {
		t.Fatalf("coluna da esquerda mudou: %v", l.colW)
	}
	if math.Abs(l.colW[0]+l.colW[1]-0.8) > 1e-9 || math.Abs(l.colW[2]-0.2) > 1e-9 {
		t.Fatalf("divisa não foi para 0.8: %v", l.colW)
	}
	if r := TileRect(wa, l.Cell(1), g); r.Right != got.Right {
		t.Fatalf("borda no grid = %d, want %d", r.Right, got.Right)
	}

	// Borda de baixo da janela de cima da coluna do meio sobe para y=150.
	start = TileRect(wa, l.Cell(1), g)
	got = start
	got.Bottom = 150 - g.Inner/2
	l.FollowEdges(wa, g, 1, false, start, got)
	if math.Abs(l.rowW[1][0]-0.25) > 1e-9 {
		t.Fatalf("linhas = %v, want [0.25 0.75]", l.rowW[1])
	}

	// Com altura total, a borda de baixo não mexe nas linhas.
	before := l.Snapshot()
	start = TileRect(wa, l.Cell(1), g)
	got = start
	got.Bottom += 100
	l.FollowEdges(wa, g, 1, true, start, got)
	if !equalFloats(l.rowW[1], before.row[1]) {
		t.Fatalf("altura total mexeu nas linhas: %v", l.rowW[1])
	}

	// Arrastar além do limite respeita MinPart.
	start = TileRect(wa, l.Cell(1), g)
	got = start
	got.Right = 999
	l.FollowEdges(wa, g, 1, false, start, got)
	if l.colW[2] < MinPart-1e-9 {
		t.Fatalf("coluna abaixo do mínimo: %v", l.colW)
	}

	// Borda sem vizinha (esquerda da primeira coluna) é ignorada.
	before = l.Snapshot()
	start = TileRect(wa, l.Cell(0), g)
	got = start
	got.Left += 50
	l.FollowEdges(wa, g, 0, false, start, got)
	if !equalFloats(l.colW, before.col) {
		t.Fatalf("borda externa mexeu no grid: %v", l.colW)
	}
}

func TestRestoreIgnoresOtherShape(t *testing.T) {
	var l Layout
	l.Fit(2)
	old := l.Snapshot()
	l.Fit(9)
	l.Restore(old) // não pode entrar em pânico nem mudar nada
	if len(l.colW) != 3 {
		t.Fatalf("colW = %v", l.colW)
	}
}

func TestBlend(t *testing.T) {
	var l Layout
	l.Fit(2)
	a := l.Snapshot()
	l.Grow(0, true, 0.2)
	b := l.Snapshot()
	mid := Blend(a, b, 0.5)
	if math.Abs(mid.col[0]-0.6) > 1e-9 || math.Abs(mid.col[0]+mid.col[1]-1) > 1e-9 {
		t.Fatalf("Blend = %v", mid.col)
	}
}

func equalFloats(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Abs(a[i]-b[i]) > 1e-12 {
			return false
		}
	}
	return true
}
