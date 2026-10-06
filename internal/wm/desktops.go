//go:build windows

package wm

import "minwinm/internal/win"

// Áreas de trabalho virtuais: ir para uma área e mandar a janela focada para
// outra. Cada área tem seu próprio grid por monitor (ver Manager.spaces), então
// trocar de área não mexe nas proporções guardadas das outras.

// desktopTarget escolhe a área de destino: n ≥ 1 é a área n (1 = primeira
// da Visão de Tarefas); n == 0 é a vizinha da atual na direção step (±1),
// dando a volta nas pontas.
func (m *Manager) desktopTarget(n, step int) (win.DesktopID, bool) {
	list := m.vd.List()
	if n > len(list) {
		m.logf("área de trabalho %d não existe (há %d); crie mais com Win+Ctrl+D", n, len(list))
	}
	return pickDesktop(list, m.desk, n, step)
}

// pickDesktop é a regra de desktopTarget, sem acesso ao Windows.
func pickDesktop(list []win.DesktopID, cur win.DesktopID, n, step int) (win.DesktopID, bool) {
	if n > 0 {
		if n > len(list) {
			return win.DesktopID{}, false
		}
		return list[n-1], true
	}
	if len(list) < 2 {
		return win.DesktopID{}, false
	}
	i := 0
	for j, id := range list {
		if id == cur {
			i = j
		}
	}
	return list[(i+step+len(list))%len(list)], true
}

// gotoDesktop vai para a área n (ou a vizinha, com n == 0; ver desktopTarget).
func (m *Manager) gotoDesktop(n, step int) {
	m.refreshDesktop()
	id, ok := m.desktopTarget(n, step)
	if !ok || id == m.desk {
		return
	}
	if err := m.vd.Switch(id); err != nil {
		m.logf("áreas de trabalho: %v", err)
		return
	}
	m.desk = id
	// A troca pela API interna não muda o foco: o teclado continuaria na
	// janela da área anterior, agora oculta.
	m.focusTop()
}

// sendToDesktop manda a janela para a área n (ou a vizinha, com n == 0) e
// continua na área atual. No grid, a janela sai do grid daqui e entra no
// fim do grid do mesmo monitor na área de destino.
func (m *Manager) sendToDesktop(hwnd uintptr, n, step int) {
	m.refreshDesktop()
	id, ok := m.desktopTarget(n, step)
	if !ok || id == m.desktopOf(hwnd) {
		return
	}
	if err := m.vd.MoveWindow(hwnd, id); err != nil {
		m.logf("áreas de trabalho: %v", err)
		return
	}
	if ws, i := m.find(hwnd); ws != nil {
		tl := ws.tiles[i]
		ws.tiles = append(ws.tiles[:i], ws.tiles[i+1:]...)
		target := m.spaceFor(ws.mon, id)
		target.tiles = append(target.tiles, tl)
		m.layout(ws)
	}
	m.focusTop()
}

// focusTop foca a janela mais ao topo da área atual (se houver).
func (m *Manager) focusTop() {
	if hs, _ := appWindows(0); len(hs) > 0 {
		win.Focus(hs[0])
	}
}
