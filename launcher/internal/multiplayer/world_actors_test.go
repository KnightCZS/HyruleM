package multiplayer

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	core "hyrulelauncher/internal/launcher"
	"math"
	"strings"
	"testing"
	"time"
)

func testWorldActor(id uint64, name string, x float32) WorldActor {
	a := WorldActor{ID: id, Hash: uint32(id + 100), Actor: name, Kind: worldActorKind(name), Spawn: Vec3{float64(x), 236, 2100}}
	a.Matrix = [12]float32{1, 0, 0, x, 0, 1, 0, 236, 0, 0, 1, 2100}
	a.Tracks[0] = AnimationTrack{Name: "Walk", Sequence: id, Force: true}
	return a
}
func testWorldFrame(epoch, seq uint64) WorldState {
	return WorldState{Epoch: epoch, Sequence: seq, Center: Vec3{3430, 236, 2100}, Complete: true, Actors: encodeWorldActors([]WorldActor{testWorldActor(1, "Enemy_Bokoblin_Junior", 3430), testWorldActor(2, "Npc_Test", 3431), testWorldActor(3, "Obj_BarrelOld_A_01", 3432), testWorldActor(4, "Weapon_Sword_001", 3433)})}
}
func TestWorldActorBinaryAndKinds(t *testing.T) {
	frame := testWorldFrame(1, 1)
	if !frame.Valid() {
		t.Fatal("valid map actors rejected")
	}
	rows, err := decodeWorldActors(frame.Actors)
	if err != nil || len(rows) != 4 || rows[1].Kind != 2 || rows[2].Tracks[0].Name != "Walk" {
		t.Fatal(rows, err)
	}
	for _, name := range []string{"GameROMPlayer", "HyruleM_RemoteLink", "Enemy_Ganon", "Enemy_SiteBoss_Sword", "Armor_001_Upper", "Weapon_Sword_070", "Weapon_Sword_502"} {
		if worldActorKind(name) != 0 {
			t.Fatal("unsupported actor", name)
		}
	}
	for _, mutate := range []func([]byte){func(b []byte) { b[0] = 'X' }, func(b []byte) { b[6] = 1 }, func(b []byte) { binary.LittleEndian.PutUint64(b[8:], 0) }, func(b []byte) { binary.LittleEndian.PutUint32(b[8+12:], 2) }, func(b []byte) { binary.LittleEndian.PutUint32(b[8+92:], math.Float32bits(float32(math.NaN()))) }, func(b []byte) { b[8+16+63] = 'x' }, func(b []byte) { copy(b[8+300:8+600], b[8:8+300]) }, func(b []byte) { binary.LittleEndian.PutUint32(b[8+140+72:], 2) }} {
		raw, _ := base64.StdEncoding.DecodeString(frame.Actors)
		mutate(raw)
		if _, err := decodeWorldActors(base64.StdEncoding.EncodeToString(raw)); err == nil {
			t.Fatal("malformed world accepted")
		}
	}
	frame.Center.X += 500
	if frame.Valid() {
		t.Fatal("out of source scope accepted")
	}
	rows = make([]WorldActor, MaxWorldActors)
	for i := range rows {
		rows[i] = testWorldActor(uint64(i+1), "Enemy_Bokoblin_Junior", 3430+float32(i))
	}
	frame = testWorldFrame(1, 1)
	frame.Actors = encodeWorldActors(rows)
	cmd, err := worldActorCommand("0123456789abcdef", "0123456789abcdef01234567", 1, 2, &frame)
	if err != nil || len(cmd) > 65535 {
		t.Fatal(len(cmd), err)
	}
	t.Logf("64-actor world command: %d bytes", len(cmd))
}
func TestWorldAuthorityEpochAndReplay(t *testing.T) {
	now := time.Now()
	s := &Server{environmentHost: "host", environmentEpoch: 3}
	host := &serverPeer{player: Player{ID: "host", Pose: Pose{Ready: true, Position: Vec3{3430, 236, 2100}}}, poseAt: now}
	other := &serverPeer{player: host.player, poseAt: now}
	other.player.ID = "client"
	frame := testWorldFrame(3, 10)
	if err := s.acceptWorldLocked(other, &frame, now); err == nil || s.worldState != nil {
		t.Fatal("follower became source")
	}
	if err := s.acceptWorldLocked(host, &frame, now); err != nil || s.worldState.Sequence != 10 {
		t.Fatal(err)
	}
	old := frame
	old.Sequence = 9
	if err := s.acceptWorldLocked(host, &old, now); err != nil || s.worldState.Sequence != 10 {
		t.Fatal("replay replaced world")
	}
	old = frame
	old.Epoch = 2
	old.Sequence = 99
	s.acceptWorldLocked(host, &old, now)
	if s.worldState.Sequence != 10 {
		t.Fatal("old epoch replaced world")
	}
	old = frame
	old.Center.X += 500
	old.Actors = encodeWorldActors(nil)
	s.acceptWorldLocked(host, &old, now)
	if s.worldState.Sequence != 10 {
		t.Fatal("old capture after teleport replaced world")
	}
	s.peers = map[string]*serverPeer{"client": other}
	other.player.Slot = 1
	s.chooseEnvironmentHostLocked()
	if s.worldState != nil || s.environmentHost != "client" || s.environmentEpoch != 4 {
		t.Fatal("old authority survived handoff")
	}
}
func TestWorldFramesOverNetworkAndHostLoss(t *testing.T) {
	s := testServer(t)
	a, b := testClient(t, s, "Host"), testClient(t, s, "Client")
	p := Pose{Ready: true, Position: Vec3{3430, 236, 2100}}
	a.Publish(p)
	b.Publish(p)
	eventually(t, func() bool { return a.WorldActors().HostID == a.Welcome().ID && b.WorldActors().Epoch > 0 })
	epoch := a.WorldActors().Epoch
	frame := testWorldFrame(epoch, 1)
	if err := a.PublishWorldActors(frame); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { v := b.WorldActors(); return v.State != nil && v.State.Actors == frame.Actors })
	copy := b.WorldActors()
	copy.State.Actors = "broken"
	if b.WorldActors().State.Actors != frame.Actors {
		t.Fatal("view escaped its lock")
	}
	a.Close()
	eventually(t, func() bool {
		v := b.WorldActors()
		return v.HostID == b.Welcome().ID && v.Epoch > epoch && v.State == nil
	})
	next := testWorldFrame(b.WorldActors().Epoch, 1)
	b.PublishWorldActors(next)
	eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.worldState != nil && s.worldState.Epoch == next.Epoch
	})
}
func TestWorldFollowerCannotPublish(t *testing.T) {
	s := testServer(t)
	a, b := testClient(t, s, "A"), testClient(t, s, "B")
	p := Pose{Ready: true, Position: Vec3{3430, 236, 2100}}
	a.Publish(p)
	b.Publish(p)
	eventually(t, func() bool { return b.WorldActors().Epoch > 0 })
	frame := testWorldFrame(b.WorldActors().Epoch, 1)
	b.PublishWorldActors(frame)
	eventually(t, func() bool { return !b.Connected() })
	if !a.Connected() {
		t.Fatal("host disconnected")
	}
}
func TestWorldBridgeThroughProductionControl(t *testing.T) {
	t.Setenv(core.NativePreviewEnv, "1")
	s := testServer(t)
	a, b := testClient(t, s, "A"), testClient(t, s, "B")
	frame := testWorldFrame(1, 1)
	rows := make([]WorldActor, MaxWorldActors)
	for i := range rows {
		rows[i] = testWorldActor(uint64(i+1), "Enemy_Bokoblin_Junior", 3430+float32(i))
	}
	frame.Actors = encodeWorldActors(rows)
	ga := &fakeNativeGame{fakeGame: &fakeGame{position: frame.Center}, worldActors: frame.Actors}
	gb := &fakeNativeGame{fakeGame: &fakeGame{position: frame.Center}}
	ba := NewBridge(context.Background(), a, gameSocket(t, ga), 40, nil)
	bb := NewBridge(context.Background(), b, gameSocket(t, gb), 40, nil)
	t.Cleanup(func() { ba.Close(); bb.Close(); <-ba.Done(); <-bb.Done() })
	eventually(t, func() bool {
		gb.mu.Lock()
		defer gb.mu.Unlock()
		for _, cmd := range gb.commands {
			if strings.HasPrefix(cmd, "hm_world_sample ") && strings.HasSuffix(cmd, " "+frame.Actors) {
				return true
			}
		}
		return false
	})
	if !a.Connected() || !b.Connected() {
		t.Fatal(a.Reason(), b.Reason())
	}
}
