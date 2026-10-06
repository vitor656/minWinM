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
// monitor, leva a janela para o grid do monitor vizinho. Com o tiling
// desligado, só troca a posição das duas janelas.
func (m *Manager) move(hwnd uintptr, d grid.Dir) {
	if !m.enabled {
		m.swapManual(hwnd, d)
		return
	}
	ws, i := m.find(hwnd)
	if ws == nil || ws.tiles[i].floating {
		return
	}
	from, ok := win.FrameBounds(hwnd)
	if !ok {
		return
	}
	var idx []int
	var rects []grid.Rect
	for j, tl := range ws.tiles {
		if j == i || tl.floating || win.IsIconic(tl.hwnd) || win.IsCloaked(tl.hwnd) {
			continue
		}
		if r, ok := win.FrameBounds(tl.hwnd); ok {
			idx = append(idx, j)
			rects = append(rects, r)
		}
	}
	if k := grid.Neighbor(from, rects, d); k >= 0 {
		j := idx[k]
		ws.tiles[i], ws.tiles[j] = ws.tiles[j], ws.tiles[i]
		m.layout(ws)
		return
	}

	cur, _ := win.MonitorInfo(ws.mon)
	var mrects []grid.Rect
	var handles []uintptr
	for _, mon := range win.Monitors() {
		if mon.Handle != ws.mon {
			mrects = append(mrects, mon.Full)
			handles = append(handles, mon.Handle)
		}
	}
	if k := grid.Neighbor(cur.Full, mrects, d); k >= 0 {
		// Entra pelo lado de onde veio.
		m.transfer(ws, i, m.spaceFor(handles[k], ws.desk), d == grid.Left || d == grid.Up)
	}
}

// transfer move o tile i de ws para o grid target (no fim ou no começo).
func (m *Manager) transfer(ws *workspace, i int, target *workspace, atEnd bool) {
	if target == nil || target == ws {
		return
	}
	tl := ws.tiles[i]
	ws.tiles = append(ws.tiles[:i], ws.tiles[i+1:]...)
	if atEnd {
		target.tiles = append(target.tiles, tl)
	} else {
		target.tiles = append([]*tile{tl}, target.tiles...)
	}
	m.layout(ws)
	m.layout(target)
}

// moveToMonitor implementa next/prev-monitor: com o tiling ligado leva a
// janela para o grid do monitor vizinho; desligado, mantém posição e
// tamanho relativos.
func (m *Manager) moveToMonitor(hwnd uintptr, step int) {
	if ws, i := m.find(hwnd); m.enabled && ws != nil {
		if spaces := m.current(); len(spaces) >= 2 {
			k := 0
			for j, s := range spaces {
				if s == ws {
					k = j
				}
			}
			m.transfer(ws, i, spaces[(k+step+len(spaces))%len(spaces)], true)
			return
		}
	}
	moveKeepingPosition(hwnd, step)
}

// moveKeepingPosition leva a janela para o monitor vizinho mantendo a
// posição e o tamanho relativos à área útil.
func moveKeepingPosition(hwnd uintptr, step int) {
	mons := win.Monitors()
	if len(mons) < 2 {
		return
	}
	cur, ok := win.MonitorOf(hwnd)
	if !ok {
		return
	}
	idx := 0
	for i, mon := range mons {
		if mon.Handle == cur.Handle {
			idx = i
		}
	}
	next := mons[(idx+step+len(mons))%len(mons)]

	vis, ok := win.FrameBounds(hwnd)
	if !ok {
		if vis, ok = win.WindowRect(hwnd); !ok {
			return
		}
	}
	f := grid.RelativeFrac(vis, cur.Work)
	win.Place(hwnd, grid.TileRect(next.Work, f, grid.Gaps{}))
}

// swapManual, com o tiling desligado, troca a posição de duas janelas.
func (m *Manager) swapManual(hwnd uintptr, d grid.Dir) {
	from, ok := win.FrameBounds(hwnd)
	if !ok {
		return
	}
	cands, rects := appWindows(hwnd)
	if k := grid.Neighbor(from, rects, d); k >= 0 {
		win.Place(hwnd, rects[k])
		win.Place(cands[k], from)
	}
}

// --- tamanho -------------------------------------------------------------

// resize muda a largura (horizontal) ou a altura da janela em delta,
// tirando o espaço proporcionalmente das vizinhas para o grid continuar
// fechado, sem deixar nenhuma janela transbordar (ver applySizes).
func (m *Manager) resize(hwnd uintptr, horizontal bool, delta float64) {
	ws, i := m.find(hwnd)
	if !m.enabled || ws == nil {
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

// balance volta as proporções do grid da janela para iguais.
func (m *Manager) balance(hwnd uintptr) {
	if ws, _ := m.find(hwnd); ws != nil {
		ws.Reset()
		m.layout(ws)
	}
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
	if s := slotOf(slots, tl); s >= 0 {
		for _, sib := range ws.sameColumn(slots, s) {
			sib.tall = false
		}
	}
	tl.tall = true
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

// preset aplica um preset fixo. Repetir o mesmo preset desfaz: com tiling a
// janela volta ao slot do grid (que ficou reservado); sem tiling (ou numa
// janela fora do grid), volta para a posição de antes do primeiro preset.
func (m *Manager) preset(hwnd uintptr, name string) {
	if ws, i := m.find(hwnd); m.enabled && ws != nil {
		tl := ws.tiles[i]
		if tl.floating && tl.preset == name {
			tl.floating, tl.preset = false, ""
			m.layout(ws)
			return
		}
		tl.floating, tl.preset = true, name
		m.placePreset(hwnd, name)
		return
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
	s.preset = name
	m.saved[hwnd] = s
	m.placePreset(hwnd, name)
}

// placePreset posiciona a janela na área do preset, no monitor onde ela está.
func (m *Manager) placePreset(hwnd uintptr, name string) {
	f, ok := grid.Preset(name)
	if !ok {
		return
	}
	mon, ok := win.MonitorOf(hwnd)
	if !ok {
		return
	}
	win.Place(hwnd, grid.TileRect(mon.Work, f, m.gaps))
}
