//go:build windows

package wm

import (
	"minwinm/internal/grid"
	"minwinm/internal/win"
)

// tile é uma janela gerenciada.
type tile struct {
	hwnd uintptr
	// floating: fora do grid (centralizada ou num preset), mas o slot
	// continua reservado para ela voltar ao mesmo lugar.
	floating bool
	preset   string // preset em que está flutuando
	tall     bool   // ocupa a altura inteira da coluna (toggle-full-height)
	desc     string // identificação para o log (só preenchida com o log detalhado)
}

// workspace é o grid de um monitor numa área de trabalho virtual. tiles fica
// na ordem do grid, coluna a coluna, de cima para baixo; janelas
// minimizadas continuam na lista (sem ocupar slot) para voltarem ao mesmo
// lugar.
type workspace struct {
	mon   uintptr
	desk  win.DesktopID
	tiles []*tile
	grid.Layout
	solo soloState
}

// soloState guarda o toggle-solo-column: a janela que ganhou a coluna e o
// arranjo automático de antes, para voltar a ele. Só vale enquanto o
// formato fixado durar (ver grid.Layout.Custom).
type soloState struct {
	tile  *tile
	tiles []*tile    // ordem antes do primeiro solo
	sizes grid.Sizes // proporções antes do primeiro solo
}

// unsolo volta ao formato automático, com a ordem e as proporções de antes
// do solo (a ordem só se as janelas ainda forem as mesmas).
func (ws *workspace) unsolo(mon win.Monitor) {
	prev := ws.solo
	ws.solo = soloState{}
	ws.ClearShape()
	if sameTiles(prev.tiles, ws.tiles) {
		copy(ws.tiles, prev.tiles)
	}
	ws.slots(mon) // refaz o formato automático antes de restaurar
	ws.Restore(prev.sizes)
}

// sameTiles diz se a e b têm os mesmos tiles, em qualquer ordem.
func sameTiles(a, b []*tile) bool {
	if len(a) != len(b) {
		return false
	}
	in := map[*tile]bool{}
	for _, t := range a {
		in[t] = true
	}
	for _, t := range b {
		if !in[t] {
			return false
		}
	}
	return true
}

// slots devolve os tiles que ocupam o grid agora (inclusive flutuantes, que
// reservam o slot) e ajusta o Layout para essa quantidade. O índice na
// lista devolvida é o número do slot.
func (ws *workspace) slots(mon win.Monitor) []*tile {
	var out []*tile
	for _, tl := range ws.tiles {
		if occupies(tl, mon) {
			out = append(out, tl)
		}
	}
	ws.Fit(len(out))
	return out
}

// slotOf devolve o número do slot de tl em slots, ou -1.
func slotOf(slots []*tile, tl *tile) int {
	for s, o := range slots {
		if o == tl {
			return s
		}
	}
	return -1
}

// slotFrac é a célula do slot i, ou a coluna inteira se o tile está em
// altura total (as vizinhas da coluna ficam atrás dele).
func (ws *workspace) slotFrac(slots []*tile, i int) grid.Frac {
	f := ws.Cell(i)
	if slots[i].tall {
		f.Y, f.H = 0, 1
	}
	return f
}

// slotRect é o retângulo em pixels do slot i.
func (ws *workspace) slotRect(mon win.Monitor, slots []*tile, i int, g grid.Gaps) grid.Rect {
	return grid.TileRect(mon.Work, ws.slotFrac(slots, i), g)
}

// sameColumn devolve os slots da mesma coluna do slot i.
func (ws *workspace) sameColumn(slots []*tile, i int) []*tile {
	c, _ := ws.ColRow(i)
	var out []*tile
	for s, tl := range slots {
		if sc, _ := ws.ColRow(s); sc == c {
			out = append(out, tl)
		}
	}
	return out
}

// coveredByTall diz se outro slot da coluna do slot i está em altura total.
func (ws *workspace) coveredByTall(slots []*tile, i int) bool {
	for _, sib := range ws.sameColumn(slots, i) {
		if sib.tall && sib != slots[i] {
			return true
		}
	}
	return false
}

// overflows diz se alguma janela do grid ficou maior que o seu slot
// (tamanho mínimo do app), ou seja, se está sobrepondo uma vizinha.
func (ws *workspace) overflows(mon win.Monitor, g grid.Gaps) bool {
	const tolerance = 2
	slots := ws.slots(mon)
	for i, tl := range slots {
		if tl.floating || win.IsHung(tl.hwnd) || (!tl.tall && ws.coveredByTall(slots, i)) {
			continue // flutuantes e vizinhas de um tile em altura total ficam atrás de propósito
		}
		got, ok := win.FrameBounds(tl.hwnd)
		if !ok {
			continue
		}
		want := ws.slotRect(mon, slots, i, g)
		if got.W() > want.W()+tolerance || got.H() > want.H()+tolerance {
			return true
		}
	}
	return false
}
