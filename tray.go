//go:build windows

package main

import (
	"fmt"

	"minwinm/internal/icon"
	"minwinm/internal/win"
	"minwinm/internal/wm"
)

// trayUI é o ícone na área de notificação: mostra que o minWinM está
// rodando, indica se o tiling está ligado (azul) ou desligado (cinza) e abre
// um menu ao ser clicado.
type trayUI struct {
	tray    *win.Tray
	m       *wm.Manager
	on, off uintptr // ícones pré-criados para cada estado
	shown   bool    // estado exibido agora
	ready   bool
}

const (
	menuToggle = iota + 1
	menuRetile
	menuQuit
)

// startTray cria o ícone. Se falhar, o programa segue sem ícone.
func startTray(m *wm.Manager) *trayUI {
	u := &trayUI{m: m}
	tray, err := win.NewTray(u.openMenu)
	if err != nil {
		fmt.Println("ícone da área de notificação indisponível:", err)
		return nil
	}
	size := win.SmallIconSize()
	u.on, _ = win.NewIcon(size, icon.Pixels(size, icon.ColorActive))
	u.off, _ = win.NewIcon(size, icon.Pixels(size, icon.ColorInactive))
	u.tray = tray
	u.refresh()
	return u
}

// refresh atualiza ícone e texto se o estado do tiling mudou.
func (u *trayUI) refresh() {
	if u == nil || (u.ready && u.shown == u.m.Enabled()) {
		return
	}
	u.shown, u.ready = u.m.Enabled(), true
	ic, state := u.off, "desligado"
	if u.shown {
		ic, state = u.on, "ligado"
	}
	u.tray.Set(ic, "minWinM — tiling "+state)
}

func (u *trayUI) openMenu() {
	enabled := u.m.Enabled()
	choice := u.tray.ShowMenu([]win.MenuItem{
		{ID: menuToggle, Text: "Tiling automático", Checked: enabled},
		{ID: menuRetile, Text: "Reorganizar agora", Disabled: !enabled},
		{Separator: true},
		{ID: menuQuit, Text: "Sair"},
	})
	switch choice {
	case menuToggle:
		u.run("toggle-tiling")
	case menuRetile:
		u.run("retile")
	case menuQuit:
		win.PostQuit()
	}
	u.refresh()
}

func (u *trayUI) run(action string) {
	if a, ok := wm.Lookup(action); ok {
		u.m.Run(a)
	}
}

// close tira o ícone da área de notificação (senão ele fica lá até o mouse
// passar por cima).
func (u *trayUI) close() {
	if u == nil {
		return
	}
	u.tray.Remove()
	win.DestroyIcon(u.on)
	win.DestroyIcon(u.off)
}
