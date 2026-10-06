// Package config carrega o config.json do minWinM e interpreta os atalhos.
//
// O config.json deste diretório é embutido no executável e vale quando o
// usuário não fornece um próprio.
package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed config.json
var defaultConfig []byte

// Config espelha o config.json.
type Config struct {
	Gap        int               `json:"gap"`         // pixels entre janelas vizinhas
	OuterGap   *int              `json:"outer_gap"`   // pixels nas bordas da tela (padrão: igual a gap)
	Tiling     *bool             `json:"tiling"`      // grid automático ao iniciar (padrão true)
	ResizeStep float64           `json:"resize_step"` // fração da tela por grow/shrink (padrão 0.05)
	Ignore     []string          `json:"ignore"`      // executáveis ou classes de janela fora do grid
	TrayIcon   *bool             `json:"tray_icon"`   // ícone na área de notificação (padrão true)
	Bindings   map[string]string `json:"bindings"`    // "atalho": "ação"
}

// Load usa, nessa ordem: o caminho dado (flag -config), config.json ao lado
// do executável, ou a configuração embutida. Devolve também de onde leu.
func Load(path string) (Config, string, error) {
	var data []byte
	var src string
	var err error

	switch {
	case path != "":
		data, err = os.ReadFile(path)
		src = path
	default:
		if exe, e := os.Executable(); e == nil {
			p := filepath.Join(filepath.Dir(exe), "config.json")
			if d, e := os.ReadFile(p); e == nil {
				data, src = d, p
			}
		}
		if data == nil {
			data, src = defaultConfig, "(padrão embutido)"
		}
	}
	if err != nil {
		return Config{}, src, err
	}
	return parse(data, src)
}

func parse(data []byte, src string) (Config, string, error) {
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, src, fmt.Errorf("config inválida: %w", err)
	}
	return c, src, nil
}
