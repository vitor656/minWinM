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

// OSBuild devolve o número de build do Windows (ex.: 26100), para o log.
func OSBuild() uint32 {
	return windows.RtlGetVersion().BuildNumber
}

// OpenFile abre o arquivo no programa padrão (o Bloco de Notas, para .log).
func OpenFile(path string) error {
	allowForeground()
	verb, _ := windows.UTF16PtrFromString("open")
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}

// allowForeground deixa qualquer processo tomar o foco. Sem isso, um editor
// que já está aberto (VS Code, Bloco de Notas com abas) recebe o arquivo da
// instância nova mas não consegue vir para frente — parece que nada abriu.
// Só funciona porque o minWinM acabou de ter o foco (menu do ícone).
func allowForeground() {
	const asfwAny = ^uintptr(0) // ASFW_ANY = (DWORD)-1
	pAllowSetForeground.Call(asfwAny)
}

// EditFile abre o arquivo no programa padrão para o tipo e, se não houver um
// (comum com .json), no Bloco de Notas.
func EditFile(path string) error {
	if OpenFile(path) == nil {
		return nil
	}
	verb, _ := windows.UTF16PtrFromString("open")
	notepad, _ := windows.UTF16PtrFromString("notepad.exe")
	arg, err := windows.UTF16PtrFromString(`"` + path + `"`)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, notepad, arg, nil, windows.SW_SHOWNORMAL)
}
