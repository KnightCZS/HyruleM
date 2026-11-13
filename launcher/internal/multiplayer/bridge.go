package multiplayer

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	core "hyrulelauncher/internal/launcher"
	"math"
	"sync"
	"time"
)

type GameControl interface {
	Command(context.Context, string) (core.ControlReply, error)
	Close()
}
type Bridge struct {
	Client  *Client
	control GameControl
	cancel  context.CancelFunc
	done    chan struct{}
	mu      sync.Mutex
	camera  Camera
	fov     float64
	status  func(string)
}

func NewBridge(parent context.Context, c *Client, game GameControl, fov float64, status func(string)) *Bridge {
	if !finite(fov) || fov < 10 || fov > 150 {
		fov = 40
	}
	ctx, cancel := context.WithCancel(parent)
	b := &Bridge{Client: c, control: game, cancel: cancel, done: make(chan struct{}), fov: fov, status: status}
	go b.run(ctx)
	return b
}
func (b *Bridge) Close()                { b.cancel(); b.Client.Close() }
func (b *Bridge) Done() <-chan struct{} { return b.done }
func (b *Bridge) SetFOV(v float64) {
	if finite(v) && v >= 10 && v <= 150 {
		b.mu.Lock()
		b.fov = v
		b.mu.Unlock()
	}
}
func (b *Bridge) Camera() Camera { b.mu.Lock(); defer b.mu.Unlock(); return b.camera }
func (b *Bridge) say(s string) {
	if b.status != nil {
		b.status(s)
	}
}
func (b *Bridge) command(ctx context.Context, cmd string, out any) error {
	r, e := b.control.Command(ctx, cmd)
	if e != nil {
		return e
	}
	if out != nil {
		return json.Unmarshal(r.Data, out)
	}
	return nil
}
func wait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
func (b *Bridge) teleport(ctx context.Context) error {
	p := b.Client.Welcome().Spawn
	var reply struct{ State string }
	if e := b.command(ctx, fmt.Sprintf("goto %.4f %.4f %.4f", p.X, p.Y, p.Z), &reply); e != nil {
		return e
	}
	deadline := time.Now().Add(50 * time.Second)
	for {
		switch reply.State {
		case "done", "already_there":
			return nil
		case "failed":
			return fmt.Errorf("o jogo recusou o teleporte")
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("teleporte não confirmou em 50 segundos")
		}
		if !wait(ctx, 500*time.Millisecond) {
			return ctx.Err()
		}
		if e := b.command(ctx, "teleport status", &reply); e != nil {
			return e
		}
	}
}

// Passive optional camera-layout probe for Switch 1.9.0. No memory is written.
// Reject anything inconsistent; manual effective FOV remains available in UI.
func (b *Bridge) probeCamera(ctx context.Context) (float64, Vec3, bool) {
	var cell struct{ Address string }
	if b.command(ctx, "cell camera.eye", &cell) != nil || cell.Address == "" {
		return 0, Vec3{}, false
	}
	var memory struct{ Hex string }
	if b.command(ctx, "mem read "+cell.Address+" 48 hex", &memory) != nil {
		return 0, Vec3{}, false
	}
	raw, e := hex.DecodeString(memory.Hex)
	if e != nil || len(raw) != 48 {
		return 0, Vec3{}, false
	}
	var v [12]float64
	for i := range v {
		v[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:])))
		if !finite(v[i]) {
			return 0, Vec3{}, false
		}
	}
	up := Vec3{v[6], v[7], v[8]}
	fov := v[9]
	if fov < 3 {
		fov *= 180 / math.Pi
	}
	if up.Len() < .95 || up.Len() > 1.05 || fov < 15 || fov > 110 || v[10] <= 0 || v[10] > 20 || v[11] < 10000 || v[11] > 100000 {
		return 0, Vec3{}, false
	}
	var setting struct{ FOV float64 }
	if b.command(ctx, "get fov", &setting) != nil || setting.FOV < 50 || setting.FOV > 200 {
		return 0, Vec3{}, false
	}
	// rt_persp scales the half-angle, not tan(half-angle).
	return math.Min(150, fov*setting.FOV/100), up, true
}
func (b *Bridge) run(ctx context.Context) {
	defer close(b.done)
	defer b.Client.Close()
	defer b.control.Close()
	go func() {
		select {
		case <-b.Client.Done():
			b.cancel()
		case <-ctx.Done():
		}
	}()
	b.say("Conectado. No jogo, escolha Continuar e aguarde o mundo carregar.")
	var previous Vec3
	var at time.Time
	var phase, yaw float64
	joined := false
	stable := 0
	cameraProbe := time.Time{}
	autoFOV := 0.0
	up := Vec3{Y: 1}
	for ctx.Err() == nil && b.Client.Connected() {
		var pos Vec3
		if e := b.command(ctx, "pos", &pos); e != nil || !pos.Valid() {
			_ = b.Client.Publish(Pose{})
			stable = 0
			at = time.Time{}
			b.mu.Lock()
			b.camera = Camera{}
			b.mu.Unlock()
			if !wait(ctx, 700*time.Millisecond) {
				return
			}
			continue
		}
		if !joined {
			stable++
			if stable < 3 {
				if !wait(ctx, 350*time.Millisecond) {
					return
				}
				continue
			}
			b.say("Mundo carregado. Posicionando jogador em Hateno…")
			if e := b.teleport(ctx); e != nil {
				b.say("Falha ao reunir jogadores: " + e.Error() + ". Reconecte quando estiver parado no mundo.")
				return
			}
			// These are private per-client video settings. An owned desktop overlay
			// requires a composited window, and its projection uses full view.
			_ = b.command(ctx, "set fullscreen off", nil)
			_ = b.command(ctx, "set wide_view on", nil)
			_ = b.command(ctx, "set hdr off", nil)
			joined = true
			at = time.Time{}
			b.say("Sincronizando movimento · avatar visual provisório")
			continue
		}
		now := time.Now()
		velocity := Vec3{}
		if !at.IsZero() {
			dt := now.Sub(at).Seconds()
			if dt > .01 && dt < 2 {
				velocity = pos.Sub(previous).Mul(1 / dt)
				if velocity.Len() > 100 {
					velocity = Vec3{}
				}
			}
		}
		speed := math.Hypot(velocity.X, velocity.Z)
		if speed > .15 {
			yaw = math.Atan2(velocity.X, velocity.Z)
			phase = math.Mod(phase+speed*now.Sub(at).Seconds()*2.8, 2*math.Pi)
		}
		previous = pos
		at = now
		if b.Client.Publish(Pose{Position: pos, Velocity: velocity, Yaw: yaw, Phase: phase, Ready: true}) != nil {
			return
		}
		var cam struct{ X, Y, Z, DX, DY, DZ float64 }
		if b.command(ctx, "camera", &cam) == nil {
			if now.Sub(cameraProbe) > 2*time.Second {
				autoFOV, up, _ = b.probeCamera(ctx)
				cameraProbe = now
			}
			b.mu.Lock()
			fov := b.fov
			if autoFOV > 0 {
				fov = autoFOV
			}
			b.camera = Camera{Eye: Vec3{cam.X, cam.Y, cam.Z}, Forward: Vec3{cam.DX, cam.DY, cam.DZ}, Up: up, FOV: fov, At: time.Now(), Calibrated: autoFOV > 0}
			b.mu.Unlock()
		}
		if !wait(ctx, 80*time.Millisecond) {
			return
		}
	}
}
