@echo off
setlocal
cd /d "%~dp0"
go test -count=1 ./internal/gen1 ./internal/nativebridge
if errorlevel 1 exit /b 1
go build -trimpath -ldflags "-s -w" -o dist/HyruleM-Gen1-Server.exe ./cmd/hyrulem-gen1-server
if errorlevel 1 exit /b 1
go build -trimpath -ldflags "-s -w" -o dist/HyruleM-Native-Probe.exe ./cmd/hyrulem-native-probe
if errorlevel 1 exit /b 1
echo Ferramentas de desenvolvimento criadas em dist. Backend BOTW nativo ainda pendente.
