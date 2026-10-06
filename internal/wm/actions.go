//go:build windows

package wm

import (
	"fmt"

	"minwinm/internal/grid"
	"minwinm/internal/win"
)

// Quit é a ação que encerra o programa; quem trata é o loop principal.
const Quit = "quit"

// maxDesktopShortcut: há ações desktop-N e move-to-desktop-N para N de 1 a 9.
const maxDesktopShortcut = 9

// Action é o que um atalho do config.json dispara.
type Action struct {
	// Repeat: o atalho dispara de novo enquanto a tecla estiver segurada.
	Repeat bool
	run    func(m *Manager)
}

// Lookup encontra a ação pelo nome usado no config.json.
func Lookup(name string) (Action, bool) {
	a, ok := actions[name]
	return a, ok
}

// Run executa a ação.
func (m *Manager) Run(a Action) {
	if a.run != nil {
		a.run(m)
	}
}

// actions é a lista completa de ações; é a única fonte da verdade (o README
// documenta os mesmos nomes).
var actions = buildActions()

func buildActions() map[string]Action {
	acts := map[string]Action{
		Quit: {}, // tratada pelo loop principal

		// Grid (não dependem da janela focada).
		"toggle-tiling": {run: (*Manager).toggle},
		"retile": {run: func(m *Manager) {
			if m.enabled {
				m.retile()
			}
		}},
		"gap-increase": {Repeat: true, run: func(m *Manager) { m.changeGap(+2) }},
		"gap-decrease": {Repeat: true, run: func(m *Manager) { m.changeGap(-2) }},

		// Tamanho.
		"grow-width":    {Repeat: true, run: onFocused(func(m *Manager, h uintptr) { m.resize(h, true, m.step) })},
		"shrink-width":  {Repeat: true, run: onFocused(func(m *Manager, h uintptr) { m.resize(h, true, -m.step) })},
		"grow-height":   {Repeat: true, run: onFocused(func(m *Manager, h uintptr) { m.resize(h, false, m.step) })},
		"shrink-height": {Repeat: true, run: onFocused(func(m *Manager, h uintptr) { m.resize(h, false, -m.step) })},
		"balance":       {run: onFocused((*Manager).balance)},

		// Janela focada.
		"toggle-center":      {run: onFocused((*Manager).toggleCenter)},
		"toggle-full-height": {run: onFocused((*Manager).toggleFullHeight)},
		"minimize":           {run: onFocused(func(_ *Manager, h uintptr) { win.Minimize(h) })},
		"next-monitor":       {run: onFocused(func(m *Manager, h uintptr) { m.moveToMonitor(h, +1) })},
		"prev-monitor":       {run: onFocused(func(m *Manager, h uintptr) { m.moveToMonitor(h, -1) })},
	}

	// Foco e movimento, um por direção: focus-left, move-left, ...
	for _, name := range grid.DirNames() {
		d, _ := grid.ParseDir(name)
		acts["focus-"+name] = Action{run: func(m *Manager) { m.focus(d) }}
		acts["move-"+name] = Action{run: onFocused(func(m *Manager, h uintptr) { m.move(h, d) })}
	}

	// Áreas de trabalho virtuais: desktop-1..9, move-to-desktop-1..9, e as
	// vizinhas (next/prev dão a volta nas pontas).
	for n := 1; n <= maxDesktopShortcut; n++ {
		acts[fmt.Sprintf("desktop-%d", n)] = Action{run: func(m *Manager) { m.gotoDesktop(n, 0) }}
		acts[fmt.Sprintf("move-to-desktop-%d", n)] = Action{run: onFocused(func(m *Manager, h uintptr) { m.sendToDesktop(h, n, 0) })}
	}
	acts["desktop-next"] = Action{run: func(m *Manager) { m.gotoDesktop(0, +1) }}
	acts["desktop-prev"] = Action{run: func(m *Manager) { m.gotoDesktop(0, -1) }}
	acts["move-to-desktop-next"] = Action{run: onFocused(func(m *Manager, h uintptr) { m.sendToDesktop(h, 0, +1) })}
	acts["move-to-desktop-prev"] = Action{run: onFocused(func(m *Manager, h uintptr) { m.sendToDesktop(h, 0, -1) })}

	// Presets: o nome da ação é o nome do preset.
	for _, name := range grid.PresetNames() {
		acts[name] = Action{run: onFocused(func(m *Manager, h uintptr) { m.preset(h, name) })}
	}
	return acts
}

// onFocused adapta uma ação sobre a janela em foco; não faz nada se o foco
// está no desktop ou na barra de tarefas.
func onFocused(f func(m *Manager, hwnd uintptr)) func(*Manager) {
	return func(m *Manager) {
		if hwnd := win.ForegroundWindow(); shouldManage(hwnd) {
			f(m, hwnd)
		}
	}
}
