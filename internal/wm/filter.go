//go:build windows

package wm

import (
	"strings"

	"minwinm/internal/win"
)

// shouldManage ignora desktop, barra de tarefas e menu Iniciar.
func shouldManage(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	switch win.ClassName(hwnd) {
	case "Shell_TrayWnd", "Shell_SecondaryTrayWnd", "Progman", "WorkerW",
		"Windows.UI.Core.CoreWindow":
		return false
	}
	return true
}

// isAppWindow aproxima o critério do Alt+Tab: janela de topo visível, sem
// dono, com título e que não seja de ferramenta. Define os candidatos a foco.
func isAppWindow(hwnd uintptr) bool {
	// Janelas-filhas primeiro: são a maioria dos eventos e o teste é o mais barato.
	if hwnd == 0 || win.Style(hwnd)&win.StyleChild != 0 {
		return false
	}
	if !shouldManage(hwnd) || !win.IsVisible(hwnd) || win.IsIconic(hwnd) || win.IsCloaked(hwnd) {
		return false
	}
	if win.HasOwner(hwnd) {
		return false
	}
	if win.ExStyle(hwnd)&(win.ExToolWindow|win.ExNoActivate) != 0 {
		return false
	}
	return win.HasTitle(hwnd)
}

// tileable decide se uma janela nova entra no grid: janela de app,
// redimensionável e fora da lista ignore.
func (m *Manager) tileable(hwnd uintptr) bool {
	if !isAppWindow(hwnd) || win.Style(hwnd)&win.StyleThickFrame == 0 {
		return false
	}
	if len(m.ignore) > 0 &&
		(m.ignore[strings.ToLower(win.ClassName(hwnd))] || m.ignore[strings.ToLower(win.ProcessName(hwnd))]) {
		return false
	}
	return true
}

// occupies diz se o tile ocupa um slot do grid agora: minimizadas, em outra
// área de trabalho virtual ou em tela cheia guardam o lugar sem ocupá-lo.
func occupies(tl *tile, mon win.Monitor) bool {
	if win.IsIconic(tl.hwnd) || win.IsCloaked(tl.hwnd) {
		return false
	}
	// Tela cheia (jogos, F11): deixa quieta e fora do grid.
	if r, ok := win.WindowRect(tl.hwnd); ok && !win.IsZoomed(tl.hwnd) &&
		r.Left <= mon.Full.Left && r.Top <= mon.Full.Top &&
		r.Right >= mon.Full.Right && r.Bottom >= mon.Full.Bottom {
		return false
	}
	return true
}
