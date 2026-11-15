package multiplayer

import (
	"context"
	core "hyrulelauncher/internal/launcher"
	"strings"
	"testing"
	"time"
)

func TestAdaptiveDelayKeepsLanNearOneSnapshot(t *testing.T) {
	start := time.Unix(100, 0)
	frames := make([]Frame, 6)
	for i := range frames {
		frames[i].At = start.Add(time.Duration(i) * 20 * time.Millisecond)
	}
	if got := presentationDelay(frames); got != 25*time.Millisecond {
		t.Fatal(got)
	}
	c := &Client{connected: true, welcome: Welcome{ID: "self"}, frames: frames}
	for i := range c.frames {
		c.frames[i].Players = []Player{{ID: "other", Pose: Pose{Ready: true, Position: Vec3{X: float64(i) * .2}}}}
	}
	r := c.Remote(start.Add(100 * time.Millisecond))
	if len(r) != 1 || r[0].Pose.Position.X < .7499 || r[0].Pose.Position.X > .7501 {
		t.Fatal(r)
	}
	frames[5].At = frames[4].At.Add(90 * time.Millisecond)
	if got := presentationDelay(frames); got <= 25*time.Millisecond || got > 100*time.Millisecond {
		t.Fatal("jitter reserve", got)
	}
}

func TestPiecePalettesDoNotMixSameNamedBones(t *testing.T) {
	body, head := testBone(42, 1), testBone(42, 10)
	head.Part = 4
	old := encodeSkeleton([]PoseBone{body, head})
	body.Matrix[3] = 3
	head.Matrix[3] = 20
	next := encodeSkeleton([]PoseBone{head, body})
	encoded, snap := interpolateSkeleton(old, next, .5)
	bones, err := decodeSkeleton(encoded)
	if err != nil || snap || len(bones) != 2 || bones[0].Part != 4 || bones[0].Matrix[3] != 15 || bones[1].Matrix[3] != 2 {
		t.Fatal(bones, snap, err)
	}
	for _, part := range []uint32{1, 2, 3, 7, ^uint32(0)} {
		head.Part = part
		if _, err := decodeSkeleton(encodeSkeleton([]PoseBone{head})); err == nil {
			t.Fatal("invalid part accepted", part)
		}
	}
}

func TestNativeEffectFrameBoundsAndClock(t *testing.T) {
	effect := VisualEffect{Sequence: 3, Slot: 1, Active: true, Name: "AttackSpin"}
	pose := Pose{Ready: true, Effects: []VisualEffect{effect}}
	cmd, err := nativePoseCommand("0123456789abcdef", 1, pose)
	if err != nil || !strings.HasSuffix(cmd, " - 1 3 1 1 0 AttackSpin") {
		t.Fatal(cmd, err)
	}
	for _, bad := range []VisualEffect{{Sequence: 0, Name: "AttackSpin"}, {Sequence: 1, Slot: 4, Name: "AttackSpin"}, {Sequence: 1, Name: "Attack Spin"}, {Sequence: 1, Name: "AttackSpin", AgeMS: 5001}} {
		if validEffects([]VisualEffect{bad}) {
			t.Fatal("invalid effect", bad)
		}
	}
	if validEffects([]VisualEffect{effect, effect}) {
		t.Fatal("duplicate effect")
	}
	start := time.Unix(200, 0)
	a, b := testBone(1, 0), testBone(1, 2)
	c := &Client{connected: true, frames: []Frame{{At: start, Players: []Player{{ID: "other", Pose: Pose{Ready: true, Skeleton: encodeSkeleton([]PoseBone{a})}}}}, {At: start.Add(100 * time.Millisecond), Players: []Player{{ID: "other", Pose: Pose{Ready: true, Skeleton: encodeSkeleton([]PoseBone{b}), Effects: []VisualEffect{effect}}}}}}}
	if r := c.Remote(start.Add(150 * time.Millisecond)); len(r) != 1 || len(r[0].Pose.Effects) != 0 {
		t.Fatal("event ran ahead of body", r)
	}
	if r := c.Remote(start.Add(200 * time.Millisecond)); len(r) != 1 || len(r[0].Pose.Effects) != 1 {
		t.Fatal("event lost", r)
	}
}
func TestNativeSampleCarriesEffectsInOneRoundtrip(t *testing.T) {
	t.Setenv(core.NativePreviewEnv, "1")
	s := testServer(t)
	a, b := testClient(t, s, "A"), testClient(t, s, "B")
	fx := VisualEffect{Sequence: 7, Slot: 0, Active: true, Name: "AttackSpin"}
	bone := testBone(7, 2)
	bone.Part = 4
	ga := &fakeNativeGame{fakeGame: &fakeGame{position: Vec3{3430, 236, 2100}}, skeleton: encodeSkeleton([]PoseBone{bone}), effects: []VisualEffect{fx}}
	gb := &fakeNativeGame{fakeGame: &fakeGame{position: Vec3{3431, 236, 2100}}, skeleton: encodeSkeleton([]PoseBone{testBone(1, 1)})}
	ba := NewBridge(context.Background(), a, ga, 40, nil)
	bb := NewBridge(context.Background(), b, gb, 40, nil)
	t.Cleanup(func() { ba.Close(); bb.Close(); <-ba.Done(); <-bb.Done() })
	eventually(t, func() bool {
		gb.mu.Lock()
		defer gb.mu.Unlock()
		for _, cmd := range gb.commands {
			if strings.HasPrefix(cmd, "hm_native_sample ") && strings.Contains(cmd, " AttackSpin") {
				return true
			}
		}
		return false
	})
	gb.mu.Lock()
	defer gb.mu.Unlock()
	// Once the world is joined, remote submission and local sampling share a
	// single command rather than two commands each waiting for a displayed frame.
	seen := false
	for _, cmd := range gb.commands {
		if strings.HasPrefix(cmd, "hm_native_sample ") {
			seen = true
		}
		if seen && (cmd == "hm_local_state" || strings.HasPrefix(cmd, "hm_native_remote ")) {
			t.Fatal("split roundtrip after sample", cmd)
		}
	}
}

func TestSnapshotDoesNotEchoOwnCosmeticsOrMutateAuthority(t *testing.T) {
	p := Pose{Ready: true, Skeleton: encodeSkeleton([]PoseBone{testBone(1, 1)}), Effects: []VisualEffect{{Sequence: 1, Name: "AttackSpin"}}}
	players := []Player{{ID: "A", Pose: p}, {ID: "B", Pose: p}}
	a, b := recipientPlayers(players, "A"), recipientPlayers(players, "B")
	if a[0].Pose.Skeleton != "" || len(a[0].Pose.Effects) != 0 || a[1].Pose.Skeleton != p.Skeleton || b[1].Pose.Skeleton != "" || b[0].Pose.Skeleton != p.Skeleton || players[0].Pose.Skeleton != p.Skeleton || players[1].Pose.Skeleton != p.Skeleton {
		t.Fatal(a, b, players)
	}
}
