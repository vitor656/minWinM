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
	"time"
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
	LogVerbose bool              `json:"log_verbose"` // log detalhado no minWinM.log desde o início
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
		if p := UserPath(""); p != "" {
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

// UserPath é o arquivo de configuração que o usuário edita, exista ou não: o
// caminho dado (flag -config) ou config.json ao lado do executável. Vazio, se
// não der para saber onde está o executável.
func UserPath(path string) string {
	if path != "" {
		return path
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "config.json")
}

// EnsureFile cria o arquivo com a configuração padrão se ele ainda não
// existir (nunca sobrescreve), para o usuário ter o que editar. Diz se criou.
func EnsureFile(path string) (bool, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if os.IsExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := f.Write(defaultConfig); err != nil {
		f.Close()
		return true, err
	}
	return true, f.Close()
}

// ModTime é a data de modificação do arquivo (zero se não existe), para
// saber se ele mudou depois de lido.
func ModTime(path string) time.Time {
	st, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return st.ModTime()
}

func parse(data []byte, src string) (Config, string, error) {
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, src, fmt.Errorf("config inválida: %w", err)
	}
	return c, src, nil
}
