//go:build windows

package win

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Mínimo de COM para falar com as interfaces do Explorer: criar objetos e
// chamar métodos pela vtable. Sem dependências além de x/sys.

var (
	ole32             = windows.NewLazySystemDLL("ole32.dll")
	pCoCreateInstance = ole32.NewProc("CoCreateInstance")
)

const (
	clsctxInprocServer = 0x1
	clsctxLocalServer  = 0x4
)

// comObject é a forma em memória de qualquer objeto COM: um ponteiro para a
// vtable. Os objetos vivem fora do heap do Go (são do Explorer/ole32).
type comObject struct {
	vtbl *[64]uintptr
}

func mustGUID(s string) windows.GUID {
	g, err := windows.GUIDFromString(s)
	if err != nil {
		panic(err)
	}
	return g
}

// call chama o método número slot da vtable e devolve o HRESULT. Os slots
// 0–2 são os do IUnknown (QueryInterface, AddRef, Release).
func (o *comObject) call(slot int, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(o.vtbl[slot], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return r
}

func (o *comObject) release() {
	if o != nil {
		o.call(2)
	}
}

// ptr é o objeto como argumento de chamada.
func (o *comObject) ptr() uintptr { return uintptr(unsafe.Pointer(o)) }

func failed(hr uintptr) bool { return int32(hr) < 0 }

// hrErr é uma chamada COM que falhou.
type hrErr struct {
	what string
	hr   uint32
}

func (e hrErr) Error() string { return fmt.Sprintf("%s: HRESULT %#x", e.what, e.hr) }

func hrError(what string, hr uintptr) error { return hrErr{what, uint32(hr)} }

// disconnected diz se o erro significa que o servidor COM (o Explorer) caiu
// ou reiniciou — o único caso em que vale reconectar.
func disconnected(err error) bool {
	e, ok := err.(hrErr)
	if !ok {
		return false
	}
	switch e.hr {
	case 0x80010108, // RPC_E_DISCONNECTED
		0x800706BA, // RPC_S_SERVER_UNAVAILABLE
		0x80010007, // RPC_E_SERVER_DIED
		0x80010012, // RPC_E_SERVER_DIED_DNE
		0x800401FD: // CO_E_OBJNOTCONNECTED
		return true
	}
	return false
}

func coCreate(clsid, iid windows.GUID, ctx uintptr) (*comObject, error) {
	var obj *comObject
	hr, _, _ := pCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsid)), 0, ctx,
		uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&obj)))
	if failed(hr) || obj == nil {
		return nil, hrError("CoCreateInstance", hr)
	}
	return obj, nil
}

// queryService chama IServiceProvider::QueryService (slot 3).
func queryService(sp *comObject, service, iid windows.GUID) (*comObject, error) {
	var obj *comObject
	hr := sp.call(3, uintptr(unsafe.Pointer(&service)), uintptr(unsafe.Pointer(&iid)),
		uintptr(unsafe.Pointer(&obj)))
	if failed(hr) || obj == nil {
		return nil, hrError("QueryService", hr)
	}
	return obj, nil
}
