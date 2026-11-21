package launcher

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func ensureNativeLibrary(ctx context.Context, c Config, running bool, notify func(Progress)) error {
	spec, err := BuildLaunch(c, false, os.Environ())
	if err != nil {
		return err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	probe := exec.CommandContext(probeCtx, c.Executable, "--hyrulem-gamelib-id")
	prepareCommand(probe)
	probe.Dir = spec.Dir
	probe.Env = spec.Env
	output, err := probe.Output()
	if err != nil {
		return fmt.Errorf("não foi possível conferir a biblioteca privada: %w", err)
	}
	expected := strings.TrimSpace(string(output))
	if len(expected) != 16 || strings.IndexFunc(expected, func(r rune) bool { return !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') }) >= 0 {
		return fmt.Errorf("runtime sem identificação da biblioteca; reinstale o pacote nativo atualizado")
	}
	gameDir := filepath.Join(c.DataDir, "extracted")
	library := filepath.Join(gameDir, "swgame.dll")
	marker := library + ".id"
	id, readErr := os.ReadFile(marker)
	if readErr == nil && strings.TrimSpace(string(id)) == expected && IsFile(library) {
		return nil
	}
	if running {
		return fmt.Errorf("feche os jogos desta instalação para recompilar a biblioteca privada")
	}
	if notify != nil {
		notify(Progress{Stage: "Recompilando a biblioteca privada do jogo para o runtime atualizado…"})
	}
	logDir := filepath.Join(c.DataDir, "hyrulem")
	if err = os.MkdirAll(logDir, 0700); err != nil {
		return err
	}
	logPath := filepath.Join(logDir, "native-build.log")
	log, err := os.Create(logPath)
	if err != nil {
		return err
	}
	build := exec.CommandContext(ctx, c.Executable, "--hyrulem-build-game", gameDir)
	prepareNativeBuild(build)
	build.Dir = spec.Dir
	build.Env = spec.Env
	build.Stdout = log
	build.Stderr = log
	err = build.Run()
	closeErr := log.Close()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("a recompilação privada falhou: %w. Relatório em %s", err, logPath)
	}
	if closeErr != nil {
		return closeErr
	}
	id, err = os.ReadFile(marker)
	if err != nil || strings.TrimSpace(string(id)) != expected || !IsFile(library) {
		return fmt.Errorf("a recompilação não produziu uma biblioteca compatível; confira %s", logPath)
	}
	return nil
}
