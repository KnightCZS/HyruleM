@echo off
setlocal
cd /d "%~dp0"
echo Deixe os dois clientes abertos pelo multiplayer do launcher, sem minimizar.
echo Este diagnostico consulta cada cliente e gera dois relatorios nesta pasta.
echo.
HyruleM-Native-Probe.exe --slot 1
HyruleM-Native-Probe.exe --slot 2
echo.
pause
