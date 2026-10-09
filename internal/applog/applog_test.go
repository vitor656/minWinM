package applog

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setup(t *testing.T) string {
	t.Helper()
	console = io.Discard
	p := filepath.Join(t.TempDir(), "minWinM.log")
	Init(p)
	verbose = false
	t.Cleanup(Close)
	return p
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestOnlyErrorsUnlessVerbose(t *testing.T) {
	p := setup(t)
	Printf("detalhe 1")
	if _, err := os.Stat(p); err == nil {
		t.Fatal("arquivo criado sem nenhuma linha que devesse ir para ele")
	}
	Errorf("falha %d", 1)
	SetVerbose(true)
	Printf("detalhe 2")
	SetVerbose(false)
	Printf("detalhe 3")

	got := read(t, p)
	for _, want := range []string{"[erro] falha 1", "log detalhado ligado", "[info] detalhe 2", "log detalhado desligado"} {
		if !strings.Contains(got, want) {
			t.Errorf("falta %q em:\n%s", want, got)
		}
	}
	for _, not := range []string{"detalhe 1", "detalhe 3"} {
		if strings.Contains(got, not) {
			t.Errorf("%q gravado com o log detalhado desligado", not)
		}
	}
}

func TestRotatesAtMaxSize(t *testing.T) {
	p := setup(t)
	big := strings.Repeat("x", 1000)
	for i := 0; i < MaxSize/1000+10; i++ {
		Errorf("%s", big)
	}
	cur, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	old, err := os.Stat(p + ".old")
	if err != nil {
		t.Fatal("não criou o .old:", err)
	}
	if cur.Size() > MaxSize || old.Size() > MaxSize {
		t.Fatalf("passou do limite: atual %d, old %d", cur.Size(), old.Size())
	}
}

func TestGuardRecovers(t *testing.T) {
	p := setup(t)
	Guard("teste", func() { panic("boom") })
	if got := read(t, p); !strings.Contains(got, "pânico em teste: boom") {
		t.Fatalf("pânico não registrado:\n%s", got)
	}
}
