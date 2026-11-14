package multiplayer

import (
	"context"
	"encoding/json"
	"fmt"
	core "hyrulelauncher/internal/launcher"
	"math"
	"strings"
	"testing"
	"time"
)

func TestEnvironmentHostAuthorityLateJoinAndHandoff(t *testing.T) {
	s := testServer(t)
	a := testClient(t, s, "Host")
	eventually(t, func() bool { return a.Environment().HostID == a.Welcome().ID })
	_ = a.Publish(Pose{Ready: true})
	epoch := a.Environment().Epoch
	e := Environment{Epoch: epoch, Sequence: 1, Hour: 23.99, Weather: "rain"}
	if err := a.PublishEnvironment(e); err != nil {
		t.Fatal(err)
	}
	b := testClient(t, s, "Late")
	eventually(t, func() bool {
		v := b.Environment()
		return v.State != nil && v.State.Hour == 23.99 && v.HostID == a.Welcome().ID
	})
	e.Hour = .02
	e.Sequence = 2
	_ = a.PublishEnvironment(e)
	eventually(t, func() bool { return b.Environment().State != nil && b.Environment().State.Hour == .02 })
	// A follower cannot overwrite the world clock even with a valid packet.
	e.Hour = 8
	e.Sequence = 3
	_ = b.PublishEnvironment(e)
	eventually(t, func() bool { return !b.Connected() && s.Count() == 1 })
	if a.Environment().State == nil || a.Environment().State.Hour != .02 {
		t.Fatal("follower changed clock")
	}
	b = testClient(t, s, "Successor")
	a.Close()
	eventually(t, func() bool { return b.Environment().HostID == b.Welcome().ID && b.Environment().Epoch > epoch })
	newEpoch := b.Environment().Epoch
	c := testClient(t, s, "New slot zero")
	eventually(t, func() bool { return c.Environment().HostID == b.Welcome().ID })
	_ = b.Publish(Pose{Ready: true})
	e = Environment{Epoch: newEpoch, Sequence: 1, Hour: 7, Frozen: true, Weather: "snow"}
	_ = b.PublishEnvironment(e)
	eventually(t, func() bool { return c.Environment().State != nil && c.Environment().State.Hour == 7 })
	// A delayed sample from the previous authority epoch is ignored.
	e.Epoch = epoch
	e.Sequence = 500
	e.Hour = 2
	_ = b.PublishEnvironment(e)
	e.Epoch = newEpoch
	e.Sequence = 2
	e.Hour = 9
	_ = b.PublishEnvironment(e)
	eventually(t, func() bool { return c.Environment().State != nil && c.Environment().State.Hour == 9 })
	s.mu.Lock()
	s.environmentAt = time.Now().Add(-4 * time.Second)
	s.mu.Unlock()
	eventually(t, func() bool { return c.Environment().State == nil })
}

type environmentGame struct {
	hour            float64
	frozen          bool
	policy, weather string
	commands        []string
	noWorld         bool
}

func (g *environmentGame) Close() {}
func (g *environmentGame) Command(ctx context.Context, cmd string) (core.ControlReply, error) {
	g.commands = append(g.commands, cmd)
	var data any = map[string]bool{"applied": true}
	switch {
	case cmd == "time":
		data = map[string]any{"hour": g.hour, "hour_held": g.frozen}
		if g.noWorld {
			data = map[string]any{"hour_held": g.frozen}
		}
	case cmd == "hm_native_status":
		data = map[string]bool{"game_paused": false}
	case cmd == "weather":
		data = map[string]string{"weather": g.policy, "now": g.weather}
	case cmd == "time freeze":
		g.frozen = true
	case cmd == "time unfreeze":
		g.frozen = false
	case strings.HasPrefix(cmd, "time set "):
		if _, err := fmt.Sscanf(cmd, "time set %f", &g.hour); err != nil {
			return core.ControlReply{}, err
		}
	case strings.HasPrefix(cmd, "weather "):
		g.policy = strings.TrimPrefix(cmd, "weather ")
		g.weather = g.policy
	default:
		return core.ControlReply{}, fmt.Errorf("unexpected command %s", cmd)
	}
	b, _ := json.Marshal(data)
	return core.ControlReply{OK: true, Data: b}, nil
}
func TestEnvironmentApplyAndRestorePoliciesWithoutRewindingHour(t *testing.T) {
	s := testServer(t)
	a, b := testClient(t, s, "Host"), testClient(t, s, "Follower")
	eventually(t, func() bool { return b.Environment().HostID == a.Welcome().ID })
	_ = a.Publish(Pose{Ready: true})
	e := Environment{Epoch: a.Environment().Epoch, Sequence: 1, Hour: 23.99, Frozen: true, Weather: "heavy_rain"}
	_ = a.PublishEnvironment(e)
	eventually(t, func() bool { return b.Environment().State != nil })
	g := &environmentGame{hour: 1, frozen: false, policy: "no_storms", weather: "cloudy"}
	bridge := &Bridge{Client: b, control: g}
	n := &environmentSync{}
	if err := bridge.updateEnvironment(context.Background(), n, time.Now()); err != nil {
		t.Fatal(err)
	}
	if !g.frozen || g.hour != 23.99 || g.policy != "heavy_rain" {
		t.Fatal("host environment not applied", g)
	}
	bridge.restoreEnvironment(context.Background(), n)
	if g.frozen || g.policy != "no_storms" || g.hour != 23.99 {
		t.Fatal("cleanup changed hour or failed to restore policies", g)
	}
	if clockDistance(23.99, .01) > .021 {
		t.Fatal("midnight compared linearly")
	}
	// If somebody explicitly changes the policy, cleanup leaves their choice.
	n.last = time.Time{}
	if err := bridge.updateEnvironment(context.Background(), n, time.Now()); err != nil {
		t.Fatal(err)
	}
	g.policy = "clear"
	bridge.restoreEnvironment(context.Background(), n)
	if g.policy != "clear" {
		t.Fatal("overwrote manual policy")
	}
	for _, bad := range []Environment{{Epoch: 1, Sequence: 1, Hour: math.NaN(), Weather: "clear"}, {Epoch: 1, Sequence: 1, Hour: 24, Weather: "clear"}, {Epoch: 1, Sequence: 1, Hour: 4, Weather: "rain\nquit"}} {
		if bad.Valid() {
			t.Fatal("invalid environment accepted")
		}
	}
}

func TestAtomicVisualFrameValidatesEquipmentAndAnimationSlots(t *testing.T) {
	p := Pose{Ready: true, Position: Vec3{3434, 236, 2101}, Upper: AnimationTrack{Name: "CutS1", Slot: 1, Sequence: 4, Force: true}}
	for i := 0; i < 6; i++ {
		e := Equipment{Slot: i, Matrix: [12]float64{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0}}
		if i == 0 {
			e.Name = "Weapon_Sword_001"
		}
		if i == 3 {
			e.Name = "Armor_Default_Head"
		}
		p.Equipment = append(p.Equipment, e)
	}
	command, err := nativePoseCommand("0123456789abcdef", 9, p)
	if err != nil || len(strings.Fields(command)) != 94 || len(command) > 4095 || strings.ContainsAny(command, "\r\n") {
		t.Fatal(command, err)
	}
	p.Equipment[0].Name = "Enemy_Bokoblin_Junior"
	if p.Valid() {
		t.Fatal("gameplay actor accepted as visual equipment")
	}
	p.Equipment[0].Name = "Weapon_Sword_001"
	p.Equipment[0].Matrix[0] = math.Inf(1)
	if p.Valid() {
		t.Fatal("invalid transform accepted")
	}
	p.Equipment[0].Matrix[0] = 1
	p.Upper.Slot = 2
	if p.Valid() {
		t.Fatal("invalid AS slot accepted")
	}
}

func TestEnvironmentHostClockInitializationAndUnloadedCleanup(t *testing.T) {
	s := testServer(t)
	host := testClient(t, s, "Host")
	eventually(t, func() bool { return host.Environment().HostID == host.Welcome().ID })
	_ = host.Publish(Pose{Ready: true})
	g := &environmentGame{hour: 11, policy: "game", weather: "clear"}
	b := &Bridge{Client: host, control: g}
	n := &environmentSync{}
	for i := 0; i < 2; i++ {
		n.last = time.Time{}
		if err := b.updateEnvironment(context.Background(), n, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	sets := 0
	for _, cmd := range g.commands {
		if strings.HasPrefix(cmd, "time set ") {
			sets++
		}
	}
	if sets != 1 || g.hour != 11 || g.frozen {
		t.Fatal("initial clock policy changed or gate reset repeated", g.commands)
	}
	g.noWorld = true
	g.frozen = true
	g.policy = "rain"
	n.following = true
	n.originalFrozen = false
	n.lastFrozen = true
	n.originalWeather = "game"
	n.lastWeather = "rain"
	b.restoreEnvironment(context.Background(), n)
	if g.frozen || g.policy != "game" || n.following {
		t.Fatal("unloaded cleanup retained session overrides")
	}
}
func TestVisualInterpolationKeepsSnapshotsAndActionBoundaries(t *testing.T) {
	now := time.Now()
	matrix := [12]float64{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0}
	old := Player{ID: "other", Pose: Pose{Ready: true, Yaw: 3.13, Upper: AnimationTrack{Name: "Wait", Sequence: 1}, Equipment: []Equipment{{Slot: 0, Name: "Weapon_Sword_001", Matrix: matrix}}}}
	next := old
	next.Pose.Yaw = -3.13
	next.Pose.Upper = AnimationTrack{Name: "CutS1", Sequence: 2}
	next.Pose.Equipment = append([]Equipment(nil), old.Pose.Equipment...)
	next.Pose.Equipment[0].Matrix[3] = 4
	c := &Client{connected: true, welcome: Welcome{ID: "self"}, frames: []Frame{{now.Add(-150 * time.Millisecond), []Player{old}}, {now.Add(-50 * time.Millisecond), []Player{next}}}}
	r := c.Remote(now)
	if len(r) != 1 || math.Abs(r[0].Pose.Yaw-math.Pi) > .001 || r[0].Pose.Upper.Name != "Wait" || r[0].Pose.Equipment[0].Matrix[3] != 2 {
		t.Fatal(r)
	}
	r[0].Pose.Equipment[0].Matrix[3] = 99
	if c.frames[1].Players[0].Pose.Equipment[0].Matrix[3] != 4 {
		t.Fatal("mutated stored source snapshot")
	}
}
