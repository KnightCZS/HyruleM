package launcher

import (
	"context"
	"fmt"
	"time"
)

// MultiplayerPreparer serializes shared sandbox/filter preparation for the two
// local clients. Each launch keeps its own private data root and control port.
// The gate is cancellable and must never be held on the window thread.
type MultiplayerPreparer struct {
	gate chan struct{}
	// Running reports any runtime that could be using the shared filter. Tests
	// may supply a process model; production uses the operating-system finder.
	Running func(Config) bool
}

func NewMultiplayerPreparer() *MultiplayerPreparer {
	return &MultiplayerPreparer{gate: make(chan struct{}, 1), Running: func(c Config) bool { return len((SystemFinder{}).Find(c.Executable, time.Time{})) > 0 }}
}
func (p *MultiplayerPreparer) Prepare(ctx context.Context, c Config, slot, port int, env []string, notify func(Progress)) (MultiplayerInstance, error) {
	var result MultiplayerInstance
	if slot < 0 || slot > 1 || port < 1024 || port > 65535 {
		return result, fmt.Errorf("cliente ou porta de controle inválidos")
	}
	select {
	case <-ctx.Done():
		return result, ctx.Err()
	case p.gate <- struct{}{}:
	}
	defer func() { <-p.gate }()
	if e := ctx.Err(); e != nil {
		return result, e
	}
	if e := Validate(c); e != nil {
		return result, e
	}
	if c.Backend != Savage || PreparationNeeded(c) {
		return result, fmt.Errorf("prepare e teste o Savage Wind antes de entrar")
	}
	say := func(s string) {
		if notify != nil {
			notify(Progress{Stage: s})
		}
	}
	running := p.Running != nil && p.Running(c)
	say("Conferindo sandbox compartilhado…")
	if _, e := LoadSandbox(c); e != nil {
		if running {
			return result, fmt.Errorf("feche os jogos para preparar o sandbox inicial: %w", e)
		}
		if _, e = PrepareSandbox(c, false); e != nil {
			return result, e
		}
	}
	if !c.KeepCampaign {
		// A running client can reuse a published layer, but that layer must never
		// be rebuilt underneath it. The next stopped launch checks source/policy.
		if running {
			if _, e := loadFilter(c, ""); e != nil {
				return result, fmt.Errorf("feche os jogos para preparar o filtro de campanha e conecte novamente: %w", e)
			}
			say("Reutilizando filtro com o jogo em execução…")
		} else {
			say("Preparando filtro de campanha para o multiplayer…")
			if _, e := PrepareCampaignFilter(ctx, c, false, notify); e != nil {
				return result, fmt.Errorf("não foi possível preparar o filtro de campanha: %w", e)
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	if nativePreviewRequested(env) {
		say("Preparando ator remoto nativo experimental…")
		if e := prepareNativePreview(c, running); e != nil {
			return result, fmt.Errorf("modo nativo: %w", e)
		}
	}
	say("Preparando dados e save deste cliente…")
	return PrepareMultiplayerInstance(c, slot, port, env)
}
