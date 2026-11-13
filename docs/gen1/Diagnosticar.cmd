@echo off
setlocal
cd /d "%~dp0"
echo Deixe o Cliente 1 aberto pelo multiplayer do launcher, sem minimizar.
echo Este diagnostico apenas consulta o jogo e gera um relatorio nesta pasta.
echo.
HyruleM-Native-Probe.exe --slot 1
echo.
pause
