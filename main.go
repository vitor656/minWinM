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
	"runtime"
	"sort"

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

	win.EnablePerMonitorDPI()

	cfg, src, err := config.Load(*cfgPath)
	if err != nil {
		fatal(fmt.Sprintf("Erro ao ler a configuração (%s):\n\n%v", src, err))
	}
	fmt.Println("config:", src)

	bindings := registerBindings(cfg.Bindings)
	if len(bindings) == 0 {
		fatal("Nenhum atalho foi registrado. Confira os 'bindings' do config.json " +
			"(rode com 'go run .' para ver o motivo de cada um).")
	}

	m := wm.New(cfg)
	m.Start()

	var tray *trayUI
	if cfg.TrayIcon == nil || *cfg.TrayIcon {
		tray = startTray(m)
		defer tray.close()
	}
	fmt.Printf("rodando (tiling %s); Ctrl+C ou o atalho '%s' para sair\n",
		map[bool]string{true: "ligado", false: "desligado"}[m.Enabled()], wm.Quit)

	// Atalhos e timers chegam como mensagens da thread (hwnd 0); as da
	// janela do ícone (cliques, Explorer reiniciado) vão para ela.
	var msg win.Msg
	for win.GetMessage(&msg) {
		switch {
		case msg.Hwnd != 0:
			win.DispatchMessage(&msg)
		case msg.Message == win.WMTimer:
			m.OnTimer(msg.WParam)
		case msg.Message == win.WMHotkey:
			b := bindings[int(msg.WParam)]
			if b.name == wm.Quit {
				return
			}
			// lParam (palavra baixa) traz os modificadores do atalho.
			if msg.LParam&(config.ModAlt|config.ModWin) != 0 {
				win.MaskModifierRelease()
			}
			m.Run(b.action)
			tray.refresh() // o atalho pode ter ligado/desligado o tiling
		}
	}
}

// fatal avisa o erro e encerra. Usa uma caixa de mensagem porque, rodando
// em segundo plano (sem console), o stderr não aparece para ninguém.
func fatal(text string) {
	fmt.Fprintln(os.Stderr, "erro:", text)
	win.ErrorBox("minWinM", text)
	os.Exit(1)
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
			fmt.Printf("  ignorado  %-24s ação desconhecida %q\n", k, name)
			continue
		}
		mods, vk, err := config.ParseBinding(k)
		if err != nil {
			fmt.Printf("  ignorado  %-24s %v\n", k, err)
			continue
		}
		if err := win.RegisterHotKey(nextID, mods, vk, action.Repeat); err != nil {
			fmt.Printf("  falhou    %-24s %v (já em uso por outro programa?)\n", k, err)
			continue
		}
		out[nextID] = binding{name: name, action: action}
		fmt.Printf("  ok        %-24s -> %s\n", k, name)
		nextID++
	}
	return out
}
