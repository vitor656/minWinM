//go:build windows

package win

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// Eventos de janela (SetWinEventHook) que o wm recebe.
const (
	EventForeground     = 0x0003 // EVENT_SYSTEM_FOREGROUND
	EventMoveSizeStart  = 0x000A // EVENT_SYSTEM_MOVESIZESTART
	EventMoveSizeEnd    = 0x000B // EVENT_SYSTEM_MOVESIZEEND
	EventMinimizeStart  = 0x0016 // EVENT_SYSTEM_MINIMIZESTART
	EventMinimizeEnd    = 0x0017 // EVENT_SYSTEM_MINIMIZEEND
	EventDestroy        = 0x8001 // EVENT_OBJECT_DESTROY
	EventShow           = 0x8002 // EVENT_OBJECT_SHOW
	EventHide           = 0x8003 // EVENT_OBJECT_HIDE
	EventLocationChange = 0x800B // EVENT_OBJECT_LOCATIONCHANGE
	EventCloaked        = 0x8017 // EVENT_OBJECT_CLOAKED
	EventUncloaked      = 0x8018 // EVENT_OBJECT_UNCLOAKED
)

// eventHandler recebe os eventos; é chamado de dentro do GetMessage, na
// mesma thread do loop principal.
var eventHandler func(event uint32, hwnd uintptr)

// O callback é criado uma vez só: syscall.NewCallback nunca é liberado.
var winEventCb = syscall.NewCallback(func(hook, event, hwnd, idObject, idChild, thread, time uintptr) uintptr {
	// Só eventos da própria janela (OBJID_WINDOW, CHILDID_SELF).
	if int32(idObject) == 0 && int32(idChild) == 0 && eventHandler != nil {
		eventHandler(uint32(event), hwnd)
	}
	return 0
})

// InstallEventHooks passa a entregar a handler os eventos de janela de todos
// os processos (menos o próprio). Os eventos chegam durante GetMessage, então
// deve ser chamado na thread do loop de mensagens.
func InstallEventHooks(handler func(event uint32, hwnd uintptr)) {
	eventHandler = handler
	for _, r := range [][2]uint32{
		{EventForeground, EventForeground},
		{EventMoveSizeStart, EventMoveSizeEnd},
		{EventMinimizeStart, EventMinimizeEnd},
		{EventDestroy, EventHide},
		{EventCloaked, EventUncloaked},
	} {
		pSetWinEventHook.Call(uintptr(r[0]), uintptr(r[1]), 0, winEventCb, 0, 0,
			winEventOutOfContext|winEventSkipOwnProcess)
	}
}

// HookLocationChanges acompanha movimentos/redimensionamentos só da thread
// dona de hwnd (o evento é global e muito frequente). Devolve o hook, que
// deve ser liberado com Unhook.
func HookLocationChanges(hwnd uintptr) uintptr {
	var pid uint32
	tid, _ := windows.GetWindowThreadProcessId(windows.HWND(hwnd), &pid)
	h, _, _ := pSetWinEventHook.Call(EventLocationChange, EventLocationChange, 0,
		winEventCb, uintptr(pid), uintptr(tid), winEventOutOfContext)
	return h
}

func Unhook(h uintptr) { pUnhookWinEvent.Call(h) }
