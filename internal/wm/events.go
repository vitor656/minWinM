//go:build windows

package wm

import "minwinm/internal/win"

// OnEvent recebe os eventos de janela do Windows. A maioria só agenda um
// re-tile (ver retileDelay); foco e arrasto são tratados na hora.
func (m *Manager) OnEvent(event uint32, hwnd uintptr) {
	// Janela fechada no meio do arrasto: o MOVESIZEEND não vem, e o hook de
	// movimentos da thread dela ficaria entregando eventos.
	if event == win.EventDestroy && hwnd != 0 && hwnd == m.drag.hwnd {
		m.dragStop()
	}
	if !m.enabled {
		return
	}
	ws, _ := m.find(hwnd)
	switch event {
	case win.EventForeground:
		m.followFocus(hwnd)
		return
	case win.EventShow, win.EventUncloaked, win.EventMinimizeEnd:
		if ws == nil && !m.tileable(hwnd) {
			return
		}
	case win.EventMoveSizeStart:
		m.dragStart(hwnd)
		return
	case win.EventLocationChange:
		if hwnd == m.drag.hwnd {
			m.dragMove()
		}
		return
	case win.EventMoveSizeEnd:
		if m.dragEnd(hwnd) || ws == nil {
			return
		}
		m.dropped(hwnd)
	default: // destroy, hide, minimize, cloak
		if ws == nil {
			return
		}
	}
	if m.timer == 0 {
		m.timer = win.SetTimer(retileDelay)
	}
}

// OnTimer trata o WM_TIMER do re-tile agendado por OnEvent.
func (m *Manager) OnTimer(id uintptr) {
	if id != m.timer {
		return
	}
	win.KillTimer(id)
	m.timer = 0
	if m.enabled {
		m.retile()
	}
}

// dropped trata uma janela do grid arrastada com o mouse (sem mudar de
// tamanho): soltar sobre outra janela troca as duas de lugar; soltar em
// outro monitor move para o grid dele. Qualquer outro caso volta ao slot no
// próximo re-tile.
func (m *Manager) dropped(hwnd uintptr) {
	ws, i := m.find(hwnd)
	if ws.tiles[i].floating {
		return
	}
	p := win.CursorPos()
	for _, ows := range m.current() { // janelas de outras áreas estão ocultas, mas ainda têm posição
		for j, tl := range ows.tiles {
			if tl.hwnd == hwnd || tl.floating {
				continue
			}
			if r, ok := win.FrameBounds(tl.hwnd); ok && win.IsVisible(tl.hwnd) && !win.IsIconic(tl.hwnd) &&
				r.Contains(p.X, p.Y) {
				ws.tiles[i], ows.tiles[j] = ows.tiles[j], ws.tiles[i]
				return
			}
		}
	}
	if target := m.spaceFor(win.MonitorAt(p), ws.desk); target != ws {
		tl := ws.tiles[i]
		ws.tiles = append(ws.tiles[:i], ws.tiles[i+1:]...)
		target.tiles = append(target.tiles, tl)
	}
}

// followFocus: numa coluna em altura total, a janela que recebe o foco
// passa a ser a que ocupa a coluna inteira (a coluna vira uma pilha).
func (m *Manager) followFocus(hwnd uintptr) {
	ws, i := m.find(hwnd)
	if ws == nil || ws.tiles[i].tall || ws.tiles[i].floating {
		return
	}
	mon, ok := win.MonitorInfo(ws.mon)
	if !ok {
		return
	}
	slots := ws.slots(mon)
	s := slotOf(slots, ws.tiles[i])
	if s < 0 {
		return
	}
	for _, sib := range ws.sameColumn(slots, s) {
		if sib.tall {
			sib.tall = false
			ws.tiles[i].tall = true
			m.layout(ws)
			return
		}
	}
}
