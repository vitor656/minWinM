//go:build windows

// Package wm é o gerenciador de janelas do minWinM: mantém um grid por
// monitor, reage aos eventos do Windows e executa as ações dos atalhos.
//
// Arquivos:
//   - manager.go   estado geral, sincronização com as janelas e layout
//   - workspace.go o grid de um monitor e seus slots
//   - filter.go    quais janelas entram no grid
//   - events.go    eventos do Windows (abrir, fechar, foco, arrastar)
//   - drag.go      redimensionamento pela borda com o mouse
//   - actions.go   tabela de ações (nome no config.json → função)
//   - commands.go  implementação das ações
//
// Tudo roda na thread do loop de mensagens (eventos e atalhos chegam por
// GetMessage), então não há concorrência nem locks.
package wm

import (
	"fmt"
	"sort"
	"strings"

	"minwinm/internal/applog"
	"minwinm/internal/config"
	"minwinm/internal/grid"
	"minwinm/internal/win"
)

// retileDelay é o atraso entre um evento de janela e o re-tile: abrir uma
// janela dispara vários eventos seguidos e o app costuma se reposicionar
// logo após aparecer.
const retileDelay = 150 // ms

// Manager é o estado do gerenciador.
type Manager struct {
	enabled bool
	gaps    grid.Gaps
	// outerFollows: sem outer_gap na config, a margem das bordas acompanha
	// o gap entre janelas (inclusive quando muda em tempo de execução).
	outerFollows bool
	step         float64 // fração da tela por grow/shrink
	ignore       map[string]bool
	// spaces tem um grid por (monitor, área de trabalho virtual); só os da
	// área atual (desk) são posicionados — ver current().
	spaces []*workspace
	desk   win.DesktopID // área de trabalho atual (zero se desconhecida)
	vd     *win.Desktops
	saved  map[uintptr]savedPos // presets no modo manual (ver preset)
	timer  uintptr              // re-tile agendado (0 = nenhum)
	drag   dragState            // arrasto com o mouse em andamento
}

// savedPos guarda, no modo manual, onde a janela estava antes do preset.
type savedPos struct {
	r      grid.Rect
	preset string
}

// New cria o gerenciador a partir da config. Não mexe em nenhuma janela;
// chame Start depois.
func New(cfg config.Config) *Manager {
	m := &Manager{
		enabled:      cfg.Tiling == nil || *cfg.Tiling,
		gaps:         grid.Gaps{Inner: int32(max(0, cfg.Gap)), Outer: int32(max(0, cfg.Gap))},
		outerFollows: cfg.OuterGap == nil,
		step:         cfg.ResizeStep,
		ignore:       map[string]bool{},
		saved:        map[uintptr]savedPos{},
	}
	if cfg.OuterGap != nil {
		m.gaps.Outer = int32(max(0, *cfg.OuterGap))
	}
	if m.step <= 0 {
		m.step = 0.05
	}
	for _, s := range cfg.Ignore {
		m.ignore[strings.ToLower(s)] = true
	}
	return m
}

// Start instala os hooks de eventos e, com o tiling ligado, monta o grid
// inicial. Deve rodar na thread do loop de mensagens.
func (m *Manager) Start() {
	m.vd = win.OpenDesktops()
	if !m.vd.Available() {
		m.errorf("áreas de trabalho: só o grid por área funciona; trocar/mover não é suportado nesta versão do Windows (build %d)", win.OSBuild())
	}
	win.InstallEventHooks(func(event uint32, hwnd uintptr) {
		applog.Guard("evento de janela", func() { m.OnEvent(event, hwnd) })
	})
	if m.enabled {
		m.retile()
	}
}

// Enabled diz se o tiling automático está ligado.
func (m *Manager) Enabled() bool { return m.enabled }

func (m *Manager) find(hwnd uintptr) (*workspace, int) {
	for _, ws := range m.spaces {
		for i, tl := range ws.tiles {
			if tl.hwnd == hwnd {
				return ws, i
			}
		}
	}
	return nil, -1
}

// spaceFor devolve o grid do monitor mon na área de trabalho desk, criando
// se ainda não existe.
func (m *Manager) spaceFor(mon uintptr, desk win.DesktopID) *workspace {
	for _, ws := range m.spaces {
		if ws.mon == mon && ws.desk == desk {
			return ws
		}
	}
	ws := &workspace{mon: mon, desk: desk}
	m.spaces = append(m.spaces, ws)
	return ws
}

// current devolve os grids da área de trabalho atual, um por monitor, da
// esquerda para a direita.
func (m *Manager) current() []*workspace {
	var out []*workspace
	for _, mon := range win.Monitors() {
		out = append(out, m.spaceFor(mon.Handle, m.desk))
	}
	return out
}

// windowDesktop diz em qual área de trabalho está a janela.
func (m *Manager) windowDesktop(hwnd uintptr) (win.DesktopID, bool) {
	if m.vd == nil {
		return win.DesktopID{}, false
	}
	return m.vd.WindowDesktop(hwnd)
}

// desktopOf é windowDesktop, ou a área atual se não der para saber.
func (m *Manager) desktopOf(hwnd uintptr) win.DesktopID {
	if id, ok := m.windowDesktop(hwnd); ok {
		return id
	}
	return m.desk
}

func (m *Manager) refreshDesktop() {
	if m.vd != nil {
		if id, ok := m.vd.Current(); ok {
			m.desk = id
		}
	}
}

// sync alinha o estado com as janelas, monitores e áreas de trabalho que
// existem agora.
func (m *Manager) sync() {
	m.refreshDesktop()

	// Grids de monitores desconectados ou de áreas de trabalho removidas
	// somem; as janelas deles entram abaixo como novas, onde o Windows as
	// colocou.
	monitors := map[uintptr]bool{}
	for _, mon := range win.Monitors() {
		monitors[mon.Handle] = true
	}
	desks := map[win.DesktopID]bool{}
	if m.vd != nil {
		for _, id := range m.vd.List() {
			desks[id] = true
		}
	}
	kept := m.spaces[:0]
	for _, ws := range m.spaces {
		if monitors[ws.mon] && (len(desks) == 0 || desks[ws.desk] || ws.desk == m.desk) {
			kept = append(kept, ws)
		} else {
			m.logf("grid removido (monitor %#x ou área %v não existe mais), %d janelas", ws.mon, ws.desk, len(ws.tiles))
		}
	}
	m.spaces = kept

	for _, ws := range m.spaces {
		alive := ws.tiles[:0]
		for _, tl := range ws.tiles {
			if win.IsWindow(tl.hwnd) && win.IsVisible(tl.hwnd) {
				alive = append(alive, tl)
			} else {
				m.logf("saiu do grid: %s", tl.name())
			}
		}
		ws.tiles = alive
	}

	// Janelas que mudaram de área de trabalho por fora (Visão de Tarefas)
	// vão para o grid da nova área, no mesmo monitor.
	for _, ws := range append([]*workspace(nil), m.spaces...) {
		for i := 0; i < len(ws.tiles); i++ {
			if d, ok := m.windowDesktop(ws.tiles[i].hwnd); ok && d != ws.desk {
				tl := ws.tiles[i]
				ws.tiles = append(ws.tiles[:i], ws.tiles[i+1:]...)
				target := m.spaceFor(ws.mon, d)
				target.tiles = append(target.tiles, tl)
				i--
			}
		}
	}

	fresh := map[*workspace][]uintptr{}
	for _, h := range win.TopLevelWindows() {
		if ws, _ := m.find(h); ws != nil || !m.tileable(h) {
			continue
		}
		mon, ok := win.MonitorOf(h)
		if !ok {
			continue
		}
		ws := m.spaceFor(mon.Handle, m.desktopOf(h))
		fresh[ws] = append(fresh[ws], h)
	}
	for ws, hs := range fresh {
		// Ordena pela posição atual para o grid inicial lembrar o arranjo
		// que já existia (esquerda→direita, cima→baixo).
		sort.SliceStable(hs, func(i, j int) bool {
			a, _ := win.FrameBounds(hs[i])
			b, _ := win.FrameBounds(hs[j])
			if a.Left != b.Left {
				return a.Left < b.Left
			}
			return a.Top < b.Top
		})
		for _, h := range hs {
			tl := &tile{hwnd: h}
			if applog.Verbose() {
				tl.desc = describe(h)
				m.logf("entrou no grid: %s (monitor %#x)", tl.desc, ws.mon)
			}
			ws.tiles = append(ws.tiles, tl)
		}
	}
}

// layout posiciona as janelas do grid de ws nos seus slots. Grids de outras
// áreas de trabalho ficam como estão: suas janelas estão ocultas (cloaked),
// e calcular slots sem elas zeraria as proporções guardadas.
func (m *Manager) layout(ws *workspace) {
	if ws.desk != m.desk {
		return
	}
	mon, ok := win.MonitorInfo(ws.mon)
	if !ok {
		return
	}
	slots := ws.slots(mon)
	for i, tl := range slots {
		// A janela sendo arrastada fica com o usuário até ele soltar.
		if tl.floating || win.IsHung(tl.hwnd) || tl.hwnd == m.drag.hwnd {
			continue
		}
		win.Place(tl.hwnd, ws.slotRect(mon, slots, i, m.gaps))
	}
}

func (m *Manager) layoutAll() {
	for _, ws := range m.current() {
		m.layout(ws)
	}
}

func (m *Manager) retile() {
	m.sync()
	m.layoutAll()
}

// applySizes tenta levar o grid de ws de before para target. Apps com
// tamanho mínimo não encolhem além dele e passariam a cobrir a vizinha;
// nesse caso tenta metade e um quarto do caminho e, se nada couber, volta
// para before — esse é o limite do redimensionamento.
func (m *Manager) applySizes(ws *workspace, mon win.Monitor, before, target grid.Sizes) {
	for _, k := range []float64{1, 0.5, 0.25} {
		ws.Restore(grid.Blend(before, target, k))
		m.layout(ws)
		if !ws.overflows(mon, m.gaps) {
			return
		}
	}
	m.logf("redimensionar: alguma janela não encolhe tanto (tamanho mínimo do app); mantido como estava")
	ws.Restore(before)
	m.layout(ws)
}

func onOff(b bool) string {
	if b {
		return "ligado"
	}
	return "desligado"
}

// logf registra um detalhe (no arquivo, só com o log detalhado ligado);
// errorf, uma falha (sempre no arquivo).
func (m *Manager) logf(format string, args ...any)   { applog.Printf(format, args...) }
func (m *Manager) errorf(format string, args ...any) { applog.Errorf(format, args...) }

// describe identifica uma janela no log: handle, executável e classe. Não
// usa o título, que pode ter dados do usuário (nome de documento, e-mail...).
func describe(hwnd uintptr) string {
	return fmt.Sprintf("%#x %s [%s]", hwnd, win.ProcessName(hwnd), win.ClassName(hwnd))
}

// name identifica o tile no log mesmo depois que a janela fechou.
func (tl *tile) name() string {
	if tl.desc == "" {
		return fmt.Sprintf("%#x", tl.hwnd)
	}
	return tl.desc
}

// LogState registra o estado atual (monitores e janelas de cada grid): o
// retrato inicial quando o log detalhado é ligado.
func (m *Manager) LogState() {
	m.logf("estado: tiling %s, gap %d/%d, trocar/mover áreas de trabalho: %s",
		onOff(m.enabled), m.gaps.Inner, m.gaps.Outer, onOff(m.vd != nil && m.vd.Available()))
	for _, mon := range win.Monitors() {
		m.logf("  monitor %#x: tela %v, área útil %v", mon.Handle, mon.Full, mon.Work)
	}
	for _, ws := range m.spaces {
		cur := ""
		if ws.desk == m.desk {
			cur = " (atual)"
		}
		m.logf("  grid do monitor %#x, área %v%s: formato %v", ws.mon, ws.desk, cur, ws.Shape())
		for _, tl := range ws.tiles {
			var flags []string
			if tl.floating {
				flags = append(flags, "flutuante:"+tl.preset)
			}
			if tl.tall {
				flags = append(flags, "altura-total")
			}
			if win.IsIconic(tl.hwnd) {
				flags = append(flags, "minimizada")
			}
			m.logf("    %s %v", describe(tl.hwnd), flags)
		}
	}
}
