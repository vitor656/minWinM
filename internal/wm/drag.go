//go:build windows

package wm

// Redimensionamento com o mouse: enquanto o usuário arrasta a borda de uma
// janela do grid, as divisas do grid seguem a borda e as vizinhas se ajustam
// ao vivo; ao soltar, o tamanho final vira a nova proporção do slot.

import (
	"time"

	"minwinm/internal/grid"
	"minwinm/internal/win"
)

// dragState guarda o arrasto em andamento (hwnd 0 = nenhum).
type dragState struct {
	hwnd   uintptr
	ws     *workspace
	slot   int
	start  grid.Rect  // retângulo visível no início do arrasto (= slot)
	before grid.Sizes // proporções no início
	shape  []int      // formato do grid no início; se mudar, o arrasto é abandonado
	hook   uintptr    // hook de EVENT_OBJECT_LOCATIONCHANGE só da thread da janela
	last   time.Time  // último reposicionamento das vizinhas (ver dragInterval)
}

// dragInterval limita o reposicionamento das vizinhas durante o arrasto a
// ~60 vezes por segundo: o Windows manda um evento por pixel movido, e cada
// um faria todas as vizinhas repintarem. O tamanho final vem do dragEnd.
const dragInterval = 16 * time.Millisecond

// dragStart começa a acompanhar um arrasto de uma janela do grid.
func (m *Manager) dragStart(hwnd uintptr) {
	m.dragStop()
	ws, i := m.find(hwnd)
	if ws == nil || ws.tiles[i].floating {
		return
	}
	mon, ok := win.MonitorInfo(ws.mon)
	if !ok {
		return
	}
	start, ok := win.FrameBounds(hwnd)
	if !ok {
		return
	}
	if s := slotOf(ws.slots(mon), ws.tiles[i]); s >= 0 {
		m.drag = dragState{hwnd: hwnd, ws: ws, slot: s, start: start,
			before: ws.Snapshot(), shape: ws.Shape(), hook: win.HookLocationChanges(hwnd)}
	}
}

func (m *Manager) dragStop() {
	if m.drag.hook != 0 {
		win.Unhook(m.drag.hook)
	}
	m.drag = dragState{}
}

// resized diz se o arrasto mudou o tamanho (borda) e não só a posição.
func (d *dragState) resized(got grid.Rect) bool {
	return abs32(got.W()-d.start.W()) > 2 || abs32(got.H()-d.start.H()) > 2
}

// sameGrid: o grid não mudou de formato desde o início do arrasto (uma
// janela abriu ou fechou) e o slot ainda é da janela arrastada.
func (d *dragState) sameGrid(slots []*tile) bool {
	return d.ws.SameShape(d.shape) && d.slot < len(slots) && slots[d.slot].hwnd == d.hwnd
}

// dragMove roda a cada movimento durante o arrasto: recalcula as proporções
// e reposiciona as outras janelas (a arrastada é do usuário até soltar).
func (m *Manager) dragMove() {
	d := &m.drag
	if time.Since(d.last) < dragInterval {
		return
	}
	got, ok := win.FrameBounds(d.hwnd)
	if !ok || !d.resized(got) {
		return
	}
	mon, ok := win.MonitorInfo(d.ws.mon)
	if !ok {
		return
	}
	slots := d.ws.slots(mon)
	if !d.sameGrid(slots) {
		return
	}
	d.ws.Restore(d.before)
	d.ws.FollowEdges(mon.Work, m.gaps, d.slot, slots[d.slot].tall, d.start, got)
	m.layout(d.ws)
	d.last = time.Now()
}

// dragEnd trata o fim do arrasto. Devolve true se foi redimensionamento
// (já tratado); false se foi só mover a janela.
func (m *Manager) dragEnd(hwnd uintptr) bool {
	d := m.drag
	m.dragStop()
	if d.hwnd != hwnd {
		return false
	}
	got, ok := win.FrameBounds(hwnd)
	if !ok || !d.resized(got) {
		return false
	}
	mon, ok := win.MonitorInfo(d.ws.mon)
	if !ok {
		return true
	}
	slots := d.ws.slots(mon)
	// Grid mudou durante o arrasto, ou o Aero Snap maximizou a janela ao
	// encostar no topo: não é um ajuste de borda, só volta ao slot.
	if !d.sameGrid(slots) || win.IsZoomed(hwnd) {
		d.ws.Restore(d.before)
		m.layout(d.ws)
		return true
	}
	d.ws.Restore(d.before)
	d.ws.FollowEdges(mon.Work, m.gaps, d.slot, slots[d.slot].tall, d.start, got)
	m.applySizes(d.ws, mon, d.before, d.ws.Snapshot())
	return true
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
