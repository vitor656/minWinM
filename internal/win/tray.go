//go:build windows

package win

import (
	"errors"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Ícone na área de notificação (perto do relógio). Precisa de uma janela —
// invisível — para receber os cliques no ícone e o aviso de que o Explorer
// reiniciou (quando é preciso adicionar o ícone de novo).

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	pShellNotifyIcon       = shell32.NewProc("Shell_NotifyIconW")
	pRegisterClassEx       = user32.NewProc("RegisterClassExW")
	pCreateWindowEx        = user32.NewProc("CreateWindowExW")
	pDestroyWindow         = user32.NewProc("DestroyWindow")
	pDefWindowProc         = user32.NewProc("DefWindowProcW")
	pDispatchMessage       = user32.NewProc("DispatchMessageW")
	pPostQuitMessage       = user32.NewProc("PostQuitMessage")
	pPostMessage           = user32.NewProc("PostMessageW")
	pRegisterWindowMessage = user32.NewProc("RegisterWindowMessageW")
	pCreatePopupMenu       = user32.NewProc("CreatePopupMenu")
	pAppendMenu            = user32.NewProc("AppendMenuW")
	pTrackPopupMenu        = user32.NewProc("TrackPopupMenu")
	pDestroyMenu           = user32.NewProc("DestroyMenu")
)

const (
	trayCallbackMsg = 0x8000 + 1 // WM_APP + 1
	wmNull          = 0x0000
	wmLButtonUp     = 0x0202
	wmRButtonUp     = 0x0205

	nimAdd     = 0
	nimModify  = 1
	nimDelete  = 2
	nifMessage = 0x1
	nifIcon    = 0x2
	nifTip     = 0x4

	mfString    = 0x0000
	mfGrayed    = 0x0001
	mfChecked   = 0x0008
	mfSeparator = 0x0800

	tpmRightButton = 0x0002
	tpmBottomAlign = 0x0020
	tpmNoNotify    = 0x0080
	tpmReturnCmd   = 0x0100

	wsExToolWindow = 0x00000080
)

// notifyIconData é o NOTIFYICONDATAW (layout de 64 bits).
type notifyIconData struct {
	CbSize          uint32
	Wnd             uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GuidItem        windows.GUID
	BalloonIcon     uintptr
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

// Tray é o ícone do programa na área de notificação. Só existe um por
// processo.
type Tray struct {
	hwnd    uintptr
	icon    uintptr
	tip     string
	added   bool
	onClick func()
}

var (
	theTray           *Tray
	taskbarCreatedMsg uintptr // mensagem que o Explorer manda ao (re)iniciar
)

// O callback é criado uma vez só: syscall.NewCallback nunca é liberado.
var trayWndProc = syscall.NewCallback(func(hwnd, msg, wparam, lparam uintptr) uintptr {
	if t := theTray; t != nil {
		switch {
		case msg == trayCallbackMsg:
			if m := lparam & 0xFFFF; (m == wmLButtonUp || m == wmRButtonUp) && t.onClick != nil {
				t.onClick()
			}
			return 0
		case taskbarCreatedMsg != 0 && msg == taskbarCreatedMsg:
			t.added = false
			t.notify()
			return 0
		}
	}
	r, _, _ := pDefWindowProc.Call(hwnd, msg, wparam, lparam)
	return r
})

// NewTray cria a janela invisível do ícone. onClick roda (na thread do loop
// de mensagens) quando o usuário clica no ícone. O ícone só aparece no
// primeiro Set.
func NewTray(onClick func()) (*Tray, error) {
	if theTray != nil {
		return nil, errors.New("ícone já criado")
	}
	className, _ := windows.UTF16PtrFromString("minWinM.Tray")
	wc := wndClassEx{WndProc: trayWndProc, ClassName: className}
	wc.Size = uint32(unsafe.Sizeof(wc))
	if atom, _, err := pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		return nil, err
	}
	// Janela de topo nunca exibida (uma janela message-only não receberia o
	// TaskbarCreated, que é enviado por broadcast).
	hwnd, _, err := pCreateWindowEx.Call(wsExToolWindow, uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(className)), 0, 0, 0, 0, 0, 0, 0, 0, 0)
	if hwnd == 0 {
		return nil, err
	}
	name, _ := windows.UTF16PtrFromString("TaskbarCreated")
	taskbarCreatedMsg, _, _ = pRegisterWindowMessage.Call(uintptr(unsafe.Pointer(name)))

	theTray = &Tray{hwnd: hwnd, onClick: onClick}
	return theTray, nil
}

// Set mostra (ou atualiza) o ícone e o texto que aparece ao passar o mouse.
func (t *Tray) Set(icon uintptr, tip string) {
	t.icon, t.tip = icon, tip
	t.notify()
}

func (t *Tray) data() notifyIconData {
	d := notifyIconData{
		Wnd:             t.hwnd,
		ID:              1,
		Flags:           nifMessage | nifIcon | nifTip,
		CallbackMessage: trayCallbackMsg,
		Icon:            t.icon,
	}
	d.CbSize = uint32(unsafe.Sizeof(d))
	tip, _ := windows.UTF16FromString(t.tip)
	copy(d.Tip[:len(d.Tip)-1], tip)
	return d
}

func (t *Tray) notify() {
	d := t.data()
	if t.added {
		if ok, _, _ := pShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&d))); ok != 0 {
			return
		}
		// O Explorer pode ter reiniciado sem avisar: tenta adicionar de novo.
	}
	// Falha comum logo no login, antes da barra de tarefas existir; o
	// TaskbarCreated chega depois e o ícone é adicionado então.
	ok, _, _ := pShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&d)))
	t.added = ok != 0
}

// Remove tira o ícone da área de notificação e destrói a janela.
func (t *Tray) Remove() {
	if t == nil {
		return
	}
	d := t.data()
	pShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&d)))
	pDestroyWindow.Call(t.hwnd)
	theTray = nil
}

// MenuItem é uma linha do menu do ícone.
type MenuItem struct {
	ID        int // devolvido por ShowMenu quando escolhido (> 0)
	Text      string
	Checked   bool
	Disabled  bool
	Separator bool
}

// ShowMenu abre o menu na posição do mouse e devolve o ID escolhido, ou 0.
func (t *Tray) ShowMenu(items []MenuItem) int {
	menu, _, _ := pCreatePopupMenu.Call()
	if menu == 0 {
		return 0
	}
	defer pDestroyMenu.Call(menu)
	for _, it := range items {
		if it.Separator {
			pAppendMenu.Call(menu, mfSeparator, 0, 0)
			continue
		}
		flags := uintptr(mfString)
		if it.Checked {
			flags |= mfChecked
		}
		if it.Disabled {
			flags |= mfGrayed
		}
		text, _ := windows.UTF16PtrFromString(it.Text)
		pAppendMenu.Call(menu, flags, uintptr(it.ID), uintptr(unsafe.Pointer(text)))
	}
	p := CursorPos()
	// Sem trazer a janela para frente, o menu não fecha ao clicar fora; o
	// WM_NULL depois é a recomendação da documentação para o mesmo bug.
	pSetForegroundWindow.Call(t.hwnd)
	cmd, _, _ := pTrackPopupMenu.Call(menu, tpmRightButton|tpmBottomAlign|tpmNoNotify|tpmReturnCmd,
		uintptr(p.X), uintptr(p.Y), 0, t.hwnd, 0)
	pPostMessage.Call(t.hwnd, wmNull, 0, 0)
	return int(cmd)
}

// DispatchMessage entrega a mensagem à janela de destino (o ícone).
func DispatchMessage(m *Msg) { pDispatchMessage.Call(uintptr(unsafe.Pointer(m))) }

// PostQuit faz o GetMessage devolver false, encerrando o loop principal.
func PostQuit() { pPostQuitMessage.Call(0) }
