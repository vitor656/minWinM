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

// version é a versão publicada; o workflow de release a define com
// -ldflags "-X main.version=..." a partir da tag.
var version = "dev"

func main() {
	// Os hotkeys e os hooks de eventos ficam presos à thread que os registrou.
	runtime.LockOSThread()

	cfgPath := flag.String("config", "", "caminho do config.json")
	flag.Parse()

	// Iniciado de dentro do VS Code (extensão, tarefa), o minWinM herda
	// ELECTRON_RUN_AS_NODE=1 e o repassa a tudo que abre: o Code.exe aberto
	// por "Editar atalhos" rodaria como Node e sairia sem abrir o arquivo.
	os.Unsetenv("ELECTRON_RUN_AS_NODE")

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
	applog.Printf("minWinM %s iniciando: Windows build %d, config %s", version, win.OSBuild(), src)

	keys := cfg.Bindings
	bindings := registerBindings(keys)
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
		tray = startTray(m, config.UserPath(*cfgPath), func() bool {
			return reload(*cfgPath, m, &keys, &bindings)
		})
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

// reload relê a config com o programa rodando: troca os atalhos e aplica o
// resto ao gerenciador. Com a config inválida, avisa e mantém a atual; se
// nenhum atalho novo registrar, volta aos anteriores. Devolve false se não
// aplicou.
func reload(path string, m *wm.Manager, keys *map[string]string, bindings *map[int]binding) bool {
	cfg, src, err := config.Load(path)
	if err != nil {
		applog.Errorf("recarregar a config (%s): %v", src, err)
		win.ErrorBox("minWinM", fmt.Sprintf("A configuração não foi recarregada (%s):\n\n%v", src, err))
		return false
	}
	applog.Printf("recarregando a config %s", src)
	unregisterBindings(*bindings)
	next := registerBindings(cfg.Bindings)
	if len(next) == 0 {
		*bindings = registerBindings(*keys)
		win.ErrorBox("minWinM", "Nenhum atalho do novo config.json foi registrado; "+
			"a configuração anterior continua valendo.")
		return false
	}
	*keys, *bindings = cfg.Bindings, next
	if cfg.LogVerbose {
		applog.SetVerbose(true)
	}
	m.Apply(cfg)
	return true
}

func unregisterBindings(bindings map[int]binding) {
	for id := range bindings {
		win.UnregisterHotKey(id)
	}
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
