package config

import (
	"fmt"
	"strings"
)

// Modificadores de atalho, com os mesmos valores do MOD_* do Win32
// (RegisterHotKey e o lParam do WM_HOTKEY usam esses bits).
const (
	ModAlt     = 0x1
	ModControl = 0x2
	ModShift   = 0x4
	ModWin     = 0x8
)

var modNames = map[string]uint32{
	"alt": ModAlt, "ctrl": ModControl, "control": ModControl,
	"shift": ModShift, "win": ModWin,
}

// Virtual-key codes do Win32 para as teclas com nome.
var keyNames = map[string]uint32{
	"left": 0x25, "up": 0x26, "right": 0x27, "down": 0x28,
	"enter": 0x0D, "space": 0x20, "tab": 0x09, "esc": 0x1B,
	"home": 0x24, "end": 0x23, "pageup": 0x21, "pagedown": 0x22,
	"=": 0xBB, "-": 0xBD, // VK_OEM_PLUS / VK_OEM_MINUS (teclas =+ e -_)
}

// ParseBinding transforma "ctrl+alt+left" em (modificadores, virtual-key).
func ParseBinding(s string) (mods, vk uint32, err error) {
	parts := strings.Split(strings.ToLower(strings.ReplaceAll(s, " ", "")), "+")
	key := parts[len(parts)-1]
	for _, p := range parts[:len(parts)-1] {
		m, ok := modNames[p]
		if !ok {
			return 0, 0, fmt.Errorf("modificador desconhecido %q", p)
		}
		mods |= m
	}
	if mods == 0 {
		return 0, 0, fmt.Errorf("precisa de ao menos um modificador")
	}

	switch {
	case keyNames[key] != 0:
		vk = keyNames[key]
	case len(key) == 1 && key[0] >= 'a' && key[0] <= 'z':
		vk = uint32(key[0] - 'a' + 'A')
	case len(key) == 1 && key[0] >= '0' && key[0] <= '9':
		vk = uint32(key[0])
	case len(key) >= 2 && key[0] == 'f':
		var n int
		if _, e := fmt.Sscanf(key[1:], "%d", &n); e == nil && n >= 1 && n <= 12 {
			vk = uint32(0x70 + n - 1)
		}
	}
	if vk == 0 {
		return 0, 0, fmt.Errorf("tecla desconhecida %q", key)
	}
	return mods, vk, nil
}
