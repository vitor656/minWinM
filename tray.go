//go:build windows

package main

import (
	"os"
	"time"

	"minwinm/internal/applog"
	"minwinm/internal/config"
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
	cfgPath string      // config.json que o usuário edita (pode ainda não existir)
	loaded  time.Time   // data de modificação do cfgPath quando foi lido
	reload  func() bool // relê a config; false se não aplicou
	on, off uintptr     // ícones pré-criados para cada estado
	shown   bool        // estado exibido agora
	ready   bool
}

const (
	menuToggle = iota + 1
	menuRetile
	menuConfig
	menuReload
	menuLog
	menuOpenLog
	menuQuit
)

// startTray cria o ícone. Se falhar, o programa segue sem ícone.
func startTray(m *wm.Manager, cfgPath string, reload func() bool) *trayUI {
	u := &trayUI{m: m, cfgPath: cfgPath, loaded: config.ModTime(cfgPath), reload: reload}
	tray, err := win.NewTray(func() { applog.Guard("menu do ícone", u.openMenu) })
	if err != nil {
		applog.Errorf("ícone da área de notificação indisponível: %v", err)
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
		{ID: menuConfig, Text: "Editar atalhos (config.json)", Disabled: u.cfgPath == ""},
		{ID: menuReload, Text: "Recarregar config", Disabled: !u.configChanged()},
		{ID: menuLog, Text: "Log detalhado", Checked: applog.Verbose()},
		{ID: menuOpenLog, Text: "Abrir log", Disabled: !logExists()},
		{Separator: true},
		{ID: menuQuit, Text: "Sair"},
	})
	switch choice {
	case menuToggle:
		u.run("toggle-tiling")
	case menuRetile:
		u.run("retile")
	case menuConfig:
		u.editConfig()
	case menuReload:
		if u.reload() {
			u.loaded = config.ModTime(u.cfgPath)
		}
	case menuLog:
		applog.SetVerbose(!applog.Verbose())
		if applog.Verbose() {
			u.m.LogState()
		}
	case menuOpenLog:
		if err := win.OpenFile(applog.Path()); err != nil {
			applog.Errorf("abrir o log: %v", err)
		}
	case menuQuit:
		win.PostQuit()
	}
	u.refresh()
}

// editConfig abre o config.json para edição, criando-o com o padrão se o
// programa está usando a configuração embutida. As mudanças valem ao
// usar "Recarregar config".
func (u *trayUI) editConfig() {
	created, err := config.EnsureFile(u.cfgPath)
	if err != nil {
		applog.Errorf("criar %s: %v", u.cfgPath, err)
		return
	}
	if created {
		// Igual ao padrão embutido que já está valendo: não conta como mudança.
		u.loaded = config.ModTime(u.cfgPath)
	}
	if err := win.EditFile(u.cfgPath); err != nil {
		applog.Errorf("abrir %s: %v", u.cfgPath, err)
	}
}

// configChanged diz se o config.json mudou (ou sumiu) depois de lido.
func (u *trayUI) configChanged() bool {
	return u.cfgPath != "" && !config.ModTime(u.cfgPath).Equal(u.loaded)
}

// logExists diz se o arquivo de log já foi criado.
func logExists() bool {
	p := applog.Path()
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
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
