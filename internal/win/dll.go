//go:build windows

// Package win embrulha as chamadas à API do Windows (user32/dwmapi) que o
// minWinM usa. Só mecanismo: quais janelas gerenciar e o que fazer com elas
// é decidido no pacote wm.
//
// Retângulos usam grid.Rect, que tem o mesmo layout de memória do RECT.
package win

import "golang.org/x/sys/windows"

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	dwmapi = windows.NewLazySystemDLL("dwmapi.dll")

	pRegisterHotKey                = user32.NewProc("RegisterHotKey")
	pGetMessage                    = user32.NewProc("GetMessageW")
	pGetForegroundWindow           = user32.NewProc("GetForegroundWindow")
	pSetWindowPos                  = user32.NewProc("SetWindowPos")
	pGetWindowRect                 = user32.NewProc("GetWindowRect")
	pIsZoomed                      = user32.NewProc("IsZoomed")
	pShowWindow                    = user32.NewProc("ShowWindow")
	pGetClassName                  = user32.NewProc("GetClassNameW")
	pMonitorFromWindow             = user32.NewProc("MonitorFromWindow")
	pGetMonitorInfo                = user32.NewProc("GetMonitorInfoW")
	pEnumDisplayMonitors           = user32.NewProc("EnumDisplayMonitors")
	pSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	pDwmGetWindowAttribute         = dwmapi.NewProc("DwmGetWindowAttribute")

	pEnumWindows         = user32.NewProc("EnumWindows")
	pIsWindow            = user32.NewProc("IsWindow")
	pIsWindowVisible     = user32.NewProc("IsWindowVisible")
	pIsIconic            = user32.NewProc("IsIconic")
	pIsHungAppWindow     = user32.NewProc("IsHungAppWindow")
	pGetWindow           = user32.NewProc("GetWindow")
	pGetWindowLong       = user32.NewProc("GetWindowLongW")
	pGetWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	pSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	pBringWindowToTop    = user32.NewProc("BringWindowToTop")
	pAttachThreadInput   = user32.NewProc("AttachThreadInput")
	pGetCursorPos        = user32.NewProc("GetCursorPos")
	pMonitorFromPoint    = user32.NewProc("MonitorFromPoint")
	pSetWinEventHook     = user32.NewProc("SetWinEventHook")
	pUnhookWinEvent      = user32.NewProc("UnhookWinEvent")
	pSetTimer            = user32.NewProc("SetTimer")
	pKillTimer           = user32.NewProc("KillTimer")
	pKeybdEvent          = user32.NewProc("keybd_event")
)

// Constantes internas da API.
const (
	modNoRepeat = 0x4000

	swMinimize = 6
	swRestore  = 9

	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010

	monitorDefaultToNearest  = 2
	dwmwaExtendedFrameBounds = 9
	dwmwaCloaked             = 14

	gwOwner    = 4
	gwlStyle   = -16
	gwlExStyle = -20

	winEventOutOfContext   = 0x0000
	winEventSkipOwnProcess = 0x0002
)
