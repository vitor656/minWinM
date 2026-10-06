# Remove o minWinM instalado por install.ps1: fecha o programa, tira os
# atalhos (Inicializar e menu Iniciar) e apaga %LOCALAPPDATA%\Programs\minWinM.
#
# Uso:
#   powershell -ExecutionPolicy Bypass -File scripts\uninstall.ps1
#   powershell -ExecutionPolicy Bypass -File scripts\uninstall.ps1 -KeepConfig   # guarda o config.json

param(
    [switch]$KeepConfig
)

$ErrorActionPreference = 'Stop'

$dest = Join-Path $env:LOCALAPPDATA 'Programs\minWinM'
$lnk = Join-Path ([Environment]::GetFolderPath('Startup')) 'minWinM.lnk'
$menuLnk = Join-Path ([Environment]::GetFolderPath('Programs')) 'minWinM.lnk'

$running = Get-Process minWinM -ErrorAction SilentlyContinue
if ($running) {
    $running | Stop-Process -Force
    $running | Wait-Process -Timeout 10 -ErrorAction SilentlyContinue
}

foreach ($l in $lnk, $menuLnk) {
    if (Test-Path $l) { Remove-Item $l }
}
Write-Host 'Atalhos (Inicializar e menu Iniciar) removidos.'

if (Test-Path $dest) {
    if ($KeepConfig) {
        Get-ChildItem $dest -Exclude 'config.json' | Remove-Item -Recurse -Force
        Write-Host "Removido (config mantida em $dest\config.json)."
    }
    else {
        Remove-Item $dest -Recurse -Force
        Write-Host "Removido: $dest"
    }
}

Write-Host 'minWinM desinstalado.'
