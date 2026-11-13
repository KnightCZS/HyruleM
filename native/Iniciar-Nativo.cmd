@echo off
setlocal
cd /d "%~dp0"
set HYRULEM_NATIVE_PREVIEW=1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0Install-NativePreview.ps1" -Ensure
if errorlevel 1 (
    echo A instalacao nativa precisa ser corrigida antes de abrir o launcher.
    pause
    exit /b 1
)
start "" "%~dp0HyruleLauncher.exe"
