//go:build windows

package wm

import (
	"minwinm/internal/grid"
	"minwinm/internal/win"
)

// --- grid ----------------------------------------------------------------

// toggle liga/desliga o tiling automático. Ao religar, recomeça do arranjo
// atual da tela.
func (m *Manager) toggle() {
	m.enabled = !m.enabled
	// Desligado, OnEvent não vê o fim de um arrasto em andamento.
	m.dragStop()
	if m.enabled {
		m.spaces = nil
		m.retile()
	}
	m.logf("tiling: %s", onOff(m.enabled))
}

// changeGap ajusta o espaço entre janelas em tempo de execução.
func (m *Manager) changeGap(delta int32) {
	m.gaps.Inner = max(0, m.gaps.Inner+delta)
	if m.outerFollows {
		m.gaps.Outer = m.gaps.Inner
	}
	m.logf("gap: %d (bordas %d)", m.gaps.Inner, m.gaps.Outer)
	if m.enabled {
		m.layoutAll()
	}
}

// --- foco e movimento ----------------------------------------------------

// appWindows devolve as janelas de app (menos except) e seus retângulos
// visíveis, em ordem Z.
func appWindows(except uintptr) ([]uintptr, []grid.Rect) {
	var hs []uintptr
	var rects []grid.Rect
	for _, h := range win.TopLevelWindows() {
		if h == except || !isAppWindow(h) {
			continue
		}
		if r, ok := win.FrameBounds(h); ok {
			hs = append(hs, h)
			rects = append(rects, r)
		}
	}
	return hs, rects
}

// focus move o foco para a janela vizinha na direção d (em qualquer monitor).
func (m *Manager) focus(d grid.Dir) {
	cur := win.ForegroundWindow()
	cands, rects := appWindows(cur)
	if len(cands) == 0 {
		return
	}
	from, ok := win.FrameBounds(cur)
	if !ok || !isAppWindow(cur) {
		win.Focus(cands[0]) // nada focado: pega a janela mais ao topo
		return
	}
	// Janela centralizada: a direção vale a partir do lugar dela no grid
	// (do centro da tela, "esquerda/direita" seria arbitrário), e o centro
	// passa para a vizinha — ela volta ao lugar e a nova vai para o centro.
	home, centered := m.centeredHome(cur)
	if centered {
		from = home
	}
	i := grid.Neighbor(from, rects, d)
	if i < 0 {
		return
	}
	if centered {
		m.preset(cur, "center")      // repetir o preset devolve ao lugar
		m.preset(cands[i], "center") // e centraliza a vizinha
	}
	win.Focus(cands[i])
}

// centeredHome diz se hwnd está centralizada (preset "center") e devolve o
// retângulo para onde ela volta: o slot do grid ou a posição anterior.
func (m *Manager) centeredHome(hwnd uintptr) (grid.Rect, bool) {
	ws, i := m.find(hwnd)
	if !m.enabled || ws == nil {
		s, ok := m.saved[hwnd]
		return s.r, ok && s.preset == "center"
	}
	tl := ws.tiles[i]
	if !tl.floating || tl.preset != "center" {
		return grid.Rect{}, false
	}
	mon, ok := win.MonitorInfo(ws.mon)
	if !ok {
		return grid.Rect{}, false
	}
	slots := ws.slots(mon)
	if s := slotOf(slots, tl); s >= 0 {
		return ws.slotRect(mon, slots, s, m.gaps), true
	}
	return grid.Rect{}, false
}

// move troca a janela de lugar com a vizinha na direção d; na borda do
// monitor (sem vizinha naquela direção), leva a janela para o monitor ao
// lado — no grid dele, entrando pelo lado de onde veio. Uma janela
// flutuante (center, maximize, top, bottom) não troca de lugar com
// ninguém: vai direto para o monitor vizinho, no mesmo preset. Com o tiling
// desligado, ou numa janela fora do grid, ver moveFree.
func (m *Manager) move(hwnd uintptr, d grid.Dir) {
	ws, i := m.find(hwnd)
	if !m.enabled || ws == nil {
		m.moveFree(hwnd, d)
		return
	}
	tl := ws.tiles[i]
	from, ok := win.FrameBounds(hwnd)
	if !ok {
		return
	}
	if !tl.floating {
		idx, rects := placedTiles(ws, tl)
		if k := grid.Neighbor(from, rects, d); k >= 0 {
			j := idx[k]
			ws.tiles[i], ws.tiles[j] = ws.tiles[j], ws.tiles[i]
			m.layout(ws)
			return
		}
	}
	next, ok := neighborMonitor(ws.mon, d)
	if !ok {
		return
	}
	target := m.spaceFor(next.Handle, ws.desk)
	idx, rects := placedTiles(target, nil)
	at := len(target.tiles)
	if k := grid.EntryIndex(from, rects, d); k < len(idx) {
		at = idx[k]
	}
	m.transfer(ws, i, target, at)
}

// placedTiles devolve as posições (em ws.tiles) e os retângulos visíveis
// dos tiles posicionados no grid agora — sem flutuantes, minimizados,
// ocultos e except —, na ordem do grid.
func placedTiles(ws *workspace, except *tile) ([]int, []grid.Rect) {
	var idx []int
	var rects []grid.Rect
	for j, tl := range ws.tiles {
		if tl == except || tl.floating || win.IsIconic(tl.hwnd) || win.IsCloaked(tl.hwnd) {
			continue
		}
		if r, ok := win.FrameBounds(tl.hwnd); ok {
			idx = append(idx, j)
			rects = append(rects, r)
		}
	}
	return idx, rects
}

// neighborMonitor devolve o monitor vizinho de cur na direção d.
func neighborMonitor(cur uintptr, d grid.Dir) (win.Monitor, bool) {
	from, ok := win.MonitorInfo(cur)
	if !ok {
		return win.Monitor{}, false
	}
	var mons []win.Monitor
	var rects []grid.Rect
	for _, mon := range win.Monitors() {
		if mon.Handle != cur {
			mons = append(mons, mon)
			rects = append(rects, mon.Full)
		}
	}
	if k := grid.Neighbor(from.Full, rects, d); k >= 0 {
		return mons[k], true
	}
	return win.Monitor{}, false
}

// transfer move o tile i de ws para o grid target, na posição at da lista
// (len(target.tiles) = no fim). O que era do lugar antigo fica para trás: o
// encaixe customizado (preset dockado ou solo-column) de que ela era a
// âncora e a altura total, que no outro monitor cobriria janelas que não
// estavam empilhadas com ela. Uma janela flutuante continua no mesmo
// preset, agora no monitor novo.
func (m *Manager) transfer(ws *workspace, i int, target *workspace, at int) {
	if target == nil || target == ws {
		return
	}
	tl := ws.tiles[i]
	ws.tiles = append(ws.tiles[:i], ws.tiles[i+1:]...)
	ws.release(tl)
	tl.tall = false
	tl.frac = grid.Frac{} // o tamanho era do outro monitor: entra como janela nova
	at = min(max(at, 0), len(target.tiles))
	target.tiles = append(target.tiles[:at], append([]*tile{tl}, target.tiles[at:]...)...)
	m.logf("mudou de monitor: %s (%#x → %#x)", tl.name(), ws.mon, target.mon)
	m.layout(ws)
	m.layout(target)
	if tl.floating {
		if mon, ok := win.MonitorInfo(target.mon); ok {
			m.placePreset(tl.hwnd, tl.preset, mon)
		}
	}
}

// moveToMonitor implementa next/prev-monitor: com o tiling ligado leva a
// janela para o fim do grid do próximo/anterior monitor (da esquerda para a
// direita, dando a volta); desligado, mantém posição e tamanho relativos.
func (m *Manager) moveToMonitor(hwnd uintptr, step int) {
	mons := win.Monitors()
	if len(mons) < 2 {
		return
	}
	cur := 0
	if mon, ok := win.MonitorOf(hwnd); ok {
		for j, o := range mons {
			if o.Handle == mon.Handle {
				cur = j
			}
		}
	}
	if ws, i := m.find(hwnd); m.enabled && ws != nil {
		for j, o := range mons {
			if o.Handle == ws.mon {
				cur = j
			}
		}
		target := m.spaceFor(mons[(cur+step+len(mons))%len(mons)].Handle, ws.desk)
		m.transfer(ws, i, target, len(target.tiles))
		return
	}
	m.moveKeepingPosition(hwnd, mons[(cur+step+len(mons))%len(mons)])
}

// moveFree é o move-* com o tiling desligado ou numa janela fora do grid.
// Desligado, troca a janela de lugar com a vizinha do mesmo monitor; sem
// vizinha (ou fora do grid, com o tiling ligado — não troca com uma janela
// do grid), leva a janela para o monitor naquela direção, mantendo posição
// e tamanho relativos.
func (m *Manager) moveFree(hwnd uintptr, d grid.Dir) {
	from, ok := win.FrameBounds(hwnd)
	if !ok {
		return
	}
	cur, ok := win.MonitorOf(hwnd)
	if !ok {
		return
	}
	if !m.enabled {
		var hs []uintptr
		var rects []grid.Rect
		cands, all := appWindows(hwnd)
		for k, h := range cands {
			if mon, ok := win.MonitorOf(h); ok && mon.Handle == cur.Handle {
				hs = append(hs, h)
				rects = append(rects, all[k])
			}
		}
		if k := grid.Neighbor(from, rects, d); k >= 0 {
			win.Place(hwnd, rects[k])
			win.Place(hs[k], from)
			return
		}
	}
	if next, ok := neighborMonitor(cur.Handle, d); ok {
		m.moveKeepingPosition(hwnd, next)
	}
}

// moveKeepingPosition leva a janela para o monitor next mantendo a posição e
// o tamanho relativos à área útil. Uma janela num preset do modo manual vai
// para o mesmo preset no outro monitor, e o lugar para onde ela volta ao
// desfazer o preset também passa para lá.
func (m *Manager) moveKeepingPosition(hwnd uintptr, next win.Monitor) {
	cur, ok := win.MonitorOf(hwnd)
	if !ok || cur.Handle == next.Handle {
		return
	}
	if s, ok := m.saved[hwnd]; ok {
		s.r = grid.TileRect(next.Work, grid.RelativeFrac(s.r, cur.Work), grid.Gaps{})
		m.saved[hwnd] = s
		m.placePreset(hwnd, s.preset, next)
		return
	}
	vis, ok := win.FrameBounds(hwnd)
	if !ok {
		if vis, ok = win.WindowRect(hwnd); !ok {
			return
		}
	}
	f := grid.RelativeFrac(vis, cur.Work)
	placeInside(hwnd, grid.TileRect(next.Work, f, grid.Gaps{}), next.Work)
}

// placeInside posiciona a janela em r e, se ela não encolher até lá
// (tamanho mínimo do app) e passar da área útil do monitor, desliza de volta
// para dentro dela: pode cobrir a vizinha, mas não atravessa para o monitor
// ao lado nem fica atrás da barra de tarefas.
func placeInside(hwnd uintptr, r, work grid.Rect) {
	win.Place(hwnd, r)
	got, ok := win.FrameBounds(hwnd)
	if !ok {
		return
	}
	if in := grid.Inside(got, work); in != got {
		win.Place(hwnd, in)
	}
}

// --- tamanho -------------------------------------------------------------

// resize muda a largura (horizontal) ou a altura da janela em delta,
// tirando o espaço proporcionalmente das vizinhas para o grid continuar
// fechado, sem deixar nenhuma janela transbordar (ver applySizes).
func (m *Manager) resize(hwnd uintptr, horizontal bool, delta float64) {
	ws, i := m.find(hwnd)
	if !m.enabled || ws == nil || ws.tiles[i].floating {
		return
	}
	mon, ok := win.MonitorInfo(ws.mon)
	if !ok {
		return
	}
	slot := slotOf(ws.slots(mon), ws.tiles[i])
	if slot < 0 {
		return
	}
	before := ws.Snapshot()
	ws.Grow(slot, horizontal, delta)
	m.applySizes(ws, mon, before, ws.Snapshot())
}

// balance volta as proporções do grid da janela para iguais. Preserva a
// largura (e as linhas) da coluna de um preset dockado: balance desfaz
// redimensionamento manual, não é pra desfazer o próprio preset.
func (m *Manager) balance(hwnd uintptr) {
	ws, _ := m.find(hwnd)
	if !m.enabled || ws == nil {
		return
	}
	mon, ok := win.MonitorInfo(ws.mon)
	if !ok {
		return
	}
	if c, ok := ws.dockedColumn(ws.slots(mon)); ok {
		ws.ResetExcept(c)
	} else {
		ws.Reset()
	}
	m.layout(ws)
}

// --- janela --------------------------------------------------------------

// toggleFullHeight faz a janela ocupar a coluna inteira; de novo, volta a
// dividir a coluna com as vizinhas.
func (m *Manager) toggleFullHeight(hwnd uintptr) {
	ws, i := m.find(hwnd)
	if !m.enabled || ws == nil || ws.tiles[i].floating {
		return
	}
	tl := ws.tiles[i]
	if tl.tall {
		tl.tall = false
		m.layout(ws)
		return
	}
	mon, ok := win.MonitorInfo(ws.mon)
	if !ok {
		return
	}
	slots := ws.slots(mon)
	s := slotOf(slots, tl)
	if s < 0 {
		return
	}
	if ws.tallBlockedByPin(slots, s) {
		return // a coluna pertence a um preset dockado; não sobrepõe
	}
	for _, sib := range ws.sameColumn(slots, s) {
		sib.tall = false
	}
	tl.tall = true
	m.layout(ws)
}

// toggleSoloColumn deixa a janela sozinha na coluna dela, em altura total,
// e empurra as outras da coluna para a coluna vizinha (o grid se refaz, sem
// janelas atrás de outras). De novo, volta ao arranjo de antes; abrir ou
// fechar uma janela também volta ao formato automático. Disputa o mesmo
// encaixe customizado que um preset dockado (ver preset) — aplicar um
// desfaz o outro.
func (m *Manager) toggleSoloColumn(hwnd uintptr) {
	ws, i := m.find(hwnd)
	if !m.enabled || ws == nil || ws.tiles[i].floating {
		return
	}
	mon, ok := win.MonitorInfo(ws.mon)
	if !ok {
		return
	}
	tl := ws.tiles[i]
	slots := ws.slots(mon)
	if ws.Custom() && ws.pin.tile == tl {
		tl.preset = ""
		ws.unpin(mon)
		m.layout(ws)
		return
	}
	s := slotOf(slots, tl)
	shape, order, ok := grid.SoloColumn(ws.Shape(), s)
	if !ok {
		return
	}
	ws.takeoverPin(tl)
	ws.pin.name = ""
	for _, sib := range ws.sameColumn(slots, s) {
		sib.tall = false // a coluna passa a ser só dela
	}
	ws.reorder(slots, order)
	ws.SetShape(shape)
	m.layout(ws)
}

// toggleCenter centraliza a janela e, na segunda vez, devolve ela para onde
// estava: o slot do grid (tiling ligado) ou a posição anterior (desligado).
// Numa janela que está em qualquer preset, só faz ela voltar.
func (m *Manager) toggleCenter(hwnd uintptr) {
	if ws, i := m.find(hwnd); m.enabled && ws != nil && ws.tiles[i].floating {
		m.preset(hwnd, ws.tiles[i].preset)
		return
	}
	if s, ok := m.saved[hwnd]; ok && !m.enabled {
		m.preset(hwnd, s.preset)
		return
	}
	m.preset(hwnd, "center")
}

// preset aplica um preset fixo. Presets encaixáveis (ver grid.Dock: metade,
// terço, dois-terços, centralizado como center-third, ou quadrante) encaixam
// no grid de verdade: a janela ganha a área pedida e as outras se reajustam
// pro resto da tela, sem sobrepor (disputam o mesmo encaixe que
// toggle-solo-column — aplicar um desfaz o outro). Os demais (center,
// maximize, e top/bottom por limitação do grid) continuam como overlay
// flutuante: a janela ganha uma posição fixa e o slot que tinha fica
// reservado, sem reajustar as outras.
//
// Repetir o mesmo preset desfaz: com tiling, a janela volta ao arranjo de
// antes (automático, ou do encaixe anterior); sem tiling (ou numa janela
// fora do grid), volta para a posição de antes do primeiro preset.
func (m *Manager) preset(hwnd uintptr, name string) {
	if ws, i := m.find(hwnd); m.enabled && ws != nil {
		tl := ws.tiles[i]
		mon, ok := win.MonitorInfo(ws.mon)
		if !ok {
			return
		}

		// Repetir o mesmo encaixe desfaz.
		if ws.Custom() && ws.pin.tile == tl && ws.pin.name == name {
			tl.preset = ""
			ws.unpin(mon)
			m.layout(ws)
			return
		}

		slots := ws.slots(mon)
		s := slotOf(slots, tl)
		if s < 0 {
			return
		}
		f, _ := grid.Preset(name)
		if newShape, colW, rowW, order, ok := grid.Dock(ws.Shape(), s, f); ok {
			for _, sib := range ws.sameColumn(slots, s) {
				sib.tall = false // a coluna do preset passa a ser dele
			}
			tl.floating = false // vinha de um overlay (center/maximize): limpa
			ws.takeoverPin(tl)
			ws.reorder(slots, order)
			ws.SetLayout(newShape, colW, rowW)
			ws.pin.name, tl.preset = name, name
			m.layout(ws)
			return
		}

		// Não encaixável (center/maximize/top/bottom): se a janela estava
		// dockada, desfaz o dock antes de flutuar por cima.
		if ws.Custom() && ws.pin.tile == tl {
			tl.preset = ""
			ws.unpin(mon)
		}
		if tl.floating && tl.preset == name {
			tl.floating, tl.preset = false, ""
			m.layout(ws)
			return
		}
		tl.floating, tl.preset = true, name
		m.placePreset(hwnd, name, mon) // no monitor do grid, não onde a janela está
		return
	}
	for h := range m.saved { // esquece janelas que já fecharam
		if !win.IsWindow(h) {
			delete(m.saved, h)
		}
	}
	s, ok := m.saved[hwnd]
	if ok && s.preset == name {
		delete(m.saved, hwnd)
		win.Place(hwnd, s.r)
		return
	}
	if !ok {
		if s.r, ok = win.FrameBounds(hwnd); !ok {
			return
		}
	}
	mon, ok := win.MonitorOf(hwnd)
	if !ok {
		return
	}
	s.preset = name
	m.saved[hwnd] = s
	m.placePreset(hwnd, name, mon)
}

// placePreset posiciona a janela na área do preset dentro do monitor mon.
func (m *Manager) placePreset(hwnd uintptr, name string, mon win.Monitor) {
	if f, ok := grid.Preset(name); ok {
		placeInside(hwnd, grid.TileRect(mon.Work, f, m.gaps), mon.Work)
	}
}
