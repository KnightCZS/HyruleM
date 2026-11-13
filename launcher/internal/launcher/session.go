package launcher

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type ProcessInfo struct {
	PID        int
	ParentPID  int
	Executable string
	Started    time.Time
}
type Finder interface {
	Find(executable string, since time.Time) []ProcessInfo
}
type Event struct {
	Kind    string
	Message string
	PID     int
	Code    int
}
type Session struct {
	mu       sync.Mutex
	busy     bool
	finder   Finder
	grace    time.Duration
	interval time.Duration
}

func NewSession(finder Finder) *Session {
	return &Session{finder: finder, grace: 8 * time.Second, interval: 400 * time.Millisecond}
}
func (s *Session) Busy() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.busy }
func (s *Session) Start(spec LaunchSpec, logPath string, notify func(Event)) error {
	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		return fmt.Errorf("já existe uma sessão iniciada pelo launcher")
	}
	if s.finder != nil && len(s.finder.Find(spec.Executable, time.Time{})) > 0 {
		s.mu.Unlock()
		return fmt.Errorf("essa recomp já está em execução")
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		s.mu.Unlock()
		return err
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	cmd := exec.Command(spec.Executable, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	cmd.Stdout = f
	cmd.Stderr = f
	prepareCommand(cmd)
	started := time.Now()
	if err = cmd.Start(); err != nil {
		f.Close()
		s.mu.Unlock()
		return fmt.Errorf("não foi possível iniciar: %w", err)
	}
	if tracker, ok := s.finder.(interface{ TrackPID(int) }); ok {
		tracker.TrackPID(cmd.Process.Pid)
	}
	s.busy = true
	s.mu.Unlock()
	notify(Event{Kind: "started", PID: cmd.Process.Pid, Message: "Programa iniciado. Aguarde o carregamento da recomp."})
	go func() {
		err := cmd.Wait()
		_ = f.Close()
		if s.finder != nil && err == nil {
			notify(Event{Kind: "checking", Message: "Verificando se a recomp reiniciou…"})
			// Some launch options replace the original runtime process.
			deadline := time.Now().Add(s.grace)
			adopted := false
			for time.Now().Before(deadline) {
				candidates := s.finder.Find(spec.Executable, started.Add(-2*time.Second))
				if len(candidates) > 0 {
					adopted = true
					notify(Event{Kind: "restarted", PID: candidates[0].PID, Message: "A recomp reiniciou; acompanhando a sessão."})
					for len(s.finder.Find(spec.Executable, started.Add(-2*time.Second))) > 0 {
						time.Sleep(s.interval)
					}
					break
				}
				time.Sleep(s.interval)
			}
			_ = adopted
		}
		code := 0
		if err != nil {
			code = -1
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			}
		}
		s.mu.Lock()
		s.busy = false
		s.mu.Unlock()
		if err != nil {
			notify(Event{Kind: "failed", Code: code, Message: fmt.Sprintf("O programa encerrou com código %d. Abra os logs para verificar.", code)})
		} else {
			notify(Event{Kind: "ended", Message: "Sessão encerrada. Você pode iniciar novamente."})
		}
	}()
	return nil
}

// WaitIdle is used by diagnostics and tests; it never terminates the game.
func (s *Session) WaitIdle(ctx context.Context) error {
	ticker := time.NewTicker(15 * time.Millisecond)
	defer ticker.Stop()
	for {
		if !s.Busy() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
