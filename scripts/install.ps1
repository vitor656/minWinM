# Instala (ou atualiza) o minWinM para rodar em segundo plano:
#   1. compila o executável sem console;
#   2. copia para %LOCALAPPDATA%\Programs\minWinM;
#   3. cria um config.json editável lá (só se ainda não existir);
#   4. cria um atalho no menu Iniciar e outro na pasta Inicializar, para abrir
#      junto com o Windows;
#   5. inicia o minWinM.
#
# Uso (na raiz do projeto):
#   powershell -ExecutionPolicy Bypass -File scripts\install.ps1
#   powershell -ExecutionPolicy Bypass -File scripts\install.ps1 -NoStartup   # sem iniciar com o Windows
#   powershell -ExecutionPolicy Bypass -File scripts\install.ps1 -ResetConfig # troca a config instalada
#                                       pela padrão atual (a antiga vira config.json.bak)

param(
    [switch]$NoStartup,
    [switch]$ResetConfig
)

$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$dest = Join-Path $env:LOCALAPPDATA 'Programs\minWinM'
$exe = Join-Path $dest 'minWinM.exe'
$cfg = Join-Path $dest 'config.json'
$lnk = Join-Path ([Environment]::GetFolderPath('Startup')) 'minWinM.lnk'
$menuLnk = Join-Path ([Environment]::GetFolderPath('Programs')) 'minWinM.lnk'

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw 'Go não encontrado no PATH. Instale em https://go.dev/dl/'
}

# Fecha a versão que estiver rodando, senão o .exe fica travado.
$running = Get-Process minWinM -ErrorAction SilentlyContinue
if ($running) {
    Write-Host 'Fechando o minWinM em execução...'
    $running | Stop-Process -Force
    $running | Wait-Process -Timeout 10 -ErrorAction SilentlyContinue
}

Write-Host "Compilando para $exe ..."
New-Item -ItemType Directory -Force $dest | Out-Null
Push-Location $root
try {
    go build '-ldflags=-H=windowsgui -s -w' -o $exe .
    if ($LASTEXITCODE -ne 0) { throw 'go build falhou' }
}
finally {
    Pop-Location
}

# Config editável ao lado do .exe. Só sobrescreve a do usuário com -ResetConfig
# (útil para receber atalhos novos), guardando a antiga como backup.
$defaultCfg = Join-Path $root 'internal\config\config.json'
if ((Test-Path $cfg) -and $ResetConfig) {
    Copy-Item $cfg "$cfg.bak" -Force
    Copy-Item $defaultCfg $cfg -Force
    Write-Host "Config trocada pela padrão: $cfg (a anterior ficou em $cfg.bak)"
}
elseif (-not (Test-Path $cfg)) {
    Copy-Item $defaultCfg $cfg
    Write-Host "Config criada em $cfg"
}
else {
    Write-Host "Config existente mantida: $cfg (use -ResetConfig para receber atalhos novos)"
}

function New-Shortcut($path) {
    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut($path)
    $shortcut.TargetPath = $exe
    $shortcut.WorkingDirectory = $dest
    $shortcut.Description = 'minWinM - tiling de janelas'
    $shortcut.Save()
}

New-Shortcut $menuLnk
Write-Host "Menu Iniciar: $menuLnk"

if ($NoStartup) {
    if (Test-Path $lnk) { Remove-Item $lnk }
}
else {
    New-Shortcut $lnk
    Write-Host "Inicia com o Windows: $lnk"
}

Start-Process -FilePath $exe -WorkingDirectory $dest
Write-Host ''
Write-Host 'minWinM rodando em segundo plano. Para fechar: Ctrl+Alt+Q.'
Write-Host "Para mudar atalhos: edite $cfg e rode este script de novo (ou feche e abra o minWinM)."
