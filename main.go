//go:build windows

// minWinM: tiling automático de janelas para Windows, com atalhos no estilo Vim.
//
// main só faz a ligação entre as partes: carrega a config, registra os
// atalhos e roda o loop de mensagens do Windows, entregando atalhos e
// timers ao gerenciador (internal/wm).
package main

// Recursos do executável (ícone e descrição "minWinM" vista no Gerenciador de
// Tarefas): gera winres/*.png e rsrc_windows_amd64.syso, que o go build
// embute sozinho. Rodar `go generate` depois de mudar o ícone ou a versão.
//go:generate go run ./tools/genicon winres
//go:generate go run github.com/tc-hib/go-winres@v0.3.3 make --in winres/winres.json --arch amd64

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"minwinm/internal/applog"
	"minwinm/internal/config"
	"minwinm/internal/win"
	"minwinm/internal/wm"
)

func main() {
	// Os hotkeys e os hooks de eventos ficam presos à thread que os registrou.
	runtime.LockOSThread()

	cfgPath := flag.String("config", "", "caminho do config.json")
	flag.Parse()

	// Duas cópias brigariam pelos mesmos atalhos e pelas mesmas janelas.
	if !win.SingleInstance(`Local\minWinM`) {
		fatal("O minWinM já está rodando.\n\nPara fechar, use o atalho 'quit' (padrão Ctrl+Alt+Q).")
	}

	applog.Init(logPath())
	defer applog.Close()

	win.EnablePerMonitorDPI()

	cfg, src, err := config.Load(*cfgPath)
	if err != nil {
		fatal(fmt.Sprintf("Erro ao ler a configuração (%s):\n\n%v", src, err))
	}
	if cfg.LogVerbose {
		applog.SetVerbose(true)
	}
	applog.Printf("minWinM iniciando: Windows build %d, config %s", win.OSBuild(), src)

	bindings := registerBindings(cfg.Bindings)
	if len(bindings) == 0 {
		fatal("Nenhum atalho foi registrado. Confira os 'bindings' do config.json " +
			"(rode com 'go run .' para ver o motivo de cada um).")
	}

	m := wm.New(cfg)
	m.Start()
	if applog.Verbose() {
		m.LogState()
	}

	var tray *trayUI
	if cfg.TrayIcon == nil || *cfg.TrayIcon {
		tray = startTray(m)
		defer tray.close()
	}
	applog.Printf("rodando (tiling %s); Ctrl+C ou o atalho '%s' para sair",
		map[bool]string{true: "ligado", false: "desligado"}[m.Enabled()], wm.Quit)

	// Atalhos e timers chegam como mensagens da thread (hwnd 0); as da
	// janela do ícone (cliques, Explorer reiniciado) vão para ela.
	var msg win.Msg
	for win.GetMessage(&msg) {
		switch {
		case msg.Hwnd != 0:
			win.DispatchMessage(&msg)
		case msg.Message == win.WMTimer:
			applog.Guard("re-tile", func() { m.OnTimer(msg.WParam) })
		case msg.Message == win.WMHotkey:
			b := bindings[int(msg.WParam)]
			if b.name == wm.Quit {
				return
			}
			// lParam (palavra baixa) traz os modificadores do atalho.
			if msg.LParam&(config.ModAlt|config.ModWin) != 0 {
				win.MaskModifierRelease()
			}
			applog.Guard("atalho "+b.name, func() {
				applog.Printf("atalho: %s", b.name)
				m.Run(b.action)
			})
			tray.refresh() // o atalho pode ter ligado/desligado o tiling
		}
	}
}

// fatal avisa o erro e encerra. Usa uma caixa de mensagem porque, rodando
// em segundo plano (sem console), o stderr não aparece para ninguém.
func fatal(text string) {
	applog.Errorf("%s", text)
	applog.Close()
	win.ErrorBox("minWinM", text)
	os.Exit(1)
}

// logPath é o minWinM.log ao lado do executável (junto do config.json da
// instalação). Vazio, se não der para saber onde está o executável.
func logPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "minWinM.log")
}

type binding struct {
	name   string
	action wm.Action
}

// registerBindings registra os atalhos da config e devolve as ações por id
// de hotkey, imprimindo o resultado de cada um.
func registerBindings(keys map[string]string) map[int]binding {
	// Ordena para ids estáveis e mensagens de erro previsíveis.
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	out := map[int]binding{}
	nextID := 1
	for _, k := range sorted {
		name := keys[k]
		action, ok := wm.Lookup(name)
		if !ok {
			applog.Errorf("atalho ignorado: %s: ação desconhecida %q", k, name)
			continue
		}
		mods, vk, err := config.ParseBinding(k)
		if err != nil {
			applog.Errorf("atalho ignorado: %s: %v", k, err)
			continue
		}
		if err := win.RegisterHotKey(nextID, mods, vk, action.Repeat); err != nil {
			applog.Errorf("atalho falhou: %s: %v (já em uso por outro programa?)", k, err)
			continue
		}
		out[nextID] = binding{name: name, action: action}
		applog.Printf("  ok        %-24s -> %s", k, name)
		nextID++
	}
	return out
}
