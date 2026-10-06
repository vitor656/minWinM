//go:build windows

package wm

import (
	"slices"

	"minwinm/internal/win"
)

// Áreas de trabalho virtuais: ir para uma área, mandar a janela focada para
// outra, criar e excluir áreas. Cada área tem seu próprio grid por monitor (ver Manager.spaces), então
// trocar de área não mexe nas proporções guardadas das outras.

// desktopTarget escolhe a área de destino: n ≥ 1 é a área n (1 = primeira
// da Visão de Tarefas); n == 0 é a vizinha da atual na direção step (±1),
// dando a volta nas pontas.
func (m *Manager) desktopTarget(n, step int) (win.DesktopID, bool) {
	list := m.vd.List()
	if n > len(list) {
		m.logf("área de trabalho %d não existe (há %d); crie mais com desktop-create ou Win+Ctrl+D", n, len(list))
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

// createDesktop cria uma área de trabalho no fim da lista e vai para ela,
// como o Win+Ctrl+D.
func (m *Manager) createDesktop() {
	id, err := m.vd.Create()
	if err != nil {
		m.logf("áreas de trabalho: %v", err)
		return
	}
	if err := m.vd.Switch(id); err != nil {
		m.logf("áreas de trabalho: %v", err)
		return
	}
	m.desk = id
	m.focusTop()
}

// deleteDesktop exclui a área atual, como o Win+Ctrl+F4: vai para a vizinha
// (ver fallbackDesktop) e as janelas da área excluída entram no fim do grid
// do mesmo monitor de lá, na mesma ordem.
func (m *Manager) deleteDesktop() {
	m.refreshDesktop()
	gone := m.desk
	to, ok := fallbackDesktop(m.vd.List(), gone)
	if !ok {
		m.logf("áreas de trabalho: não dá para excluir a única área")
		return
	}
	// Troca antes de excluir para a área atual ser sempre conhecida.
	if err := m.vd.Switch(to); err != nil {
		m.logf("áreas de trabalho: %v", err)
		return
	}
	m.desk = to
	if err := m.vd.Remove(gone, to); err != nil {
		m.logf("áreas de trabalho: %v", err)
		m.focusTop()
		return
	}
	var orphans []*workspace
	m.spaces = slices.DeleteFunc(m.spaces, func(ws *workspace) bool {
		if ws.desk == gone {
			orphans = append(orphans, ws)
			return true
		}
		return false
	})
	for _, ws := range orphans {
		target := m.spaceFor(ws.mon, to)
		target.tiles = append(target.tiles, ws.tiles...)
	}
	m.layoutAll()
	m.focusTop()
}

// fallbackDesktop escolhe para onde ir ao excluir a área cur: a anterior,
// ou a seguinte se cur é a primeira (mesma regra do Windows). Falha se só
// existe uma área.
func fallbackDesktop(list []win.DesktopID, cur win.DesktopID) (win.DesktopID, bool) {
	if len(list) < 2 {
		return win.DesktopID{}, false
	}
	for i, id := range list {
		if id == cur {
			if i == 0 {
				return list[1], true
			}
			return list[i-1], true
		}
	}
	return win.DesktopID{}, false
}

// focusTop foca a janela mais ao topo da área atual (se houver).
func (m *Manager) focusTop() {
	if hs, _ := appWindows(0); len(hs) > 0 {
		win.Focus(hs[0])
	}
}
