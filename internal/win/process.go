//go:build windows

package win

import "golang.org/x/sys/windows"

// SingleInstance cria um mutex com o nome dado e devolve false se outra
// instância do programa já o criou. O mutex vive até o processo terminar.
func SingleInstance(name string) bool {
	n, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return true
	}
	_, err = windows.CreateMutex(nil, false, n)
	return err != windows.ERROR_ALREADY_EXISTS
}

// ErrorBox mostra uma caixa de erro — o único jeito de avisar o usuário
// quando o programa roda sem console (build com -H=windowsgui).
func ErrorBox(title, text string) {
	t, _ := windows.UTF16PtrFromString(title)
	x, _ := windows.UTF16PtrFromString(text)
	windows.MessageBox(0, x, t, windows.MB_OK|windows.MB_ICONERROR)
}
