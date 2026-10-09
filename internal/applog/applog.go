// Package applog é o log do minWinM para diagnóstico.
//
// Tudo continua saindo no console (útil com `go run .`). No arquivo:
//   - Errorf grava sempre: falhas são raras, então o arquivo fica pequeno;
//   - Printf (log detalhado) só grava enquanto SetVerbose(true).
//
// O arquivo só é criado na primeira linha gravada e, ao passar de MaxSize,
// vira <arquivo>.old e recomeça — no máximo dois arquivos.
package applog

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sync"
	"time"
)

// MaxSize é o tamanho a partir do qual o arquivo é trocado por um novo.
const MaxSize = 1 << 20 // 1 MB

var (
	mu      sync.Mutex
	path    string
	verbose bool
	file    *os.File
	size    int64
	console io.Writer = os.Stdout
)

// Init define o arquivo de log. Não cria nada: o arquivo só aparece quando a
// primeira linha for gravada. Caminho vazio = só console.
func Init(p string) {
	mu.Lock()
	defer mu.Unlock()
	closeFile()
	path = p
}

// Path devolve o caminho do arquivo de log.
func Path() string {
	mu.Lock()
	defer mu.Unlock()
	return path
}

// SetVerbose liga ou desliga o log detalhado no arquivo.
func SetVerbose(on bool) {
	mu.Lock()
	if verbose == on {
		mu.Unlock()
		return
	}
	verbose = on
	mu.Unlock()
	write("info", "log detalhado "+map[bool]string{true: "ligado", false: "desligado"}[on], true)
}

// Verbose diz se o log detalhado está ligado.
func Verbose() bool {
	mu.Lock()
	defer mu.Unlock()
	return verbose
}

// Printf registra um detalhe: no arquivo, só com o log detalhado ligado.
func Printf(format string, args ...any) {
	write("info", fmt.Sprintf(format, args...), false)
}

// Errorf registra uma falha: vai sempre para o arquivo.
func Errorf(format string, args ...any) {
	write("erro", fmt.Sprintf(format, args...), true)
}

// Guard roda f e, se ela entrar em pânico, registra o pânico com a pilha e
// segue em frente — sem isso, rodando em segundo plano, o programa fecharia
// sem deixar rastro.
func Guard(what string, f func()) {
	defer func() {
		if r := recover(); r != nil {
			Errorf("pânico em %s: %v\n%s", what, r, debug.Stack())
		}
	}()
	f()
}

// Close fecha o arquivo (se aberto).
func Close() {
	mu.Lock()
	defer mu.Unlock()
	closeFile()
}

func write(level, msg string, always bool) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Fprintln(console, msg)
	if path == "" || !(always || verbose) {
		return
	}
	line := time.Now().Format("2006-01-02 15:04:05.000") + " [" + level + "] " + msg + "\n"
	if file == nil && !open() {
		return
	}
	if size > 0 && size+int64(len(line)) > MaxSize {
		closeFile()
		os.Remove(path + ".old")
		os.Rename(path, path+".old")
		if !open() {
			return
		}
	}
	n, _ := file.WriteString(line)
	size += int64(n)
}

// open abre o arquivo para acrescentar linhas. Falhas são ignoradas: o log
// nunca deve atrapalhar o programa.
func open() bool {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return false
	}
	file, size = f, st.Size()
	return true
}

func closeFile() {
	if file != nil {
		file.Close()
		file = nil
	}
}
