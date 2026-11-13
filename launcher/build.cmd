@echo off
setlocal
cd /d "%~dp0"
where go >nul 2>nul
if errorlevel 1 (
  echo Instale Go 1.23 ou superior para compilar as fontes.
  exit /b 1
)
if not exist dist mkdir dist
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -buildvcs=false -trimpath -ldflags "-s -w -H=windowsgui" -o dist\HyruleLauncher.exe ./cmd/hyrule
if errorlevel 1 exit /b 1
go build -buildvcs=false -trimpath -ldflags "-s -w" -o dist\HyruleM-Server.exe ./cmd/hyrulem-server
if errorlevel 1 exit /b 1
echo Servidor criado em dist\HyruleM-Server.exe
echo Executavel criado em dist\HyruleLauncher.exe

