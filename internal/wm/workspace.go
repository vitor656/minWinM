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
	// frac é a célula do tile no último slots(): o tamanho que ele tinha,
	// usado para refazer o grid quando o formato muda (ver grid.FitKeep).
	// Zero = ainda sem lugar no grid.
	frac grid.Frac
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
	pin pinState
}

// pinState guarda o "encaixe customizado" ativo — toggle-solo-column ou um
// preset de borda dockado (ver grid.Dock) — e o arranjo automático de antes,
// para voltar a ele. Só vale enquanto o formato fixado durar (ver
// grid.Layout.Custom). name é "" para solo-column, ou o nome do preset para
// um dock; os dois disputam o mesmo encaixe (só um vale por vez).
type pinState struct {
	tile  *tile
	name  string
	tiles []*tile    // ordem antes do primeiro encaixe
	sizes grid.Sizes // proporções antes do primeiro encaixe
}

// unpin volta ao formato automático, com a ordem e as proporções de antes
// do encaixe (a ordem só se as janelas ainda forem as mesmas).
func (ws *workspace) unpin(mon win.Monitor) {
	prev := ws.pin
	ws.pin = pinState{}
	ws.ClearShape()
	if sameTiles(prev.tiles, ws.tiles) {
		copy(ws.tiles, prev.tiles)
	}
	ws.slots(mon) // refaz o formato automático antes de restaurar
	ws.Restore(prev.sizes)
}

// takeoverPin dá o encaixe customizado a tl: se pertencia a outro tile,
// limpa o preset dele (pode ter sido um dock); se não havia nenhum encaixe
// customizado ainda, guarda o arranjo de antes para poder desfazer.
func (ws *workspace) takeoverPin(tl *tile) {
	if ws.pin.tile != nil && ws.pin.tile != tl {
		ws.pin.tile.preset = ""
	}
	if !ws.Custom() {
		ws.pin.tiles = append([]*tile(nil), ws.tiles...)
		ws.pin.sizes = ws.Snapshot()
	}
	ws.pin.tile = tl
}

// reorder aplica a nova ordem (devolvida por grid.SoloColumn ou grid.Dock)
// aos tiles que ocupam slot agora; os que não ocupam (minimizados etc.)
// ficam onde estão.
func (ws *workspace) reorder(slots []*tile, order []int) {
	occupied := map[*tile]bool{}
	for _, o := range slots {
		occupied[o] = true
	}
	var pos []int
	for j, o := range ws.tiles {
		if occupied[o] {
			pos = append(pos, j)
		}
	}
	for k, o := range order {
		ws.tiles[pos[k]] = slots[o]
	}
}

// dockedColumn devolve a coluna de um preset dockado ativo (não
// toggle-solo-column, que não tem uma largura "do preset" pra preservar).
func (ws *workspace) dockedColumn(slots []*tile) (int, bool) {
	if ws.pin.name == "" || ws.pin.tile == nil {
		return 0, false
	}
	s := slotOf(slots, ws.pin.tile)
	if s < 0 {
		return 0, false
	}
	c, _ := ws.ColRow(s)
	return c, c >= 0
}

// tallBlockedByPin diz se o slot s está na coluna de um preset dockado
// ativo — nela, toggle-full-height não faz nada: cobriria a janela presa no
// preset ou destruiria a banda reservada para ela.
func (ws *workspace) tallBlockedByPin(slots []*tile, s int) bool {
	if ws.pin.name == "" || ws.pin.tile == nil {
		return false
	}
	anchor := slotOf(slots, ws.pin.tile)
	if anchor < 0 {
		return false
	}
	pc, _ := ws.ColRow(anchor)
	sc, _ := ws.ColRow(s)
	return pc >= 0 && pc == sc
}

// redock mantém um preset dockado ativo (ws.pin.name != "") reencaixado
// quando a contagem de janelas que ocupam slot muda (abrir, fechar ou
// minimizar OUTRA janela do mesmo grid) — só desfaz se a própria âncora
// sair do grid. toggle-solo-column (ws.pin.name == "") continua voltando ao
// automático nesse caso, como antes (ver FitKeep).
func (ws *workspace) redock(out []*tile) []*tile {
	if ws.pin.name == "" || ws.pin.tile == nil || !ws.Custom() {
		return out
	}
	i := -1
	for j, tl := range out {
		if tl == ws.pin.tile {
			i = j
		}
	}
	if i < 0 {
		// Âncora saiu do grid (mudou de área/monitor, minimizou): volta ao
		// automático. tile.preset é dela, não do workspace — limpa aqui,
		// senão fica um rótulo "dockado" obsoleto numa janela que já não é.
		ws.pin.tile.preset = ""
		ws.pin = pinState{}
		return out
	}
	total := 0
	for _, c := range ws.Shape() {
		total += c
	}
	if total == len(out) {
		return out
	}
	f, _ := grid.Preset(ws.pin.name)
	newShape, colW, rowW, order, ok := grid.Dock(grid.Shape(len(out)), i, f)
	if !ok { // defensivo; não deveria acontecer (mesmo preset já era dockable)
		ws.pin.tile.preset = ""
		ws.pin = pinState{}
		return out
	}
	ws.reorder(out, order)
	ws.SetLayout(newShape, colW, rowW)
	reordered := make([]*tile, len(order))
	for k, o := range order {
		reordered[k] = out[o]
	}
	return reordered
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
	out = ws.redock(out) // reencaixa um preset dockado se a contagem mudou

	var prev []grid.Frac
	for _, tl := range out {
		prev = append(prev, tl.frac)
	}
	// Se uma janela entrou ou saiu (e não havia dock pra reencaixar), o grid
	// se refaz a partir do tamanho que cada uma tinha, mantendo os
	// redimensionamentos.
	ws.FitKeep(prev)
	for i, tl := range out {
		tl.frac = ws.Cell(i)
	}
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

// overflows diz se, com as proporções atuais (já posicionadas), alguma
// janela do grid passou a ficar maior que o seu slot (tamanho mínimo do
// app) mais do que ficaria com as proporções before — ou seja, se a mudança
// fez uma janela cobrir a vizinha. Uma janela que já não cabia antes (ex.:
// muitas empilhadas numa coluna baixa) não impede mexer no resto.
func (ws *workspace) overflows(mon win.Monitor, g grid.Gaps, before grid.Sizes) bool {
	slots := ws.slots(mon)
	now := ws.Snapshot()
	wantNow := make([]grid.Rect, len(slots))
	for i := range slots {
		wantNow[i] = ws.slotRect(mon, slots, i, g)
	}
	ws.Restore(before)
	wantBefore := make([]grid.Rect, len(slots))
	for i := range slots {
		wantBefore[i] = ws.slotRect(mon, slots, i, g)
	}
	ws.Restore(now)

	for i, tl := range slots {
		if tl.floating || win.IsHung(tl.hwnd) || (!tl.tall && ws.coveredByTall(slots, i)) {
			continue // flutuantes e vizinhas de um tile em altura total ficam atrás de propósito
		}
		got, ok := win.FrameBounds(tl.hwnd)
		if !ok {
			continue
		}
		if overflowGrew(got, wantNow[i], wantBefore[i]) {
			return true
		}
	}
	return false
}

// overflowGrew diz se a janela, com tamanho got, ultrapassa o slot now mais
// do que ultrapassaria o slot before (em largura ou altura). Se ela
// ultrapassa now, got é o tamanho mínimo dela — e com before ela ficaria
// com max(got, before), então o excesso lá seria got - before.
func overflowGrew(got, now, before grid.Rect) bool {
	const tolerance = 2
	excess := func(got, want int32) int32 { return max(0, got-want) }
	return excess(got.W(), now.W()) > excess(got.W(), before.W())+tolerance ||
		excess(got.H(), now.H()) > excess(got.H(), before.H())+tolerance
}
