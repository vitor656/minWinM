# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Visão geral

minWinM é um gerenciador de janelas em tiles para Windows, em Go, com atalhos globais configuráveis via JSON. Tem dois modos: tiling automático (grid por monitor, padrão) e manual (presets fixos). Código, comentários e mensagens ao usuário estão em português — manter assim. O README é o manual do usuário (atalhos, config); mantenha-o em sincronia ao mudar comportamento.

Módulo `minwinm`; única dependência: `golang.org/x/sys/windows`. `main`, `internal/win` e `internal/wm` têm `//go:build windows`; `internal/grid`, `internal/config`, `internal/icon` e `internal/applog` não dependem do Windows.

## Comandos

    go run .                         # com console; lista quais atalhos registraram/falharam
    go run . -config meu.json        # config alternativa
    go build -ldflags="-H=windowsgui -s -w" -o minWinM.exe .   # sem console, segundo plano
    go generate .                    # só ao mudar ícone/versão: recria winres/*.png e rsrc_windows_amd64.syso
    powershell -ExecutionPolicy Bypass -File scripts\install.ps1   # instala, inicia com o Windows e abre (rodar de novo = atualizar)
    go vet ./...
    go test ./...
    go test -run TestFollowEdges ./internal/grid/
    GOOS=linux go vet ./internal/grid/ ./internal/config/ ./internal/icon/ ./internal/applog/   # garante que seguem sem dependência do Windows

Rodar o binário reorganiza as janelas reais do usuário — não faça isso sem pedir. Não há teste automatizado da interação com o Windows.

## Arquitetura

Dependências num sentido só: `main → wm → grid / win / config` (`win` também usa `grid.Rect`). Tudo roda na thread do loop de mensagens (`runtime.LockOSThread` em main: hotkeys e win-event hooks pertencem à thread que os registrou; os eventos chegam *dentro* do `GetMessage`), então não há concorrência nem locks.

- [main.go](main.go) — garante instância única (`win.SingleInstance`; erros fatais vão para `win.ErrorBox`, porque no build `-H=windowsgui` não há console), carrega a config, registra atalhos (`wm.Lookup` + `config.ParseBinding` + `win.RegisterHotKey`), roda o loop: `WM_HOTKEY` → `Manager.Run`, `WM_TIMER` → `Manager.OnTimer`. `quit` é tratado aqui. Atalhos com Alt/Win chamam `win.MaskModifierRelease` para o app em foco não abrir a barra de menu. Mensagens com `Hwnd != 0` (a janela do ícone) vão para `win.DispatchMessage`; atalhos e timers são mensagens de thread (hwnd 0).
- [tray.go](tray.go) — ícone da área de notificação (`trayUI`): dois HICONs pré-criados (azul = tiling ligado, cinza = desligado), trocados por `refresh()` depois de cada atalho e do menu; menu chama ações via `wm.Lookup` e "Sair" usa `win.PostQuit`. `close()` no `defer` remove o ícone (senão fica um ícone fantasma). Opção `tray_icon` na config.
- [internal/applog](internal/applog) — log de diagnóstico em `minWinM.log` ao lado do .exe (caminho em `main.logPath`). Tudo vai ao console; no arquivo, `Errorf` grava sempre e `Printf` só com o log detalhado (`SetVerbose`: menu do ícone ou `log_verbose`). Arquivo criado só na primeira linha; ao passar de `MaxSize` (1 MB) vira `.old`. `Guard` recupera pânicos e registra a pilha — usado em toda entrada vinda do Windows (atalho, timer, win-event, menu), senão o build sem console morreria sem rastro. No `wm`, `logf` = detalhe e `errorf` = falha. Janelas no log: `describe` (handle, exe, classe), **nunca o título** (dados do usuário); `tile.desc` só é preenchido com o log detalhado ligado.
- [internal/icon](internal/icon) — desenha o ícone (mini-grid) como pixels `0xAARRGGBB`; puro e testável. É a fonte única do ícone: a bandeja usa direto, e `go generate .` (diretivas em [main.go](main.go)) exporta PNGs com [tools/genicon](tools/genicon) e recria `rsrc_windows_amd64.syso` (ícone + VERSIONINFO com FileDescription "minWinM", que é o nome no Gerenciador de Tarefas) via go-winres a partir de [winres/winres.json](winres/winres.json). O `.syso` fica versionado; regenere ao mudar ícone ou versão. Não há RT_MANIFEST de propósito: um manifest com `dpiAwareness` fixaria o modo de DPI e faria o `SetProcessDpiAwarenessContext` (per-monitor v2) de `win.EnablePerMonitorDPI` falhar.
- [internal/config](internal/config) — `config.json` deste diretório é embutido (`go:embed`) e é o padrão. Ordem: flag `-config` → `config.json` ao lado do .exe → embutido. `ParseBinding` devolve bits `Mod*` (mesmos valores do `MOD_*` do Win32) e virtual-key.
- [internal/grid](internal/grid) — matemática pura. `Shape(n)`: ceil(√n) colunas, sobras à direita. `Layout` guarda formato + proporções (`colW`, `rowW[c]`, cada uma soma 1); slots numerados coluna a coluna; quando o formato muda, `FitKeep(prev)` recalcula as proporções a partir da fração antiga de cada janela (`keepSizes`: largura da coluna = média das larguras, alturas relativas mantidas, `normalize` fecha em 1 com ≥ `MinPart`); `Fit(n)` = sem tamanhos conhecidos, tudo igual. `SetShape` fixa um formato diferente de `Shape(n)` enquanto `n` não muda (`Custom()`); `SoloColumn` é a regra pura do `toggle-solo-column`. `Grow` (atalhos, reescala proporcional via `adjust`) e `FollowEdges` (mouse, move só a divisa arrastada via `setBoundary`); cada parte ≥ `MinPart` (10%). `TileRect` aplica `Gaps{Inner, Outer}` (Outer nas bordas da tela; metade de Inner de cada lado de uma divisa — `leadGap`/`trailGap`, que `FollowEdges` usa para inverter a conta). `Neighbor` escolhe o retângulo vizinho por direção. `grid.Rect` tem o layout de memória do RECT do Win32 — não adicionar campos.
- [internal/win](internal/win) — wrappers user32/dwmapi, sem regras do programa. Pontos não óbvios:
  - `Place` posiciona a parte **visível** (compensa a borda invisível do Win10/11 via `DWMWA_EXTENDED_FRAME_BOUNDS`), restaura maximizadas, pula se já está no lugar e chama `SetWindowPos` duas vezes por causa do reescalonamento ao cruzar monitores com DPI diferente.
  - Callbacks (`EnumWindows`, `EnumDisplayMonitors`, win-event) são globais criados uma única vez, porque `syscall.NewCallback` nunca é liberado.
  - `Focus` usa `AttachThreadInput` como fallback quando `SetForegroundWindow` é recusado.
  - [desktops.go](internal/win/desktops.go) + [com.go](internal/win/com.go): áreas de trabalho virtuais via COM cru (objetos como `*comObject`, chamada por slot de vtable — `go vet` reclama de converter `uintptr` em ponteiro, por isso o tipo). `WindowDesktop` usa a API documentada (IVirtualDesktopManager); `Current`/`List`/`Switch`/`MoveWindow`/`Create`/`Remove` usam a interna IVirtualDesktopManagerInternal **da 24H2+** (GUIDs e slots no topo do arquivo, conferidos na build 26300). Se o `QueryService` com esse IID falha (outra versão), `Available()` é false e nunca se chama uma vtable de layout desconhecido; `Current`/`List` caem para o registro (`HKCU\...\Explorer\VirtualDesktops`). `retry` só reconecta em erros de desconexão (Explorer reiniciou), não em qualquer falha. Para suportar outra versão do Windows: novos GUIDs/slots, conferidos com os testes `TestDesktops*` (só leitura ou no-op; `TestDesktopsCreateRemove` cria e exclui uma área vazia e só roda com `MINWINM_TEST_DESKTOP_WRITE=1`).
  - [tray.go](internal/win/tray.go): janela de topo **nunca exibida** (não message-only: precisa receber o broadcast `TaskbarCreated` para recolocar o ícone quando o Explorer reinicia). `ShowMenu` faz `SetForegroundWindow` + `WM_NULL` (contorno documentado para o menu fechar ao clicar fora). Enquanto o menu está aberto, o loop modal do Windows consome os WM_HOTKEY (atalhos são perdidos); o timer de re-tile se repete, então não se perde.
  - [win_test.go](internal/win/win_test.go) confere o `unsafe.Sizeof` das structs passadas à API (NOTIFYICONDATAW, WNDCLASSEXW, ...) — ao mexer nelas, mantenha o teste passando.
- [internal/wm](internal/wm) — o gerenciador (`Manager`). Um `workspace` por **(monitor, área de trabalho virtual)** = lista ordenada de `tile`s + `grid.Layout` embutido. Só os da área atual (`m.desk`, lista em `current()`) são posicionados; `layout` recusa os de outra área, porque as janelas deles estão cloaked e `slots()` refaria as proporções guardadas sem elas. `spaceFor(mon, desk)` cria sob demanda; `sync` re-homeia tiles cuja área mudou por fora. Ações de área em [desktops.go](internal/wm/desktops.go) (`pickDesktop` é a regra pura, testada); depois de trocar/mover chama `focusTop`, porque a troca pela API interna não muda o foco. O grid **não é armazenado**: `slots()` filtra os tiles que ocupam lugar agora, chama `FitKeep` com `tile.frac` (célula de cada um no último `slots()`) e atualiza `tile.frac`; a posição na lista é o slot. Por isso "mover" = trocar posições na lista.
  - [actions.go](internal/wm/actions.go): tabela única nome → `Action` (com `Repeat`); `focus-*`/`move-*` gerados por direção e uma ação por preset. `onFocused` ignora desktop/barra de tarefas. `wm_test.go` (`wantActions`) trava a lista de nomes.
  - [commands.go](internal/wm/commands.go): implementação das ações.
  - [filter.go](internal/wm/filter.go): `isAppWindow` (critério tipo Alt+Tab, candidatos a foco), `tileable` (+ redimensionável + `ignore`), `occupies` (minimizada/cloaked/tela cheia guardam o lugar sem ocupar slot).
  - [events.go](internal/wm/events.go): win-events só agendam um timer (`retileDelay`); o re-tile real é `sync()` (reconcilia monitores/janelas) + `layoutAll()`. `EVENT_SYSTEM_FOREGROUND` → `followFocus`; arrasto sem mudar tamanho → `dropped` (trocar / mudar de monitor).
  - [drag.go](internal/wm/drag.go): `EVENT_SYSTEM_MOVESIZESTART` abre um hook de `EVENT_OBJECT_LOCATIONCHANGE` só na thread da janela; cada movimento chama `FollowEdges` e reposiciona as outras (`layout` nunca posiciona `m.drag.hwnd`).
  - Estados de tile: `floating` (preset com tiling ligado; `toggle-center` é o preset `center`) **ocupa** o slot mas não é posicionado — repetir o mesmo preset (`tile.preset`) devolve ao grid; no modo manual o equivalente é `Manager.saved`. `tall` (toggle-full-height) ocupa a coluna inteira e as vizinhas ficam atrás; `followFocus` passa o `tall` para a vizinha focada. `toggle-solo-column` é a versão sem sobreposição: reordena os tiles + `SetShape`, e guarda em `workspace.solo` a ordem/proporções de antes para desfazer. Use `slotRect`/`slotFrac`, não `Cell`, para o retângulo real de um slot.
  - `applySizes` (usado por atalhos e mouse): aplica, posiciona, confere com `overflows` (janela maior que o slot = tamanho mínimo do app; só conta se o excesso **cresceu** em relação a `before` — `overflowGrew` —, senão uma janela que já não cabia, ex. muitas empilhadas, travaria todo redimensionamento); se transbordou tenta metade e um quarto do caminho e por fim desfaz.
  - Foco/mover usam `grid.Neighbor` sobre os retângulos reais. Exceção: janela centralizada parte do lugar de origem (`centeredHome`) e passa o centro para a vizinha.

### Adicionando ações

- Novo preset: entrada em [internal/grid/presets.go](internal/grid/presets.go) (a ação é criada sozinha) + nome em `wantActions`.
- Outra ação: entrada em `buildActions` ([actions.go](internal/wm/actions.go)), implementação em `commands.go`, nome em `wantActions`, atalho em [internal/config/config.json](internal/config/config.json) e documentação no [README.md](README.md).

## Limitações conhecidas

Ver o fim do [README.md](README.md) (apps elevados, tecla Win, Ctrl+Alt = AltGr em ABNT2, etc.).
