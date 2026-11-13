param([string]$RuntimeDir = '')
$ErrorActionPreference = 'Stop'
if (-not $RuntimeDir) {
    $configPath = Join-Path $env:LOCALAPPDATA 'HyruleLauncher/config.json'
    if (Test-Path -LiteralPath $configPath -PathType Leaf) {
        try {
            $config = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
            if ($config.Executable -and (Test-Path -LiteralPath $config.Executable -PathType Leaf)) {
                $RuntimeDir = Split-Path -Parent $config.Executable
            }
        } catch { $RuntimeDir = '' }
    }
}
if (-not $RuntimeDir) {
    Add-Type -AssemblyName System.Windows.Forms
    $picker = New-Object System.Windows.Forms.FolderBrowserDialog
    $picker.Description = 'Selecione a pasta onde esta savagewind.exe'
    if ($picker.ShowDialog() -ne [System.Windows.Forms.DialogResult]::OK) { exit 1 }
    $RuntimeDir = $picker.SelectedPath
    $picker.Dispose()
}
$RuntimeDir = (Resolve-Path -LiteralPath $RuntimeDir).Path
$exe = Join-Path $RuntimeDir 'savagewind.exe'
if (-not (Test-Path -LiteralPath $exe -PathType Leaf)) { throw 'Selecione a pasta da instalação existente do Savage Wind.' }
if (-not (Test-Path -LiteralPath (Join-Path $RuntimeDir 'zig/zig.exe') -PathType Leaf)) { throw 'Instalação incompleta: o compilador Zig não foi encontrado.' }
$active = Get-Process -Name savagewind -ErrorAction SilentlyContinue
if ($active) { throw 'Feche os jogos antes de instalar o runtime experimental.' }
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$backup = Join-Path $RuntimeDir "HyruleM-runtime-backup-$stamp"
$source = Join-Path $PSScriptRoot 'runtime-update'
if (-not (Test-Path -LiteralPath (Join-Path $source 'hyrulem-build.json'))) { throw 'Pacote nativo incompleto.' }
New-Item -ItemType Directory -Path $backup | Out-Null
$installed = @()
try {
    foreach ($item in Get-ChildItem -LiteralPath $source) {
        $target = Join-Path $RuntimeDir $item.Name
        if (Test-Path -LiteralPath $target) { Move-Item -LiteralPath $target -Destination (Join-Path $backup $item.Name) }
        $installed += $item.Name
        Copy-Item -LiteralPath $item.FullName -Destination $target -Recurse
    }
} catch {
    foreach ($name in $installed) {
        $target = Join-Path $RuntimeDir $name
        if (Test-Path -LiteralPath $target) { Remove-Item -LiteralPath $target -Recurse -Force }
        $saved = Join-Path $backup $name
        if (Test-Path -LiteralPath $saved) { Move-Item -LiteralPath $saved -Destination $target }
    }
    throw
}
Write-Output "Runtime experimental instalado. Backup: $backup"
Write-Output 'Abra a configuração do Savage Wind e permita reconstruir a biblioteca do jogo. Depois abra Iniciar-Nativo.cmd.'
