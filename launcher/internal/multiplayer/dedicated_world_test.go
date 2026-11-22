package multiplayer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDedicatedAuthorityDoesNotOscillateAtEqualDistance(t *testing.T) {
	now := time.Now()
	row := testWorldActor(1, "Enemy_Bokoblin_Junior", 3434)
	row.Scene = "MainField"
	peer := func(id string, dx float64) *serverPeer {
		return &serverPeer{player: Player{ID: id, Pose: Pose{Ready: true, Scene: row.Scene, Position: row.position().Add(Vec3{dx, 0, 0})}}, poseAt: now,
			worldObserved: map[string]worldObservation{row.key(): {State: row, At: now}}}
	}
	a, b := peer("a", -5), peer("b", 5)
	entity := &worldEntity{State: row, Owner: "a", Settled: true, At: now}
	s := &Server{entities: map[string]*worldEntity{row.key(): entity}, peers: map[string]*serverPeer{"a": a, "b": b}, worldLease: 1}
	for i := 0; i < 20; i++ {
		b.player.Pose.Position.X = row.position().X + 4.9 + float64(i%2)*.2
		s.reconcileWorldLocked(now)
		if entity.Owner != "a" || s.worldLease != 1 || !entity.Settled {
			t.Fatal("small distance changes reset simulation", entity.Owner, s.worldLease)
		}
	}
	b.player.Pose.Position = row.position()
	s.reconcileWorldLocked(now)
	if entity.Owner != "b" || s.worldLease != 2 || entity.Settled {
		t.Fatal("clearly nearer player did not acquire fenced authority", entity.Owner, s.worldLease)
	}
	// A paused, stale, unloaded or disconnected owner cannot retain authority
	// through the distance margin. A target in another scene is never eligible.
	for _, unavailable := range []string{"paused", "stale pose", "stale actor", "unloaded", "other scene", "disconnected"} {
		t.Run(unavailable, func(t *testing.T) {
			a, b := peer("a", -5), peer("b", 5.1)
			s.peers = map[string]*serverPeer{"a": a, "b": b}
			entity.Owner = "a"
			s.worldLease = 1
			switch unavailable {
			case "paused":
				a.worldPaused = true
			case "stale pose":
				a.poseAt = now.Add(-3 * time.Second)
			case "stale actor":
				a.worldObserved[row.key()] = worldObservation{State: row, At: now.Add(-2 * time.Second)}
			case "unloaded":
				delete(a.worldObserved, row.key())
			case "other scene":
				a.player.Pose.Scene = "MainFieldDungeon"
			case "disconnected":
				delete(s.peers, "a")
			}
			s.reconcileWorldLocked(now)
			if entity.Owner != "b" || s.worldLease != 2 {
				t.Fatal("unavailable owner retained authority", entity.Owner, s.worldLease)
			}
		})
	}
}

func TestMissingEnemyReceivesCanonicalHealthWithoutLocalObservation(t *testing.T) {
	s, err := Listen("127.0.0.1:0", ServerConfig{DedicatedWorld: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, b := testClient(t, s, "Source"), testClient(t, s, "Missing")
	p := Pose{Scene: "MainField", Ready: true, Position: Vec3{3430, 236, 2100}}
	a.Publish(p)
	b.Publish(p)
	eventually(t, func() bool { return a.WorldActors().State != nil && b.WorldActors().State != nil })
	row := testWorldActor(10, "Enemy_Bokoblin_Junior", 3434)
	row.Scene = p.Scene
	v := a.WorldActors()
	a.PublishWorldActors(WorldState{Scene: p.Scene, Epoch: v.Epoch, Lease: v.State.Lease, Sequence: 1, Center: p.Position, Actors: encodeWorldActors([]WorldActor{row})})
	e := EnemyHealth{Scene: p.Scene, Hash: row.Hash, Actor: row.Actor, Spawn: row.Spawn, Position: row.position(), Health: 13}
	a.PublishMapDamage("0123456789abcdef", []EnemyHealth{e})
	eventually(t, func() bool { return len(b.SharedEnemyHealth()) == 1 })
	a.Close()
	eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.entities[row.key()] != nil && s.entities[row.key()].Owner == ""
	})
	game := &worldControl{enemies: []EnemyHealth{}}
	bridge := &Bridge{Client: b, control: game}
	native := &nativePreview{session: "abcdef0123456789"}
	if err := bridge.updateWorldHealth(context.Background(), native, time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(game.commands) != 2 || !strings.Contains(game.commands[1], "Enemy_Bokoblin_Junior") || !strings.HasSuffix(game.commands[1], " 13 MainField") {
		t.Fatal("missing canonical health seed", game.commands)
	}
}

func TestReconstructionCannotAcquireAuthorityAndUnownedStateRemainsVisible(t *testing.T) {
	s, err := Listen("127.0.0.1:0", ServerConfig{DedicatedWorld: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, b := testClient(t, s, "Simulator"), testClient(t, s, "Reconstruction")
	a.Publish(Pose{Scene: "MainField", Ready: true, Position: Vec3{3430, 236, 2100}})
	b.Publish(Pose{Scene: "MainField", Ready: true, Position: Vec3{3434, 236, 2100}})
	eventually(t, func() bool { return a.WorldActors().State != nil && b.WorldActors().State != nil })
	row := testWorldActor(10, "Npc_Test", 3434)
	row.Scene = "MainField"
	publish := func(c *Client, seq uint64, replicas []uint64) {
		v := c.WorldActors()
		p, _ := c.LocalPose()
		if err := c.PublishWorldActors(WorldState{Scene: "MainField", Epoch: v.Epoch, Lease: v.State.Lease, Sequence: seq, Center: p.Position, Actors: encodeWorldActors([]WorldActor{row}), Replicas: replicas}); err != nil {
			t.Fatal(err)
		}
	}
	publish(a, 1, nil)
	eventually(t, func() bool {
		rows, err := decodeWorldActors(b.WorldActors().State.Actors)
		return err == nil && len(rows) == 1 && rows[0].Hash == row.Hash
	})
	publish(b, 1, []uint64{10})
	eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.peers[b.Welcome().ID].worldObserved) == 0 && s.peers[b.Welcome().ID].worldSequence == 1
	})
	s.mu.Lock()
	owner := s.entities[row.key()].Owner
	s.mu.Unlock()
	if owner != a.Welcome().ID {
		t.Fatal("reconstruction became the simulator", owner)
	}
	a.Close()
	eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.entities[row.key()].Owner == "" })
	eventually(t, func() bool {
		rows, e := decodeWorldActors(b.WorldActors().State.Actors)
		return e == nil && len(rows) == 1 && rows[0].Hash == row.Hash
	})
	frame := *b.WorldActors().State
	frame.Replicas = []uint64{1, 1}
	if frame.Valid() {
		t.Fatal("duplicate replica identities accepted")
	}
	frame.Replicas = []uint64{99}
	if frame.Valid() {
		t.Fatal("unknown replica identity accepted")
	}
}

func TestDedicatedTerminalEnemyLeavesMovementSnapshot(t *testing.T) {
	now := time.Now()
	a := testWorldActor(1, "Enemy_Bokoblin_Junior", 3434)
	a.Scene = "MainField"
	// Binary actor coordinates and three-decimal native JSON describe the same
	// placement. A second rounding to two decimals must not split its health.
	a.Spawn.X = float64(float32(1.065))
	s := &Server{entities: map[string]*worldEntity{a.key(): {State: a, Owner: "simulator", At: now, Settled: true}}, enemies: map[string]EnemyHealth{}, worldLease: 1}
	p := &serverPeer{player: Player{ID: "observer", Pose: Pose{Ready: true, Scene: a.Scene, Position: a.position()}}}
	rows, err := decodeWorldActors(s.dedicatedWorldSnapshot(p, 1, now).Actors)
	if err != nil || len(rows) != 1 {
		t.Fatal("living actor absent", rows, err)
	}
	e := EnemyHealth{Scene: a.Scene, Hash: a.Hash, Actor: a.Actor, Spawn: a.Spawn, Health: 0}
	e.Spawn.X = 1.065
	s.enemies[e.key()] = e
	rows, err = decodeWorldActors(s.dedicatedWorldSnapshot(p, 2, now.Add(101*time.Millisecond)).Actors)
	if err != nil || len(rows) != 0 {
		t.Fatal("terminal actor still receives movement authority", rows, err)
	}
}

func TestDedicatedWorldLeaseAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "world.json")
	cfg := ServerConfig{DedicatedWorld: true, StatePath: path, SharedItems: true}
	s, err := Listen("127.0.0.1:0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if other, e := Listen("127.0.0.1:0", cfg); e == nil {
		other.Close()
		t.Fatal("two servers opened one world")
	}
	a, b := testClient(t, s, "First"), testClient(t, s, "Second")
	a.Publish(Pose{Scene: "MainField", Ready: true, Position: Vec3{3430, 236, 2100}})
	b.Publish(Pose{Scene: "MainField", Ready: true, Position: Vec3{3500, 236, 2100}})
	eventually(t, func() bool {
		return a.WorldActors().Dedicated && a.WorldActors().State != nil && b.WorldActors().State != nil
	})
	originalWorld := a.WorldActors().WorldID
	entity := testWorldActor(10, "Enemy_Bokoblin_Junior", 3434)
	entity.Scene = "MainField"
	publish := func(c *Client, seq uint64, x float32) WorldState {
		t.Helper()
		v := c.WorldActors()
		row := entity
		row.Matrix[3] = x
		frame := WorldState{Scene: "MainField", Epoch: v.Epoch, Lease: v.State.Lease, Sequence: seq, Center: func() Vec3 { p, _ := c.LocalPose(); return p.Position }(), Complete: true, Actors: encodeWorldActors([]WorldActor{row})}
		if e := c.PublishWorldActors(frame); e != nil {
			t.Fatal(e)
		}
		return frame
	}
	publish(a, 1, 3434)
	eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.entities[entity.key()] != nil && s.entities[entity.key()].Owner == a.Welcome().ID
	})
	b.Publish(Pose{Scene: "MainField", Ready: true, Position: Vec3{3434, 236, 2100}})
	eventually(t, func() bool { p, ok := b.LocalPose(); return ok && p.Position.X == 3434 })
	old := publish(b, 1, 3434)
	eventually(t, func() bool { return a.WorldActors().State.Lease > old.Lease })
	publish(b, 2, 3435)
	eventually(t, func() bool {
		v := a.WorldActors()
		rows, _ := decodeWorldActors(v.State.Actors)
		return len(rows) == 1 && rows[0].Matrix[3] == 3435
	})
	stale := old
	stale.Sequence = 999
	a.PublishWorldActors(stale)
	time.Sleep(40 * time.Millisecond)
	s.mu.Lock()
	if s.entities[entity.key()].State.Matrix[3] != 3435 {
		t.Error("old lease overwrote authority")
	}
	s.mu.Unlock()
	g := GroundSource{Scene: "MainField", Local: 1, Hash: 92, Actor: "Item_Fruit_A", Spawn: Vec3{3434, 236, 2100}, Position: Vec3{3434, 236, 2100}}
	b.PublishGround([]GroundSource{g})
	eventually(t, func() bool { return len(a.Ground().Items) == 1 })
	b.ClaimGround(GroundClaim{Request: 1, Source: g})
	eventually(t, func() bool { return len(b.Ground().Claims) == 1 && b.Ground().Claims[0].Status == "granted" })
	b.ReceiptGround(GroundReceipt{Item: b.Ground().Claims[0].Item, Request: 1, Applied: true})
	death := EnemyHealth{Scene: "MainField", Hash: 42, Actor: "Enemy_Bokoblin_Junior", Spawn: g.Spawn, Position: g.Position, Health: 0}
	b.PublishMapDamage("0123456789abcdef", []EnemyHealth{death})
	eventually(t, func() bool {
		return a.Ground().Items[0].State == "taken" && len(a.SharedEnemyHealth()) == 1 && a.SharedEnemyHealth()[0].Health == 0
	})
	a.Close()
	eventually(t, func() bool { return s.Count() == 1 })
	if b.WorldActors().HostID != serverAuthority || b.WorldActors().WorldID != originalWorld {
		t.Fatal("first player owned the world")
	}
	b.Close()
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := Listen("127.0.0.1:0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	restarted.mu.Lock()
	if restarted.groundWorld != originalWorld || restarted.enemies[death.key()].Health != 0 || len(restarted.entities) != 1 {
		t.Error("world was recreated on restart")
	}
	restarted.mu.Unlock()
	c, err := Connect(context.Background(), restarted.Addr().String(), "Returned", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Publish(Pose{Scene: "MainField", Ready: true, Position: g.Position})
	eventually(t, func() bool { return len(c.Ground().Items) == 1 && c.Ground().Items[0].State == "taken" })
}

func TestWorldStoreRejectsCorruptionAndClosesOnWriteFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "world.json")
	os.WriteFile(path, []byte("{broken"), 0600)
	if s, err := Listen("127.0.0.1:0", ServerConfig{DedicatedWorld: true, StatePath: path}); err == nil {
		s.Close()
		t.Fatal("corrupt save replaced")
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "{broken" {
		t.Fatal("corrupt save overwritten")
	}
	os.Remove(path)
	s, err := Listen("127.0.0.1:0", ServerConfig{DedicatedWorld: true, StatePath: path, SharedItems: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c := testClient(t, s, "One")
	c.Publish(Pose{Scene: "MainField", Ready: true, Position: Vec3{1, 1, 1}})
	g := GroundSource{Scene: "MainField", Local: 1, Hash: 1, Actor: "Item_Fruit_A", Spawn: Vec3{1, 1, 1}, Position: Vec3{1, 1, 1}}
	c.PublishGround([]GroundSource{g})
	eventually(t, func() bool { return len(c.Ground().Items) == 1 })
	blocked := filepath.Join(filepath.Dir(path), "directory")
	os.Mkdir(blocked, 0700)
	s.mu.Lock()
	s.cfg.StatePath = blocked
	s.mu.Unlock()
	c.ClaimGround(GroundClaim{Request: 1, Source: g})
	select {
	case err := <-s.Errors():
		if !strings.Contains(err.Error(), "gravação") {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("failed persistence kept granting")
	}
	eventually(t, func() bool { return !c.Connected() })
}

func TestDedicatedItemRegionsAndGlobalToNativeSlots(t *testing.T) {
	s, err := Listen("127.0.0.1:0", ServerConfig{DedicatedWorld: true, SharedItems: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c := testClient(t, s, "Traveller")
	var first GroundSource
	for region := 0; region < 5; region++ {
		pos := Vec3{float64(region * 1000), 236, 2100}
		c.Publish(Pose{Scene: "MainField", Ready: true, Position: pos})
		eventually(t, func() bool { return c.Ground().Enabled && c.Ground().Zone == pos })
		rows := make([]GroundSource, MaxGroundItems)
		for i := range rows {
			rows[i] = GroundSource{Scene: "MainField", Local: uint64(region*32 + i + 1), Hash: uint32(region*32 + i + 1), Actor: "Item_Ore_A", Spawn: pos, Position: pos}
		}
		if region == 0 {
			first = rows[0]
		}
		c.PublishGround(rows)
		eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.groundItems) == (region+1)*32 })
		eventually(t, func() bool { return len(c.Ground().Items) == 32 && c.Ground().Items[0].ID == uint64(region*32+1) })
	}
	game := &groundGame{scene: "MainField"}
	bridge := &Bridge{Client: c, control: game, roamingGround: true, nativeSession: "0123456789abcdef"}
	n := &groundSync{}
	if err := bridge.updateGround(context.Background(), n, time.Now()); err != nil {
		t.Fatal(err)
	}
	source := c.Ground().Items[0].Source
	id := c.Ground().Items[0].ID
	slot := n.slots[id]
	if id <= MaxGroundItems || slot == 0 || slot > MaxGroundItems {
		t.Fatal("missing global/native mapping", id, slot)
	}
	game.events = []GroundClaim{{Request: 1, Item: slot, Source: source}}
	n.last = time.Time{}
	if err := bridge.updateGround(context.Background(), n, time.Now()); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		return len(c.Ground().Claims) == 1 && c.Ground().Claims[0].Item == id && c.Ground().Claims[0].Status == "granted"
	})
	game.receipts = []GroundReceipt{{Request: 1, Item: slot, Applied: true}}
	n.last = time.Time{}
	bridge.updateGround(context.Background(), n, time.Now())
	eventually(t, func() bool {
		for _, item := range c.Ground().Items {
			if item.ID == id {
				return item.State == "taken"
			}
		}
		return false
	})
	game.events = nil
	game.receipts = nil
	c.Publish(Pose{Scene: "MainField", Ready: true, Position: first.Position})
	eventually(t, func() bool {
		return c.Ground().Zone == first.Position && len(c.Ground().Items) == 32 && c.Ground().Items[0].ID == 1
	})
	for i := 0; i < 12; i++ {
		n.last = time.Time{}
		if err := bridge.updateGround(context.Background(), n, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if n.slots[id] != 0 || n.slots[1] == 0 || len(n.slots) != 32 {
		t.Fatal("slots not recycled after native acknowledgement", n.slots)
	}
}

func TestDedicatedSceneIsolation(t *testing.T) {
	s, err := Listen("127.0.0.1:0", ServerConfig{DedicatedWorld: true, SharedItems: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, b := testClient(t, s, "Shrine one"), testClient(t, s, "Shrine two")
	pos := Vec3{10, 236, 2100}
	a.Publish(Pose{Ready: true, Scene: "CDungeon/Dungeon000", Position: pos})
	b.Publish(Pose{Ready: true, Scene: "CDungeon/Dungeon001", Position: pos})
	eventually(t, func() bool {
		return a.Ground().Scene == "CDungeon/Dungeon000" && b.Ground().Scene == "CDungeon/Dungeon001"
	})
	if len(a.Remote(time.Now())) != 0 || len(b.Remote(time.Now())) != 0 || a.Ground().World == b.Ground().World {
		t.Fatal("players or ground crossed scenes")
	}
	for _, c := range []*Client{a, b} {
		v := c.WorldActors()
		row := testWorldActor(1, "Enemy_Bokoblin_Junior", 10)
		c.PublishWorldActors(WorldState{Epoch: v.Epoch, Lease: v.State.Lease, Sequence: 1, Scene: v.State.Scene,
			Center: pos, Actors: encodeWorldActors([]WorldActor{row})})
		c.PublishGround([]GroundSource{{Scene: v.State.Scene, Local: 1, Hash: 1, Actor: "Item_Ore_A", Spawn: pos, Position: pos}})
	}
	eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.entities) == 2 && len(s.groundItems) == 2 })
	eventually(t, func() bool { return len(a.Ground().Items) == 1 && len(b.Ground().Items) == 1 })
	if a.Ground().Items[0].ID == b.Ground().Items[0].ID {
		t.Fatal("same coordinates merged two shrine objects")
	}
	b.Publish(Pose{Ready: true, Scene: "CDungeon/Dungeon000", Position: pos})
	eventually(t, func() bool { return len(a.Remote(time.Now())) == 1 && len(b.Remote(time.Now())) == 1 })
}
