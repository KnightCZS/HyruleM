@echo off
setlocal
cd /d "%~dp0"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0Install-NativePreview.ps1"
if errorlevel 1 echo A instalacao nao foi concluida. Leia a mensagem acima.
pause
