package multiplayer

import (
	"context"
	core "hyrulelauncher/internal/launcher"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Exercise capture -> running bridge -> TCP server -> remote client -> native
// command. Codec-only tests missed preview 10 dropping Skeleton in Bridge.run.
func TestCapturedBonesReachOtherNativeInstance(t *testing.T) {
	t.Setenv(core.NativePreviewEnv, "1")
	s := testServer(t)
	a, b := testClient(t, s, "A"), testClient(t, s, "B")
	left := encodeSkeleton([]PoseBone{testBone(1, 2)})
	right := encodeSkeleton([]PoseBone{testBone(1, 7)})
	ga := &fakeNativeGame{fakeGame: &fakeGame{position: Vec3{3430, 236, 2100}}, skeleton: left}
	gb := &fakeNativeGame{fakeGame: &fakeGame{position: Vec3{3431, 236, 2100}}, skeleton: right}
	ba := NewBridge(context.Background(), a, ga, 40, nil)
	bb := NewBridge(context.Background(), b, gb, 40, nil)
	t.Cleanup(func() { ba.Close(); bb.Close(); <-ba.Done(); <-bb.Done() })
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		seen := func(g *fakeNativeGame, want string) bool {
			g.mu.Lock()
			defer g.mu.Unlock()
			for _, cmd := range g.commands {
				if strings.HasPrefix(cmd, "hm_native_remote ") && strings.HasSuffix(cmd, " "+want) {
					return true
				}
			}
			return false
		}
		if seen(ga, right) && seen(gb, left) {
			return
		}
		if !a.Connected() || !b.Connected() {
			t.Fatal(a.Reason(), b.Reason())
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("captured skeletal pose never reached the other native instance")
}

func TestDeathEvidenceAcknowledgedAfterCachedZeroWithoutStarvingOtherEntities(t *testing.T) {
	s := testServer(t)
	a, b := testClient(t, s, "A"), testClient(t, s, "B")
	p := Pose{Ready: true, Position: Vec3{1, 2, 3}}
	_ = a.Publish(p)
	_ = b.Publish(p)
	dead := []EnemyHealth{}
	for i := 1; i <= 7; i++ {
		dead = append(dead, EnemyHealth{Hash: uint32(i), Actor: "Enemy_Bokoblin_Junior", Spawn: p.Position, Position: p.Position})
	}
	_ = b.PublishEnemyHealth(dead)
	eventually(t, func() bool { return len(a.SharedEnemyHealth()) == 7 })
	g := &worldControl{enemies: dead}
	bridge := &Bridge{Client: a, control: g}
	n := &nativePreview{session: "0123456789abcdef", sentHealth: map[string]int{}}
	for _, e := range dead {
		n.sentHealth[e.key()] = 0
	} // death hooks can run after these writes
	now := time.Now()
	for cycle, want := range []int{5, 4, 1} {
		g.commands = nil
		n.lastWorld = time.Time{}
		if err := bridge.updateWorldHealth(context.Background(), n, now.Add(time.Duration(cycle)*250*time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		if len(g.commands) != want {
			t.Fatalf("cycle %d: %v", cycle, g.commands)
		}
	}
	g.commands = nil
	n.lastWorld = time.Time{}
	if err := bridge.updateWorldHealth(context.Background(), n, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(g.commands) != 5 {
		t.Fatal("late zero evidence never acknowledged", g.commands)
	}
}

func TestRemoteNeverMixesBodyAndWeaponSnapshots(t *testing.T) {
	start := time.Now()
	a, b := testBone(1, 1), testBone(1, 5)
	gear := func(bone PoseBone, name string) []Equipment {
		e := Equipment{Slot: 0, Name: name}
		for j, v := range bone.Matrix {
			e.Matrix[j] = float64(v)
		}
		return []Equipment{e}
	}
	old := Pose{Ready: true, Skeleton: encodeSkeleton([]PoseBone{a}), Equipment: gear(a, "Weapon_Sword_001")}
	next := Pose{Ready: true, Skeleton: encodeSkeleton([]PoseBone{b}), Equipment: gear(b, "Weapon_Sword_001")}
	c := &Client{connected: true, welcome: Welcome{ID: "self"}}
	check := func(at time.Time, want float32, name string) {
		t.Helper()
		c.frames = []Frame{{At: start, Players: []Player{{ID: "other", Pose: old}}}, {At: start.Add(100 * time.Millisecond), Players: []Player{{ID: "other", Pose: next}}}}
		v := c.Remote(at)
		if len(v) != 1 {
			t.Fatal(v)
		}
		bones, err := decodeSkeleton(v[0].Pose.Skeleton)
		if err != nil || bones[0].Matrix[3] != want || v[0].Pose.Equipment[0].Matrix[3] != float64(want) || v[0].Pose.Equipment[0].Name != name {
			t.Fatal("mixed frames", v, err)
		}
	}
	check(start.Add(100*time.Millisecond), 1, "Weapon_Sword_001") // alpha 0
	check(start.Add(150*time.Millisecond), 3, "Weapon_Sword_001")
	next.Equipment = gear(b, "Weapon_Sword_002")
	check(start.Add(150*time.Millisecond), 5, "Weapon_Sword_002")
	b.Matrix[0] = -1
	b.Matrix[10] = -1
	next.Skeleton = encodeSkeleton([]PoseBone{b})
	next.Equipment = gear(b, "Weapon_Sword_001")
	check(start.Add(150*time.Millisecond), 5, "Weapon_Sword_001") // singular blend selects one entire frame
}

func TestZeroAcknowledgementsRotateBeyondFirstSixteenActors(t *testing.T) {
	s := testServer(t)
	a, b := testClient(t, s, "A"), testClient(t, s, "B")
	p := Pose{Ready: true, Position: Vec3{1, 2, 3}}
	_ = a.Publish(p)
	_ = b.Publish(p)
	dead := []EnemyHealth{}
	for i := 1; i <= 25; i++ {
		dead = append(dead, EnemyHealth{Hash: uint32(i), Actor: "Enemy_Bokoblin_Junior", Spawn: p.Position, Position: p.Position})
	}
	_ = b.PublishEnemyHealth(dead)
	eventually(t, func() bool { return len(a.SharedEnemyHealth()) == 25 })
	g := &worldControl{enemies: dead}
	bridge := &Bridge{Client: a, control: g}
	n := &nativePreview{session: "0123456789abcdef"}
	now := time.Now()
	acked := map[int]bool{}
	for cycle := 0; cycle < 7; cycle++ {
		n.lastWorld = time.Time{}
		g.commands = nil
		if err := bridge.updateWorldHealth(context.Background(), n, now.Add(time.Duration(cycle)*250*time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		for _, cmd := range g.commands {
			if strings.HasPrefix(cmd, "hm_enemy_health ") {
				id, _ := strconv.Atoi(strings.Fields(cmd)[2])
				acked[id] = true
			}
		}
	}
	if len(acked) != 25 {
		t.Fatal("early zero acknowledgements starved other entities", len(acked), acked)
	}
}
