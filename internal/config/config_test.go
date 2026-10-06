package config

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestParseBinding(t *testing.T) {
	cases := []struct {
		in       string
		mods, vk uint32
	}{
		{"ctrl+alt+left", ModControl | ModAlt, 0x25},
		{"Alt+Shift+H", ModAlt | ModShift, 'H'},
		{"alt + 7", ModAlt, '7'},
		{"control+f12", ModControl, 0x7B},
		{"alt+shift+=", ModAlt | ModShift, 0xBB},
		{"alt+shift+-", ModAlt | ModShift, 0xBD},
		{"win+enter", ModWin, 0x0D},
	}
	for _, c := range cases {
		mods, vk, err := ParseBinding(c.in)
		if err != nil || mods != c.mods || vk != c.vk {
			t.Errorf("ParseBinding(%q) = %#x, %#x, %v; want %#x, %#x", c.in, mods, vk, err, c.mods, c.vk)
		}
	}
	for _, bad := range []string{"h", "hyper+h", "alt+f13", "alt+çç"} {
		if _, _, err := ParseBinding(bad); err == nil {
			t.Errorf("ParseBinding(%q) deveria falhar", bad)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	c, _, err := parse(defaultConfig, "embutido")
	if err != nil {
		t.Fatal(err)
	}
	if c.Gap != 0 || c.Tiling == nil || !*c.Tiling || len(c.Bindings) == 0 {
		t.Fatalf("padrão inesperado: %+v", c)
	}
	for k := range c.Bindings {
		if _, _, err := ParseBinding(k); err != nil {
			t.Errorf("atalho padrão inválido %q: %v", k, err)
		}
	}
}

// O encoding/json aceita chaves repetidas e fica com a última, então um
// atalho duplicado no config.json padrão sumiria sem aviso.
func TestDefaultConfigNoDuplicateBindings(t *testing.T) {
	// Lê só o primeiro nível do objeto e, dentro de "bindings", só as chaves.
	var top map[string]json.RawMessage
	if err := json.Unmarshal(defaultConfig, &top); err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(top["bindings"]))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		t.Fatalf("bindings não é um objeto: %v %v", tok, err)
	}
	seen := map[string]bool{}
	for dec.More() {
		tok, err := dec.Token() // chave
		if err != nil {
			t.Fatal(err)
		}
		key := tok.(string)
		if seen[key] {
			t.Errorf("atalho duplicado no config.json padrão: %q", key)
		}
		seen[key] = true
		var value string
		if err := dec.Decode(&value); err != nil { // valor (nome da ação)
			t.Fatalf("%s: %v", key, err)
		}
	}
	var c Config
	if err := json.Unmarshal(defaultConfig, &c); err != nil {
		t.Fatal(err)
	}
	if len(seen) != len(c.Bindings) || len(seen) == 0 {
		t.Fatalf("leu %d atalhos, a config tem %d", len(seen), len(c.Bindings))
	}
}
