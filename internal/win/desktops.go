//go:build windows

package win

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Áreas de trabalho virtuais.
//
// O Windows só documenta IVirtualDesktopManager, que diz em qual área está
// uma janela (é o que o grid por área usa). Trocar de área e mover janelas
// de outros processos só existe na API interna do Explorer
// (IVirtualDesktopManagerInternal), cujos GUIDs e vtables mudam entre
// versões do Windows. Aqui está a versão do Windows 11 24H2 (build 26100) em
// diante, conferida na build 26300. Se a interface não existir (outra
// versão), QueryService falha, Available() devolve false e só as funções
// documentadas continuam funcionando — nunca chamamos uma vtable de layout
// desconhecido.

// DesktopID identifica uma área de trabalho.
type DesktopID = windows.GUID

var (
	clsidVirtualDesktopManager = mustGUID("{AA509086-5CA9-4C25-8F95-589D3C07B48A}")
	iidVirtualDesktopManager   = mustGUID("{A5CD92FF-29BE-454C-8D04-D82879FB3F1B}")

	clsidImmersiveShell    = mustGUID("{C2F03A33-21F5-47FA-B4BB-156362A2F239}")
	iidServiceProvider     = mustGUID("{6D5140C1-7436-11CE-8034-00AA006009FA}")
	sidDesktopManagerInt   = mustGUID("{C5E0CDCA-7B6E-41B2-9FC4-D93975CC467B}")
	iidDesktopManagerInt   = mustGUID("{53F5CA0B-158F-4124-900C-057158060B27}") // 24H2+
	iidVirtualDesktop      = mustGUID("{3F07F4BE-B107-441A-AF0F-39D82529072C}") // 24H2+
	iidApplicationViewColl = mustGUID("{1841C6D7-4F9D-42C0-AF41-8747538F10E5}")
)

// Slots das vtables (contando os 3 do IUnknown).
const (
	// IVirtualDesktopManager (documentada)
	vdmGetWindowDesktopID = 4

	// IVirtualDesktopManagerInternal (24H2+)
	vdmiMoveViewToDesktop = 4
	vdmiGetCurrentDesktop = 6
	vdmiGetDesktops       = 7
	vdmiSwitchDesktop     = 9
	vdmiFindDesktop       = 14

	// IVirtualDesktop (24H2+)
	vdGetID = 4

	// IObjectArray
	oaGetCount = 3
	oaGetAt    = 4

	// IApplicationViewCollection
	avcGetViewForHwnd = 6
)

// Desktops dá acesso às áreas de trabalho. Deve ser usado só na thread que
// o criou (a do loop de mensagens).
type Desktops struct {
	docs     *comObject // IVirtualDesktopManager
	internal *comObject // IVirtualDesktopManagerInternal; nil se indisponível
	views    *comObject // IApplicationViewCollection; nil se indisponível
}

// OpenDesktops conecta ao Explorer. Nunca devolve nil: o que não estiver
// disponível fica desligado.
func OpenDesktops() *Desktops {
	windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	d := &Desktops{}
	d.connect()
	return d
}

func (d *Desktops) connect() {
	d.release()
	d.docs, _ = coCreate(clsidVirtualDesktopManager, iidVirtualDesktopManager, clsctxInprocServer|clsctxLocalServer)
	sp, err := coCreate(clsidImmersiveShell, iidServiceProvider, clsctxLocalServer)
	if err != nil {
		return
	}
	defer sp.release()
	d.internal, _ = queryService(sp, sidDesktopManagerInt, iidDesktopManagerInt)
	d.views, _ = queryService(sp, iidApplicationViewColl, iidApplicationViewColl)
}

func (d *Desktops) release() {
	d.docs.release()
	d.internal.release()
	d.views.release()
	d.docs, d.internal, d.views = nil, nil, nil
}

// Available diz se dá para trocar de área e mover janelas entre áreas.
func (d *Desktops) Available() bool { return d.internal != nil && d.views != nil }

// retry roda f; se falhar porque o Explorer reiniciou (objetos COM
// desconectados), reconecta e tenta mais uma vez. Outras falhas voltam direto.
func (d *Desktops) retry(f func() error) error {
	err := f()
	if disconnected(err) {
		d.connect()
		err = f()
	}
	return err
}

// WindowDesktop diz em qual área está a janela (API documentada).
func (d *Desktops) WindowDesktop(hwnd uintptr) (DesktopID, bool) {
	var id DesktopID
	err := d.retry(func() error {
		if d.docs == nil {
			return errors.New("IVirtualDesktopManager indisponível")
		}
		if hr := d.docs.call(vdmGetWindowDesktopID, hwnd, uintptr(unsafe.Pointer(&id))); failed(hr) {
			return hrError("GetWindowDesktopId", hr)
		}
		return nil
	})
	return id, err == nil && id != (DesktopID{})
}

// desktopID lê o id de um IVirtualDesktop e o libera.
func desktopID(desk *comObject) (DesktopID, bool) {
	defer desk.release()
	var id DesktopID
	if hr := desk.call(vdGetID, uintptr(unsafe.Pointer(&id))); failed(hr) {
		return id, false
	}
	return id, true
}

// Current devolve a área de trabalho atual.
func (d *Desktops) Current() (DesktopID, bool) {
	if d.internal != nil {
		var id DesktopID
		err := d.retry(func() error {
			if d.internal == nil {
				return errNoInternal
			}
			var desk *comObject
			if hr := d.internal.call(vdmiGetCurrentDesktop, uintptr(unsafe.Pointer(&desk))); failed(hr) || desk == nil {
				return hrError("GetCurrentDesktop", hr)
			}
			var ok bool
			if id, ok = desktopID(desk); !ok {
				return errors.New("IVirtualDesktop.GetId falhou")
			}
			return nil
		})
		if err == nil {
			return id, true
		}
	}
	// Sem a API interna: o Explorer também guarda a área atual no registro.
	ids := registryDesktops("CurrentVirtualDesktop")
	if len(ids) == 0 {
		return DesktopID{}, false
	}
	return ids[0], true
}

// List devolve as áreas de trabalho na ordem da Visão de Tarefas.
func (d *Desktops) List() []DesktopID {
	if d.internal != nil {
		var out []DesktopID
		err := d.retry(func() error {
			out = nil
			if d.internal == nil {
				return errNoInternal
			}
			var arr *comObject
			if hr := d.internal.call(vdmiGetDesktops, uintptr(unsafe.Pointer(&arr))); failed(hr) || arr == nil {
				return hrError("GetDesktops", hr)
			}
			defer arr.release()
			var n uint32
			arr.call(oaGetCount, uintptr(unsafe.Pointer(&n)))
			iid := iidVirtualDesktop
			for i := uint32(0); i < n; i++ {
				var desk *comObject
				if hr := arr.call(oaGetAt, uintptr(i), uintptr(unsafe.Pointer(&iid)),
					uintptr(unsafe.Pointer(&desk))); failed(hr) || desk == nil {
					return hrError("IObjectArray.GetAt", hr)
				}
				if id, ok := desktopID(desk); ok {
					out = append(out, id)
				}
			}
			return nil
		})
		if err == nil && len(out) > 0 {
			return out
		}
	}
	return registryDesktops("VirtualDesktopIDs")
}

// findDesktop devolve o IVirtualDesktop do id (liberar com release).
func (d *Desktops) findDesktop(id DesktopID) (*comObject, error) {
	var desk *comObject
	if hr := d.internal.call(vdmiFindDesktop, uintptr(unsafe.Pointer(&id)), uintptr(unsafe.Pointer(&desk))); failed(hr) || desk == nil {
		return nil, hrError("FindDesktop", hr)
	}
	return desk, nil
}

var errNoInternal = errors.New("trocar/mover entre áreas de trabalho não é suportado nesta versão do Windows")

// Switch vai para a área de trabalho id.
func (d *Desktops) Switch(id DesktopID) error {
	if !d.Available() {
		return errNoInternal
	}
	return d.retry(func() error {
		if !d.Available() {
			return errNoInternal
		}
		desk, err := d.findDesktop(id)
		if err != nil {
			return err
		}
		defer desk.release()
		if hr := d.internal.call(vdmiSwitchDesktop, desk.ptr()); failed(hr) {
			return hrError("SwitchDesktop", hr)
		}
		return nil
	})
}

// MoveWindow leva a janela (de qualquer processo) para a área id.
func (d *Desktops) MoveWindow(hwnd uintptr, id DesktopID) error {
	if !d.Available() {
		return errNoInternal
	}
	return d.retry(func() error {
		if !d.Available() {
			return errNoInternal
		}
		var view *comObject
		if hr := d.views.call(avcGetViewForHwnd, hwnd, uintptr(unsafe.Pointer(&view))); failed(hr) || view == nil {
			return hrError("GetViewForHwnd", hr)
		}
		defer view.release()
		desk, err := d.findDesktop(id)
		if err != nil {
			return err
		}
		defer desk.release()
		if hr := d.internal.call(vdmiMoveViewToDesktop, view.ptr(), desk.ptr()); failed(hr) {
			return hrError("MoveViewToDesktop", hr)
		}
		return nil
	})
}

// registryDesktops lê uma lista de GUIDs gravada pelo Explorer em
// HKCU\...\Explorer\VirtualDesktops.
func registryDesktops(value string) []DesktopID {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Explorer\VirtualDesktops`, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	b, _, err := k.GetBinaryValue(value)
	if err != nil {
		return nil
	}
	var out []DesktopID
	for i := 0; i+16 <= len(b); i += 16 {
		out = append(out, *(*DesktopID)(unsafe.Pointer(&b[i])))
	}
	return out
}
