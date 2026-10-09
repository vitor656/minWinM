# Remove o minWinM instalado por install.ps1: fecha o programa, tira os
# atalhos (Inicializar e menu Iniciar), a entrada em Configurações >
# Aplicativos e apaga %LOCALAPPDATA%\Programs\minWinM.
#
# É o que roda ao desinstalar pelo Configurações (uma cópia fica na pasta de
# instalação). Também dá para rodar direto:
#   powershell -ExecutionPolicy Bypass -File scripts\uninstall.ps1
#   powershell -ExecutionPolicy Bypass -File scripts\uninstall.ps1 -KeepConfig   # guarda o config.json
#   iex (irm https://raw.githubusercontent.com/vitor656/minWinM/main/scripts/uninstall.ps1).TrimStart([char]0xFEFF)

param(
    [switch]$KeepConfig
)

& {
    $ErrorActionPreference = 'Stop'

    $dest = Join-Path $env:LOCALAPPDATA 'Programs\minWinM'
    $lnk = Join-Path ([Environment]::GetFolderPath('Startup')) 'minWinM.lnk'
    $menuLnk = Join-Path ([Environment]::GetFolderPath('Programs')) 'minWinM.lnk'
    $uninstallKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\minWinM'

    # Este script pode estar rodando de dentro da pasta que vai apagar; com
    # ela como diretório atual, o Windows não deixaria apagá-la. O
    # Set-Location só muda o do PowerShell; o do processo é à parte.
    Set-Location ([IO.Path]::GetTempPath())
    [Environment]::CurrentDirectory = [IO.Path]::GetTempPath()

    $running = Get-Process minWinM -ErrorAction SilentlyContinue
    if ($running) {
        $running | Stop-Process -Force
        $running | Wait-Process -Timeout 10 -ErrorAction SilentlyContinue
    }

    foreach ($l in $lnk, $menuLnk) {
        if (Test-Path $l) { Remove-Item $l }
    }
    Write-Host 'Atalhos (Inicializar e menu Iniciar) removidos.'

    if (Test-Path $uninstallKey) {
        Remove-Item $uninstallKey -Recurse
        Write-Host 'Removido de Configurações > Aplicativos.'
    }

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
}
