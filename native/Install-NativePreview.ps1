param([string]$RuntimeDir = '', [string]$ConfigPath = '', [switch]$Ensure)
$ErrorActionPreference = 'Stop'
if (-not $RuntimeDir) {
    if (-not $ConfigPath) { $ConfigPath = Join-Path $env:LOCALAPPDATA 'HyruleLauncher/config.json' }
    if (Test-Path -LiteralPath $ConfigPath -PathType Leaf) {
        try {
            $config = Get-Content -LiteralPath $ConfigPath -Encoding UTF8 -Raw | ConvertFrom-Json
            if ($config.Executable -and (Test-Path -LiteralPath $config.Executable -PathType Leaf)) {
                if ([IO.Path]::GetFileName($config.Executable) -ine 'savagewind.exe') {
                    throw 'Selecione savagewind.exe no launcher antes de instalar a previa nativa.'
                }
                $RuntimeDir = Split-Path -Parent $config.Executable
            }
        } catch { throw "Nao foi possivel ler a instalacao selecionada no launcher: $_" }
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
$source = Join-Path $PSScriptRoot 'runtime-update'
if (-not (Test-Path -LiteralPath (Join-Path $source 'hyrulem-build.json'))) { throw 'Pacote nativo incompleto.' }
$source = (Resolve-Path -LiteralPath $source).Path
$package = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'hyrulem-preview.json') -Encoding UTF8 -Raw | ConvertFrom-Json
$build = Get-Content -LiteralPath (Join-Path $source 'hyrulem-build.json') -Encoding UTF8 -Raw | ConvertFrom-Json
if ($package.schema -ne 1 -or $package.preview_abi -ne 2 -or $build.preview_abi -ne 2) { throw 'Pacote nativo antigo ou incompleto. Extraia o ZIP novo inteiro.' }
if ((Get-FileHash -LiteralPath (Join-Path $source 'savagewind.exe') -Algorithm SHA256).Hash.ToLower() -ne $package.runtime_sha256) { throw 'O runtime do pacote nao corresponde ao manifesto. Extraia o ZIP novo inteiro.' }
$sourceFiles = @(Get-ChildItem -LiteralPath $source -Recurse -File)
function Test-InstalledFiles {
    foreach ($file in $sourceFiles) {
        $relative = $file.FullName.Substring($source.Length).TrimStart([char[]]'\/')
        $target = Join-Path $RuntimeDir $relative
        if (-not (Test-Path -LiteralPath $target -PathType Leaf)) { return $false }
        if ((Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -ne (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash) { return $false }
    }
    return $true
}
if ($Ensure -and (Test-InstalledFiles)) {
    Write-Output "Runtime nativo ABI 2 conferido: $exe"
    return
}
$active = Get-Process -Name savagewind -ErrorAction SilentlyContinue
if ($active) { throw 'Feche os jogos antes de atualizar o runtime experimental.' }
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss-fff'
$backup = Join-Path $RuntimeDir "HyruleM-runtime-backup-$stamp"
New-Item -ItemType Directory -Path $backup | Out-Null
$installed = @()
try {
    foreach ($item in Get-ChildItem -LiteralPath $source) {
        $target = Join-Path $RuntimeDir $item.Name
        if (Test-Path -LiteralPath $target) { Move-Item -LiteralPath $target -Destination (Join-Path $backup $item.Name) }
        $installed += $item.Name
        Copy-Item -LiteralPath $item.FullName -Destination $target -Recurse
    }
    if (-not (Test-InstalledFiles)) { throw 'A copia instalada nao corresponde ao pacote. Restaurando o backup.' }
    # This entry point exits before graphics or game initialization. It proves
    # that the user's existing runtime DLLs can load the new executable.
    $check = Start-Process -FilePath $exe -ArgumentList '--update-health' -WorkingDirectory $RuntimeDir -WindowStyle Hidden -PassThru
    if (-not $check.WaitForExit(15000)) {
        Stop-Process -Id $check.Id -Force
        throw 'O teste do runtime excedeu 15 segundos. Restaurando o backup.'
    }
    if ($check.ExitCode -ne 73) { throw "O runtime nao carregou suas dependencias (codigo $($check.ExitCode)). Restaurando o backup." }
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
Write-Output "Executavel atualizado e conferido (ABI 2): $exe"
Write-Output 'Abra a configuração do Savage Wind e permita reconstruir a biblioteca do jogo. Depois abra Iniciar-Nativo.cmd.'
