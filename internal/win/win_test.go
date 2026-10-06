//go:build windows

package win

import (
	"testing"
	"unsafe"

	"minwinm/internal/icon"
)

// As estruturas passadas por ponteiro à API precisam ter exatamente o
// tamanho das versões em C (x64); um campo a mais ou a menos corrompe a
// chamada sem erro de compilação.
func TestStructSizes(t *testing.T) {
	cases := []struct {
		name      string
		got, want uintptr
	}{
		{"NOTIFYICONDATAW", unsafe.Sizeof(notifyIconData{}), 976},
		{"WNDCLASSEXW", unsafe.Sizeof(wndClassEx{}), 80},
		{"ICONINFO", unsafe.Sizeof(iconInfo{}), 32},
		{"BITMAPINFOHEADER", unsafe.Sizeof(bitmapInfoHeader{}), 40},
		{"MONITORINFO", unsafe.Sizeof(monitorInfoRaw{}), 40},
		{"MSG", unsafe.Sizeof(Msg{}), 48},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d bytes, want %d", c.name, c.got, c.want)
		}
	}
}

// Cria e destrói um HICON de verdade (não mostra nada na tela).
func TestNewIcon(t *testing.T) {
	size := SmallIconSize()
	h, err := NewIcon(size, icon.Pixels(size, icon.ColorActive))
	if err != nil || h == 0 {
		t.Fatalf("NewIcon: %v", err)
	}
	DestroyIcon(h)
	if _, err := NewIcon(size, make([]uint32, 3)); err == nil {
		t.Error("NewIcon aceitou pixels com tamanho errado")
	}
}

// Só leitura: confere que a API interna e a documentada concordam entre si
// e com o registro. Não troca de área nem move janelas.
func TestDesktopsReadOnly(t *testing.T) {
	d := OpenDesktops()
	defer d.release()
	if !d.Available() {
		t.Skip("API interna de áreas de trabalho indisponível nesta versão do Windows")
	}
	cur, ok := d.Current()
	if !ok {
		t.Fatal("Current falhou")
	}
	list := d.List()
	if !containsDesktop(list, cur) {
		t.Fatalf("área atual %v fora da lista %v", cur, list)
	}
	if reg := registryDesktops("VirtualDesktopIDs"); len(reg) > 0 && len(reg) != len(list) {
		t.Errorf("registro tem %d áreas, API interna %d", len(reg), len(list))
	}
	for _, h := range TopLevelWindows() {
		if IsVisible(h) && !IsCloaked(h) && HasTitle(h) {
			if id, ok := d.WindowDesktop(h); ok && !containsDesktop(list, id) {
				t.Errorf("janela %#x numa área desconhecida %v", h, id)
			}
		}
	}
	if _, err := d.findDesktop(DesktopID{}); err == nil {
		t.Error("FindDesktop aceitou id vazio")
	}
}

func containsDesktop(list []DesktopID, id DesktopID) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}

// Passa pelos caminhos de Switch e MoveWindow sem efeito visível: vai para a
// área em que já está e move uma janela para a área onde ela já está.
func TestDesktopsNoOp(t *testing.T) {
	d := OpenDesktops()
	defer d.release()
	if !d.Available() {
		t.Skip("API interna de áreas de trabalho indisponível nesta versão do Windows")
	}
	cur, ok := d.Current()
	if !ok {
		t.Fatal("Current falhou")
	}
	if err := d.Switch(cur); err != nil {
		t.Fatalf("Switch(atual): %v", err)
	}
	if after, _ := d.Current(); after != cur {
		t.Fatalf("Switch(atual) mudou de área: %v -> %v", cur, after)
	}
	for _, h := range TopLevelWindows() {
		if !IsVisible(h) || IsCloaked(h) || !HasTitle(h) || HasOwner(h) {
			continue
		}
		id, ok := d.WindowDesktop(h)
		if !ok || id != cur {
			continue
		}
		if err := d.MoveWindow(h, id); err != nil {
			continue // nem toda janela tem "view" (ex.: algumas do sistema)
		}
		if again, _ := d.WindowDesktop(h); again != cur {
			t.Fatalf("MoveWindow para a mesma área mudou a janela de área")
		}
		return
	}
	t.Skip("nenhuma janela adequada para o teste de MoveWindow")
}
