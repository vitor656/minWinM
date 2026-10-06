//go:build windows

package win

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	gdi32 = windows.NewLazySystemDLL("gdi32.dll")

	pCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	pCreateBitmap       = gdi32.NewProc("CreateBitmap")
	pDeleteObject       = gdi32.NewProc("DeleteObject")
	pCreateIconIndirect = user32.NewProc("CreateIconIndirect")
	pDestroyIcon        = user32.NewProc("DestroyIcon")
	pGetSystemMetrics   = user32.NewProc("GetSystemMetrics")
)

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type iconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  uintptr
	HbmColor uintptr
}

// SmallIconSize é o tamanho, em pixels, de um ícone da área de notificação
// (SM_CXSMICON, já considerando a escala do monitor principal).
func SmallIconSize() int {
	const smCxSmIcon = 49
	n, _, _ := pGetSystemMetrics.Call(smCxSmIcon)
	if n == 0 {
		return 16
	}
	return int(n)
}

// NewIcon cria um HICON de size×size a partir de pixels 0xAARRGGBB (linha a
// linha, de cima para baixo). Liberar com DestroyIcon.
func NewIcon(size int, pixels []uint32) (uintptr, error) {
	if len(pixels) != size*size {
		return 0, errors.New("tamanho dos pixels não confere")
	}
	bi := bitmapInfoHeader{
		Size:     uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:    int32(size),
		Height:   -int32(size), // negativo = de cima para baixo
		Planes:   1,
		BitCount: 32,
	}
	var bits unsafe.Pointer
	color, _, _ := pCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if color == 0 || bits == nil {
		return 0, errors.New("CreateDIBSection falhou")
	}
	defer pDeleteObject.Call(color)
	copy(unsafe.Slice((*uint32)(bits), size*size), pixels)

	// Máscara monocromática zerada: com canal alfa, a transparência vem dele.
	mask := make([]byte, ((size+15)/16)*2*size)
	hmask, _, _ := pCreateBitmap.Call(uintptr(size), uintptr(size), 1, 1, uintptr(unsafe.Pointer(&mask[0])))
	if hmask == 0 {
		return 0, errors.New("CreateBitmap falhou")
	}
	defer pDeleteObject.Call(hmask)

	ii := iconInfo{FIcon: 1, HbmMask: hmask, HbmColor: color}
	h, _, err := pCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))
	if h == 0 {
		return 0, err
	}
	return h, nil
}

func DestroyIcon(h uintptr) {
	if h != 0 {
		pDestroyIcon.Call(h)
	}
}
