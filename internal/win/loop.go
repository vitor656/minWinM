//go:build windows

package win

import "unsafe"

// Mensagens que o loop principal trata.
const (
	WMHotkey = 0x0312 // WM_HOTKEY: WParam = id do atalho, LParam (palavra baixa) = modificadores
	WMTimer  = 0x0113 // WM_TIMER: WParam = id do timer
)

// Msg é o MSG do Win32.
type Msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      Point
}

// GetMessage espera a próxima mensagem da thread; devolve false quando o
// loop deve terminar.
func GetMessage(m *Msg) bool {
	r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(m)), 0, 0, 0)
	return int32(r) > 0
}

// EnablePerMonitorDPI faz as coordenadas serem pixels reais em todos os
// monitores, mesmo com escalas diferentes.
func EnablePerMonitorDPI() {
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4
	ctx := -4
	pSetProcessDpiAwarenessContext.Call(uintptr(ctx))
}

// RegisterHotKey registra um atalho global para a thread atual (mods são os
// bits MOD_*). repeat permite disparos contínuos com a tecla segurada.
func RegisterHotKey(id int, mods, vk uint32, repeat bool) error {
	if !repeat {
		mods |= modNoRepeat
	}
	r, _, err := pRegisterHotKey.Call(0, uintptr(id), uintptr(mods), uintptr(vk))
	if r == 0 {
		return err
	}
	return nil
}

// SetTimer cria um timer de thread (WM_TIMER chega no GetMessage com hwnd 0).
func SetTimer(ms uint32) uintptr {
	id, _, _ := pSetTimer.Call(0, 0, uintptr(ms), 0)
	return id
}

func KillTimer(id uintptr) { pKillTimer.Call(0, id) }

// MaskModifierRelease evita que soltar Alt/Win depois do atalho ative a barra
// de menu do app em foco (ou o menu Iniciar): o app vê uma tecla "no meio"
// e não trata o Alt como pressionado sozinho. 0xE8 é um VK sem uso.
func MaskModifierRelease() {
	const vkUnassigned, keyUp = 0xE8, 0x0002
	pKeybdEvent.Call(vkUnassigned, 0, 0, 0)
	pKeybdEvent.Call(vkUnassigned, 0, keyUp, 0)
}
