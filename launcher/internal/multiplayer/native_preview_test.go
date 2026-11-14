package multiplayer

import (
	"context"
	"encoding/json"
	core "hyrulelauncher/internal/launcher"
	"math"
	"strings"
	"testing"
	"time"
)

func TestNativeCommandsRejectInvalidTelemetry(t *testing.T) {
	pose := Pose{Ready: true, Position: Vec3{3434.5, 235.9, 2101.1}, Velocity: Vec3{X: 2}, Yaw: 1}
	command, err := nativePoseCommand("0123456789abcdef", 42, pose)
	if err != nil || !strings.Contains(command, " move ") || len(command) > 256 || strings.ContainsAny(command, "\r\n") {
		t.Fatal(command, err)
	}
	for _, bad := range []Pose{{Ready: false}, {Ready: true, Yaw: math.NaN()}, {Ready: true, Position: Vec3{X: math.Inf(1)}}} {
		if _, err := nativePoseCommand("0123456789abcdef", 1, bad); err == nil {
			t.Fatal("invalid pose accepted")
		}
	}
	if _, err := nativePoseCommand("0123456789abcd\nx", 1, pose); err == nil {
		t.Fatal("session injection accepted")
	}
}

type fakeNativeGame struct {
	*fakeGame
	active bool
}

type legacyNativeGame struct{ *fakeNativeGame }

type waitingNativeGame struct {
	*fakeNativeGame
	status json.RawMessage
}

func (f *waitingNativeGame) Command(ctx context.Context, cmd string) (core.ControlReply, error) {
	if cmd == "hm_native_status" {
		return core.ControlReply{OK: true, Data: f.status}, nil
	}
	return f.fakeNativeGame.Command(ctx, cmd)
}

func TestNativeActivationWaitAndFallbackDeadline(t *testing.T) {
	s := testServer(t)
	a, remote := testClient(t, s, "Local"), testClient(t, s, "Remote")
	if err := remote.Publish(Pose{Ready: true, Position: Vec3{3400, 236, 2100}}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(a.Remote(time.Now())) > 0 })
	game := &waitingNativeGame{fakeNativeGame: &fakeNativeGame{fakeGame: &fakeGame{}}, status: json.RawMessage(`{"state":"activating","tick_age_ms":0,"spawn_attempts":2}`)}
	b := &Bridge{Client: a, control: game}
	now := time.Now()
	n := &nativePreview{session: "0123456789abcdef", waitingSince: now.Add(-19 * time.Second), spawnAttempts: 1}
	if err := b.updateNative(context.Background(), n, now); err != nil {
		t.Fatal("fallback interrupted before activation", err)
	}
	n.waitingSince = now.Add(-19 * time.Second)
	n.lastStatus = time.Time{}
	if err := b.updateNative(context.Background(), n, now); err == nil {
		t.Fatal("repeated status extended the deadline")
	}
	game.status = json.RawMessage(`{"state":"active","tick_age_ms":0,"spawn_attempts":2,"active":true,"model_ready":true,"animation_ready":true}`)
	n.lastStatus = time.Time{}
	if err := b.updateNative(context.Background(), n, now); err != nil {
		t.Fatal(err)
	}
	game.status = json.RawMessage(`{"state":"activating","tick_age_ms":0,"spawn_attempts":2}`)
	n.lastStatus = time.Time{}
	if err := b.updateNative(context.Background(), n, now); err != nil {
		t.Fatal("brief reactivation used original session deadline", err)
	}
}

func (f *legacyNativeGame) Command(ctx context.Context, cmd string) (core.ControlReply, error) {
	if cmd == "hm_caps" {
		return core.ControlReply{OK: true, Data: json.RawMessage(`{"runtime":"SavageWind-HyruleM","game_version":"1.9.0","preview_abi":1,"native_preview":true}`)}, nil
	}
	return f.fakeNativeGame.Command(ctx, cmd)
}

func TestOldRuntimeRejectionSurvivesDisconnectCleanup(t *testing.T) {
	t.Setenv(core.NativePreviewEnv, "1")
	s := testServer(t)
	c := testClient(t, s, "Local")
	game := &legacyNativeGame{&fakeNativeGame{fakeGame: &fakeGame{position: Vec3{3400, 236, 2100}}}}
	b := NewBridge(context.Background(), c, game, 40, nil)
	defer b.Close()
	select {
	case <-b.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("old runtime was not rejected")
	}
	if c.Connected() || !strings.Contains(c.Reason(), "ABI 1") || !strings.Contains(c.Reason(), "ABI 6") {
		t.Fatal("specific native failure replaced by generic disconnection", c.Reason())
	}
}

func (f *fakeNativeGame) Command(ctx context.Context, cmd string) (core.ControlReply, error) {
	if !strings.HasPrefix(cmd, "hm_") {
		return f.fakeGame.Command(ctx, cmd)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commands = append(f.commands, cmd)
	var data any = map[string]bool{"queued": true}
	switch {
	case cmd == "hm_caps":
		data = map[string]any{"runtime": "SavageWind-HyruleM", "game_version": "1.9.0", "preview_abi": 6, "native_preview": true, "shared_enemy_health": true, "animation_capture": true, "actor_job_barrier": true}
	case cmd == "hm_native_status":
		data = map[string]any{"state": "idle", "game_ticks": 12, "tick_age_ms": 0, "active": f.active, "model_ready": f.active, "animation_ready": f.active}
	case cmd == "hm_local_state":
		data = map[string]any{"yaw": 0, "upper": "WaitUpper", "lower": "Wait", "ready": true, "x": f.position.X, "y": f.position.Y, "z": f.position.Z}
	case cmd == "hm_world_status":
		data = map[string]string{"enemies_json": "[]"}
	case strings.HasPrefix(cmd, "hm_native_remote "):
		f.active = true
	case strings.HasPrefix(cmd, "hm_native_clear "):
		f.active = false
	}
	b, _ := json.Marshal(data)
	return core.ControlReply{OK: true, Data: b}, nil
}
func TestNativeJoiningDoesNotTeleportButExplicitGatherDoes(t *testing.T) {
	t.Setenv(core.NativePreviewEnv, "1")
	s := testServer(t)
	a := testClient(t, s, "Local")
	b := testClient(t, s, "Remote")
	position := Vec3{3400, 236, 2100}
	game := &fakeNativeGame{fakeGame: &fakeGame{position: position}}
	_ = b.Publish(Pose{Ready: true, Position: position.Add(Vec3{X: 10})})
	bridge := NewBridge(context.Background(), a, game, 40, nil)
	defer bridge.Close()
	eventually(t, func() bool { game.mu.Lock(); defer game.mu.Unlock(); return game.active })
	game.mu.Lock()
	for _, cmd := range game.commands {
		if strings.HasPrefix(cmd, "goto ") {
			t.Error("join moved the player", cmd)
		}
	}
	got := game.position
	game.mu.Unlock()
	if got != position {
		t.Fatal("local pose changed on join")
	}
	bridge.RequestSpawn()
	eventually(t, func() bool {
		game.mu.Lock()
		defer game.mu.Unlock()
		return game.position.Sub(a.Welcome().Spawn).Len() < 0.001
	})
	bridge.Close()
	select {
	case <-bridge.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("native cleanup stalled")
	}
}
