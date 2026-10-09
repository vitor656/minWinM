//go:build windows

package win

import (
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"minwinm/internal/grid"
)

// Bits de GWL_STYLE / GWL_EXSTYLE consultados pelo wm.
const (
	StyleChild      = 0x40000000 // WS_CHILD
	StyleThickFrame = 0x00040000 // WS_THICKFRAME (redimensionável)
	ExToolWindow    = 0x00000080 // WS_EX_TOOLWINDOW
	ExNoActivate    = 0x08000000 // WS_EX_NOACTIVATE
)

func ForegroundWindow() uintptr {
	h, _, _ := pGetForegroundWindow.Call()
	return h
}

func ClassName(hwnd uintptr) string {
	buf := make([]uint16, 256)
	n, _, _ := pGetClassName.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf[:n])
}

// ProcessName devolve o nome do executável dono da janela (ex.: "Taskmgr.exe").
func ProcessName(hwnd uintptr) string {
	var pid uint32
	windows.GetWindowThreadProcessId(windows.HWND(hwnd), &pid)
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_PATH)
	n := uint32(len(buf))
	if windows.QueryFullProcessImageName(h, 0, &buf[0], &n) != nil {
		return ""
	}
	return filepath.Base(windows.UTF16ToString(buf[:n]))
}

func callBool(p *windows.LazyProc, hwnd uintptr) bool {
	r, _, _ := p.Call(hwnd)
	return r != 0
}

func IsWindow(hwnd uintptr) bool  { return callBool(pIsWindow, hwnd) }
func IsVisible(hwnd uintptr) bool { return callBool(pIsWindowVisible, hwnd) }
func IsIconic(hwnd uintptr) bool  { return callBool(pIsIconic, hwnd) }
func IsZoomed(hwnd uintptr) bool  { return callBool(pIsZoomed, hwnd) }
func IsHung(hwnd uintptr) bool    { return callBool(pIsHungAppWindow, hwnd) }

// IsCloaked: janelas em outra área de trabalho virtual ou apps UWP suspensos.
func IsCloaked(hwnd uintptr) bool {
	var c uint32
	hr, _, _ := pDwmGetWindowAttribute.Call(hwnd, dwmwaCloaked,
		uintptr(unsafe.Pointer(&c)), unsafe.Sizeof(c))
	return hr == 0 && c != 0
}

func windowLong(hwnd uintptr, index int32) uint32 {
	r, _, _ := pGetWindowLong.Call(hwnd, uintptr(index))
	return uint32(r)
}

// Style e ExStyle devolvem GWL_STYLE e GWL_EXSTYLE (ver constantes Style*/Ex*).
func Style(hwnd uintptr) uint32   { return windowLong(hwnd, gwlStyle) }
func ExStyle(hwnd uintptr) uint32 { return windowLong(hwnd, gwlExStyle) }

// HasOwner diz se a janela tem uma janela dona (diálogos, por exemplo).
func HasOwner(hwnd uintptr) bool {
	owner, _, _ := pGetWindow.Call(hwnd, gwOwner)
	return owner != 0
}

// HasTitle diz se a janela tem texto de título.
func HasTitle(hwnd uintptr) bool {
	n, _, _ := pGetWindowTextLength.Call(hwnd)
	return n > 0
}

// O callback é criado uma vez só: syscall.NewCallback nunca é liberado.
var enumWinAcc []uintptr
var enumWinCb = syscall.NewCallback(func(h, lparam uintptr) uintptr {
	enumWinAcc = append(enumWinAcc, h)
	return 1
})

// TopLevelWindows devolve as janelas de topo em ordem Z (a de cima primeiro).
func TopLevelWindows() []uintptr {
	enumWinAcc = nil
	pEnumWindows.Call(enumWinCb, 0)
	return enumWinAcc
}

func Minimize(hwnd uintptr) { pShowWindow.Call(hwnd, swMinimize) }

// Focus traz hwnd para o primeiro plano. O processo que recebe o hotkey
// normalmente pode fazer isso; se o Windows recusar, anexa a fila de
// entrada à da janela atual em foreground e tenta de novo.
func Focus(hwnd uintptr) {
	pSetForegroundWindow.Call(hwnd)
	fg := ForegroundWindow()
	if fg == hwnd {
		return
	}
	fgThread, _ := windows.GetWindowThreadProcessId(windows.HWND(fg), nil)
	me := windows.GetCurrentThreadId()
	pAttachThreadInput.Call(uintptr(me), uintptr(fgThread), 1)
	pSetForegroundWindow.Call(hwnd)
	pBringWindowToTop.Call(hwnd)
	pAttachThreadInput.Call(uintptr(me), uintptr(fgThread), 0)
}

// WindowRect devolve o retângulo da janela, incluindo a borda invisível.
func WindowRect(hwnd uintptr) (grid.Rect, bool) {
	var r grid.Rect
	ok, _, _ := pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return r, ok != 0
}

// FrameBounds devolve o retângulo visível (sem a sombra invisível do Win10/11).
func FrameBounds(hwnd uintptr) (grid.Rect, bool) {
	var r grid.Rect
	hr, _, _ := pDwmGetWindowAttribute.Call(hwnd, dwmwaExtendedFrameBounds,
		uintptr(unsafe.Pointer(&r)), unsafe.Sizeof(r))
	return r, hr == 0
}

// Place posiciona a parte VISÍVEL da janela em target, compensando as bordas
// invisíveis. Restaura a janela se estiver maximizada e não faz nada se ela
// já está no lugar.
func Place(hwnd uintptr, target grid.Rect) {
	if IsZoomed(hwnd) {
		pShowWindow.Call(hwnd, swRestore)
	}
	wr, ok := WindowRect(hwnd)
	if !ok {
		return
	}
	fr, ok := FrameBounds(hwnd)
	if !ok {
		fr = wr
	}
	if fr == target {
		return
	}
	l, t := fr.Left-wr.Left, fr.Top-wr.Top
	r, b := wr.Right-fr.Right, wr.Bottom-fr.Bottom

	x, y := target.Left-l, target.Top-t
	w, h := target.W()+l+r, target.H()+t+b

	pSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		swpNoZOrder|swpNoActivate)
	// Ao cruzar monitores com DPI diferente, o Windows reescala a janela
	// depois da primeira chamada; só então é preciso repetir.
	got, ok := FrameBounds(hwnd)
	if !ok {
		got, ok = WindowRect(hwnd)
	}
	if ok && got != target {
		pSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(w), uintptr(h),
			swpNoZOrder|swpNoActivate)
	}
}
