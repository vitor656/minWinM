//go:build windows

package win

import (
	"sort"
	"syscall"
	"unsafe"

	"minwinm/internal/grid"
)

// Monitor é um monitor conectado.
type Monitor struct {
	Handle uintptr
	Full   grid.Rect
	Work   grid.Rect // área útil (sem a barra de tarefas)
}

// Point é um ponto de tela (mesmo layout do POINT do Win32).
type Point struct{ X, Y int32 }

type monitorInfoRaw struct {
	CbSize  uint32
	Monitor grid.Rect
	Work    grid.Rect
	Flags   uint32
}

// MonitorInfo devolve os dados do monitor pelo handle.
func MonitorInfo(h uintptr) (Monitor, bool) {
	mi := monitorInfoRaw{}
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	ok, _, _ := pGetMonitorInfo.Call(h, uintptr(unsafe.Pointer(&mi)))
	if ok == 0 {
		return Monitor{}, false
	}
	return Monitor{Handle: h, Full: mi.Monitor, Work: mi.Work}, true
}

// MonitorOf devolve o monitor onde a janela está (ou o mais próximo).
func MonitorOf(hwnd uintptr) (Monitor, bool) {
	h, _, _ := pMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	return MonitorInfo(h)
}

// MonitorAt devolve o handle do monitor sob o ponto (ou o mais próximo).
func MonitorAt(p Point) uintptr {
	// POINT é passado por valor: no x64 os dois int32 vão juntos num registrador.
	h, _, _ := pMonitorFromPoint.Call(uintptr(uint32(p.X))|uintptr(uint32(p.Y))<<32, monitorDefaultToNearest)
	return h
}

func CursorPos() Point {
	var p Point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return p
}

// O callback é criado uma vez só: syscall.NewCallback nunca é liberado.
var enumMonAcc []Monitor
var enumMonCb = syscall.NewCallback(func(h, hdc, lprc, lparam uintptr) uintptr {
	if m, ok := MonitorInfo(h); ok {
		enumMonAcc = append(enumMonAcc, m)
	}
	return 1
})

// Monitors devolve os monitores ordenados da esquerda para a direita.
func Monitors() []Monitor {
	enumMonAcc = nil
	pEnumDisplayMonitors.Call(0, 0, enumMonCb, 0)
	out := enumMonAcc
	sort.Slice(out, func(i, j int) bool {
		if out[i].Full.Left != out[j].Full.Left {
			return out[i].Full.Left < out[j].Full.Left
		}
		return out[i].Full.Top < out[j].Full.Top
	})
	return out
}
