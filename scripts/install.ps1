# Instala (ou atualiza) o minWinM para rodar em segundo plano.
#
# Sem clonar o repositório (baixa o .exe da última versão publicada):
#   iex (irm https://raw.githubusercontent.com/vitor656/minWinM/main/scripts/install.ps1).TrimStart([char]0xFEFF)
#   (o TrimStart tira o BOM do arquivo, que o iex não aceita)
#
# No repositório clonado (compila a partir do código; precisa do Go):
#   powershell -ExecutionPolicy Bypass -File scripts\install.ps1
#   ... -NoStartup     # sem iniciar com o Windows
#   ... -ResetConfig   # troca a config instalada pela padrão atual
#                      # (a antiga vira config.json.bak)
#
# O que faz:
#   1. obtém o minWinM.exe (compila ou baixa);
#   2. fecha a versão em execução e copia o exe e o uninstall.ps1 para
#      %LOCALAPPDATA%\Programs\minWinM;
#   3. cria um config.json editável lá (só se ainda não existir);
#   4. cria um atalho no menu Iniciar e outro na pasta Inicializar;
#   5. registra o minWinM em Configurações > Aplicativos, para desinstalar
#      por lá (só do usuário atual, sem precisar de administrador);
#   6. inicia o minWinM.

param(
    [switch]$NoStartup,
    [switch]$ResetConfig
)

# Bloco próprio: rodando via iex, as variáveis não vazam para a sessão.
& {
    $ErrorActionPreference = 'Stop'

    $repo = 'vitor656/minWinM'
    $dest = Join-Path $env:LOCALAPPDATA 'Programs\minWinM'
    $exe = Join-Path $dest 'minWinM.exe'
    $cfg = Join-Path $dest 'config.json'
    $lnk = Join-Path ([Environment]::GetFolderPath('Startup')) 'minWinM.lnk'
    $menuLnk = Join-Path ([Environment]::GetFolderPath('Programs')) 'minWinM.lnk'
    $uninstallKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\minWinM'

    # Rodando de um clone (há go.mod ao lado de scripts\), compila; via iex
    # não há $PSScriptRoot, então baixa a última versão publicada.
    $root = if ($PSScriptRoot) { Split-Path -Parent $PSScriptRoot }
    $fromSource = $root -and (Test-Path (Join-Path $root 'go.mod'))

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ('minWinM-' + [guid]::NewGuid())
    New-Item -ItemType Directory $tmp | Out-Null
    try {
        # Os arquivos a instalar ficam em $tmp antes de mexer na instalação
        # atual: se compilar ou baixar falhar, a versão rodando continua.
        if ($fromSource) {
            if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
                throw 'Go não encontrado no PATH. Instale em https://go.dev/dl/'
            }
            Write-Host 'Compilando a partir do código...'
            Push-Location $root
            try {
                go build '-ldflags=-H=windowsgui -s -w' -o (Join-Path $tmp 'minWinM.exe') .
                if ($LASTEXITCODE -ne 0) { throw 'go build falhou' }
            }
            finally {
                Pop-Location
            }
            Copy-Item (Join-Path $root 'internal\config\config.json') $tmp
            Copy-Item (Join-Path $root 'scripts\uninstall.ps1') $tmp
        }
        else {
            $url = "https://github.com/$repo/releases/latest/download/minWinM.zip"
            Write-Host "Baixando $url ..."
            # O PowerShell 5.1 nem sempre oferece TLS 1.2, exigido pelo GitHub.
            [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
            $zip = Join-Path $tmp 'minWinM.zip'
            Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $zip
            Expand-Archive $zip $tmp -Force
            Get-ChildItem $tmp | Unblock-File
        }

        # Fecha a versão que estiver rodando, senão o .exe fica travado.
        $running = Get-Process minWinM -ErrorAction SilentlyContinue
        if ($running) {
            Write-Host 'Fechando o minWinM em execução...'
            $running | Stop-Process -Force
            $running | Wait-Process -Timeout 10 -ErrorAction SilentlyContinue
        }

        New-Item -ItemType Directory -Force $dest | Out-Null
        Copy-Item (Join-Path $tmp 'minWinM.exe') $exe -Force
        Copy-Item (Join-Path $tmp 'uninstall.ps1') (Join-Path $dest 'uninstall.ps1') -Force
        Write-Host "Instalado em $dest"

        # Config editável ao lado do .exe. Só sobrescreve a do usuário com
        # -ResetConfig (útil para receber atalhos novos), guardando a antiga.
        $defaultCfg = Join-Path $tmp 'config.json'
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
    }
    finally {
        Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
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

    # Entrada em Configurações > Aplicativos > Aplicativos instalados.
    $version = (Get-Item $exe).VersionInfo.ProductVersion
    $sizeKB = [int]((Get-ChildItem $dest -File | Measure-Object Length -Sum).Sum / 1KB)
    $uninstall = "powershell.exe -NoProfile -ExecutionPolicy Bypass -File `"$dest\uninstall.ps1`""
    New-Item -Path $uninstallKey -Force | Out-Null
    $values = @{
        DisplayName          = 'minWinM'
        DisplayVersion       = $version
        Publisher            = 'vitor656'
        DisplayIcon          = $exe
        InstallLocation      = $dest
        URLInfoAbout         = "https://github.com/$repo"
        UninstallString      = $uninstall
        QuietUninstallString = $uninstall
    }
    foreach ($name in $values.Keys) {
        New-ItemProperty -Path $uninstallKey -Name $name -Value $values[$name] -PropertyType String -Force | Out-Null
    }
    foreach ($name in 'NoModify', 'NoRepair') {
        New-ItemProperty -Path $uninstallKey -Name $name -Value 1 -PropertyType DWord -Force | Out-Null
    }
    New-ItemProperty -Path $uninstallKey -Name EstimatedSize -Value $sizeKB -PropertyType DWord -Force | Out-Null
    Write-Host "Registrado em Configurações > Aplicativos (versão $version)"

    Start-Process -FilePath $exe -WorkingDirectory $dest
    Write-Host ''
    Write-Host 'minWinM rodando em segundo plano. Para fechar: Ctrl+Alt+Q.'
    Write-Host "Para mudar atalhos: edite $cfg e reabra o minWinM."
    Write-Host 'Para desinstalar: Configurações > Aplicativos > Aplicativos instalados > minWinM.'
}
