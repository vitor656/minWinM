//go:build windows

package wm

import (
	"sort"
	"strings"
	"testing"

	"minwinm/internal/config"
	"minwinm/internal/win"
)

// Lista completa de ações aceitas no config.json; a tabela precisa ter
// exatamente estas (ao adicionar uma ação, inclua aqui e no README).
var wantActions = []string{
	"desktop-1", "desktop-2", "desktop-3", "desktop-4", "desktop-5",
	"desktop-6", "desktop-7", "desktop-8", "desktop-9", "desktop-next", "desktop-prev",
	"move-to-desktop-1", "move-to-desktop-2", "move-to-desktop-3", "move-to-desktop-4",
	"move-to-desktop-5", "move-to-desktop-6", "move-to-desktop-7", "move-to-desktop-8",
	"move-to-desktop-9", "move-to-desktop-next", "move-to-desktop-prev",
	"desktop-create", "desktop-delete",
	"quit", "next-monitor", "prev-monitor",
	"toggle-tiling", "retile", "balance", "toggle-center", "minimize", "toggle-full-height",
	"toggle-solo-column",
	"grow-width", "shrink-width", "grow-height", "shrink-height",
	"gap-increase", "gap-decrease",
	"focus-left", "focus-down", "focus-up", "focus-right",
	"move-left", "move-down", "move-up", "move-right",
	"left", "right", "top", "bottom",
	"top-left", "top-right", "bottom-left", "bottom-right",
	"left-third", "center-third", "right-third", "left-two-thirds", "right-two-thirds",
	"maximize", "center",
}

func TestActionTable(t *testing.T) {
	var got []string
	for name := range actions {
		got = append(got, name)
	}
	want := append([]string(nil), wantActions...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("ações:\n got  %v\n want %v", got, want)
	}
	for name, a := range actions {
		// Repetem com a tecla segurada exatamente grow-*, shrink-* e gap-*.
		wantRepeat := strings.HasPrefix(name, "grow-") || strings.HasPrefix(name, "shrink-") ||
			strings.HasPrefix(name, "gap-")
		if a.Repeat != wantRepeat {
			t.Errorf("%s: Repeat = %v, want %v", name, a.Repeat, wantRepeat)
		}
		if (a.run == nil) != (name == Quit) {
			t.Errorf("%s: run nil = %v", name, a.run == nil)
		}
	}
	if _, ok := Lookup("focus-diagonal"); ok {
		t.Error("Lookup aceitou ação inexistente")
	}
}

func TestDefaultBindingsResolve(t *testing.T) {
	cfg, _, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	for key, name := range cfg.Bindings {
		if _, ok := Lookup(name); !ok {
			t.Errorf("%s -> %q: ação desconhecida", key, name)
		}
	}
}

func TestFullHeight(t *testing.T) {
	// 3 janelas: A à esquerda; B e C empilhadas à direita, C em altura total.
	ws := &workspace{}
	ws.Fit(3)
	slots := []*tile{{hwnd: 1}, {hwnd: 2}, {hwnd: 3, tall: true}}
	if f := ws.slotFrac(slots, 2); f.Y != 0 || f.H != 1 || f.X != 0.5 {
		t.Fatalf("C em altura total = %+v", f)
	}
	if f := ws.slotFrac(slots, 1); f.H != 0.5 {
		t.Fatalf("B mudou: %+v", f)
	}
	if col := ws.sameColumn(slots, 1); len(col) != 2 || col[0].hwnd != 2 || col[1].hwnd != 3 {
		t.Fatalf("coluna de B = %v", col)
	}
	if !ws.coveredByTall(slots, 1) || ws.coveredByTall(slots, 2) || ws.coveredByTall(slots, 0) {
		t.Fatal("coveredByTall errado")
	}
	if slotOf(slots, slots[2]) != 2 || slotOf(slots, &tile{}) != -1 {
		t.Fatal("slotOf errado")
	}
}

func TestNewDefaults(t *testing.T) {
	gap, outer := 6, 2
	m := New(config.Config{Gap: gap, OuterGap: &outer, ResizeStep: -1, Ignore: []string{"Foo.EXE"}})
	if !m.enabled || m.gaps.Inner != 6 || m.gaps.Outer != 2 || m.outerFollows || m.step != 0.05 || !m.ignore["foo.exe"] {
		t.Fatalf("New = %+v", m)
	}
	m = New(config.Config{Gap: 4})
	if m.gaps.Outer != 4 || !m.outerFollows {
		t.Fatalf("outer_gap omitido deveria seguir gap: %+v", m.gaps)
	}
}

func TestPickDesktop(t *testing.T) {
	d := func(b byte) win.DesktopID { return win.DesktopID{Data1: uint32(b)} }
	list := []win.DesktopID{d(1), d(2), d(3)}
	cases := []struct {
		name    string
		cur     win.DesktopID
		n, step int
		want    win.DesktopID
		ok      bool
	}{
		{"pula direto da 1 para a 3", d(1), 3, 0, d(3), true},
		{"área que não existe", d(1), 4, 0, win.DesktopID{}, false},
		{"próxima", d(1), 0, +1, d(2), true},
		{"anterior", d(2), 0, -1, d(1), true},
		{"próxima na última dá a volta", d(3), 0, +1, d(1), true},
		{"anterior na primeira dá a volta", d(1), 0, -1, d(3), true},
	}
	for _, c := range cases {
		got, ok := pickDesktop(list, c.cur, c.n, c.step)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: = %v, %v; want %v, %v", c.name, got, ok, c.want, c.ok)
		}
	}
	if _, ok := pickDesktop(list[:1], d(1), 0, +1); ok {
		t.Error("com uma área só, próxima/anterior não deveria fazer nada")
	}
}

func TestFallbackDesktop(t *testing.T) {
	d := func(b byte) win.DesktopID { return win.DesktopID{Data1: uint32(b)} }
	list := []win.DesktopID{d(1), d(2), d(3)}
	cases := []struct {
		name string
		cur  win.DesktopID
		want win.DesktopID
		ok   bool
	}{
		{"do meio vai para a anterior", d(2), d(1), true},
		{"a última vai para a anterior", d(3), d(2), true},
		{"a primeira vai para a seguinte", d(1), d(2), true},
		{"área desconhecida", d(9), win.DesktopID{}, false},
	}
	for _, c := range cases {
		got, ok := fallbackDesktop(list, c.cur)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: = %v, %v; want %v, %v", c.name, got, ok, c.want, c.ok)
		}
	}
	if _, ok := fallbackDesktop(list[:1], d(1)); ok {
		t.Error("não deveria excluir a única área")
	}
}
