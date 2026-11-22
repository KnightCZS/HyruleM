package multiplayer

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGeneratedLootBatchAuthorityAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "world.json")
	cfg := ServerConfig{DedicatedWorld: true, SharedItems: true, StatePath: path, Tick: 20 * time.Millisecond, Spawn: Vec3{3437.5, 235.9, 2101.5}}
	s, err := Listen("127.0.0.1:0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, b := testClient(t, s, "A"), testClient(t, s, "B")
	pose := Pose{Ready: true, Scene: "MainField", Position: cfg.Spawn}
	_ = a.Publish(pose)
	_ = b.Publish(pose)
	eventually(t, func() bool { return len(a.Remote(time.Now())) == 1 && len(b.Remote(time.Now())) == 1 })
	parent := LootParent{Hash: 123, Actor: "Enemy_Bokoblin_Junior", Spawn: cfg.Spawn, Ordinal: 1}
	g := GroundSource{Parent: &parent, Scene: pose.Scene, Local: 0x200000001, NativeDrop: true, Drop: true, Origin: "0123456789abcdef", Actor: "Item_Enemy_00", Spawn: pose.Position, Position: pose.Position}
	e := EnemyHealth{Scene: pose.Scene, Hash: parent.Hash, Actor: parent.Actor, Spawn: parent.Spawn, Position: pose.Position, Health: 13}
	_ = a.PublishMapDamage(g.Origin, []EnemyHealth{e})
	_ = a.PublishGround([]GroundSource{g})
	time.Sleep(60 * time.Millisecond)
	if len(a.Ground().Items) != 0 {
		t.Fatal("live parent generated loot")
	}
	e.Health = 0
	e.Damage = 13
	_ = a.PublishMapDamage(g.Origin, []EnemyHealth{e})
	_ = a.PublishGround([]GroundSource{g})
	eventually(t, func() bool { return len(b.Ground().Items) == 1 })
	g2 := g
	parent2 := parent
	parent2.Ordinal = 2
	g2.Parent = &parent2
	g2.Local++
	g2.Actor = "Item_Enemy_01"
	duplicate := g2
	duplicate.Origin = "fedcba9876543210"
	_ = b.PublishGround([]GroundSource{duplicate})
	eventually(t, func() bool { return len(b.Ground().Discarded) == 1 })
	if b.Ground().Discarded[0] != duplicate.Local || len(b.Ground().Rejected) != 0 {
		t.Fatal("duplicate became local fallback")
	}
	_ = a.PublishGround([]GroundSource{g, g2})
	eventually(t, func() bool { return len(b.Ground().Items) == 2 })
	item := b.Ground().Items[0]
	if item.Source.Origin != g.Origin {
		t.Fatal("mixed native loot batches")
	}
	forged := item.Source
	forged.Origin = duplicate.Origin
	_ = b.ClaimGround(GroundClaim{Request: 1, Item: item.ID, Source: forged})
	eventually(t, func() bool { return len(b.Ground().Claims) == 1 })
	if b.Ground().Claims[0].Status != "denied" {
		t.Fatal("a duplicate physical source claimed the canonical item")
	}
	_ = b.ClaimGround(GroundClaim{Request: 2, Item: item.ID, Source: item.Source})
	eventually(t, func() bool {
		for _, c := range b.Ground().Claims {
			if c.Request == 2 {
				return c.Status == "granted"
			}
		}
		return false
	})
	_ = b.ReceiptGround(GroundReceipt{Request: 2, Item: item.ID, Applied: true})
	eventually(t, func() bool { return a.Ground().Items[0].State == "taken" })
	world := a.WorldActors().WorldID
	a.Close()
	b.Close()
	s.Close()
	again, err := Listen("127.0.0.1:0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	c := testClient(t, again, "Return")
	_ = c.Publish(pose)
	newer := g
	newer.Origin = "1111111111111111"
	_ = c.PublishGround([]GroundSource{newer})
	eventually(t, func() bool { return len(c.Ground().Discarded) == 1 })
	if len(again.groundItems) != 2 || again.groundItems[item.ID].State != "taken" || c.WorldActors().WorldID != world {
		t.Fatal("restart lost loot or its closed reservation")
	}
}

func TestGeneratedLootRequiresCompleteParentIdentity(t *testing.T) {
	parent := LootParent{Hash: 321, Actor: "Obj_TreeBroadleaf_A_L_Trunk", Spawn: Vec3{1.065, 3, 4}, Ordinal: 1}
	g := GroundSource{Parent: &parent, Scene: "MainField", Local: 0x200000001, NativeDrop: true, Drop: true, Origin: "0123456789abcdef", Actor: "Item_Material_04"}
	if !g.Valid() {
		t.Fatal("derived trunk ancestry not distinguished")
	}
	peer := &serverPeer{}
	server := &Server{cfg: ServerConfig{DedicatedWorld: true}, enemies: map[string]EnemyHealth{}}
	root := EnemyHealth{Scene: g.Scene, Hash: parent.Hash, Actor: "Obj_TreeBroadleaf_A_L", Spawn: parent.Spawn, Health: 0}
	server.enemies[root.key()] = root
	if server.acceptLoot(peer, g) {
		t.Fatal("wood accepted from the standing tree's zero alone")
	}
	root.Actor = parent.Actor
	server.enemies[root.key()] = root
	if !server.acceptLoot(peer, g) {
		t.Fatal("confirmed trunk destruction did not unlock wood")
	}
	for _, change := range []func(*GroundSource){func(g *GroundSource) { g.NativeDrop = false }, func(g *GroundSource) { g.Parent = &LootParent{} }, func(g *GroundSource) { g.Hash = 123 }} {
		bad := g
		change(&bad)
		if bad.Valid() {
			t.Fatal("partial or forged parent accepted", bad)
		}
	}
}

func TestTrunkSimulationIsExplicitAndMovesWithLease(t *testing.T) {
	now := time.Now()
	row := testWorldActor(701, "Obj_TreeBroadleaf_A_L_Trunk", 3434)
	row.Scene = "MainField"
	a := &serverPeer{player: Player{ID: "a", Pose: Pose{Ready: true, Scene: row.Scene, Position: row.position()}}, poseAt: now}
	b := &serverPeer{player: Player{ID: "b", Pose: Pose{Ready: true, Scene: row.Scene, Position: row.position().Add(Vec3{5, 0, 0})}}, poseAt: now}
	s := &Server{cfg: ServerConfig{DedicatedWorld: true}, entities: map[string]*worldEntity{row.key(): {State: row, Owner: "a", At: now, Settled: true}}, enemies: map[string]EnemyHealth{}, worldLease: 1, environmentEpoch: 1, peers: map[string]*serverPeer{"a": a, "b": b}}
	own := s.dedicatedWorldSnapshot(a, 1, now)
	other := s.dedicatedWorldSnapshot(b, 1, now)
	if !own.Valid() || len(own.Simulate) != 1 || own.Simulate[0] != 701 || len(other.Simulate) != 0 {
		t.Fatal("missing exclusive native lease", own, other)
	}
	command, err := worldActorCommand("0123456789abcdef", "0123456789abcdef01234567", 1, 3, own)
	if err != nil || !strings.HasSuffix(command, " MainField 701") {
		t.Fatal("lease lost before native control", command, err)
	}
	forged := *own
	forged.Sequence = 2
	forged.AgeMS = 0
	if err := s.acceptDedicatedWorldLocked(a, &forged, now); err == nil {
		t.Fatal("client advertised server-owned simulation metadata")
	}
	a.worldObserved = map[string]worldObservation{row.key(): {State: row, At: now}}
	b.worldObserved = map[string]worldObservation{row.key(): {State: row, At: now}}
	a.player.Pose.Position = a.player.Pose.Position.Add(Vec3{10, 0, 0})
	b.player.Pose.Position = row.position()
	s.reconcileWorldLocked(now)
	source := WorldState{Scene: row.Scene, Epoch: 1, Lease: s.worldLease, Sequence: 3, Center: b.player.Pose.Position, Actors: encodeWorldActors([]WorldActor{row})}
	if err := s.acceptDedicatedWorldLocked(b, &source, now); err != nil {
		t.Fatal(err)
	}
	a.worldOutput = nil
	b.worldOutput = nil
	if len(s.dedicatedWorldSnapshot(b, 4, now).Simulate) != 1 || len(s.dedicatedWorldSnapshot(a, 4, now).Simulate) != 0 {
		t.Fatal("lease did not migrate")
	}
	duplicate := *own
	duplicate.Simulate = []uint64{701, 701}
	if duplicate.Valid() {
		t.Fatal("duplicate native lease accepted")
	}
}

func TestEnrollmentIsIndependentOfVisibleItemWindow(t *testing.T) {
	s, err := Listen("127.0.0.1:0", ServerConfig{DedicatedWorld: true, SharedItems: true, Tick: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c := testClient(t, s, "Source")
	pose := Pose{Ready: true, Scene: "MainField", Position: Vec3{3434, 236, 2100}}
	_ = c.Publish(pose)
	eventually(t, func() bool { return c.Ground().Enabled })
	parent := LootParent{Hash: 212, Actor: "Enemy_Bokoblin_Junior", Spawn: pose.Position, Ordinal: 1}
	e := EnemyHealth{Scene: pose.Scene, Hash: parent.Hash, Actor: parent.Actor, Spawn: parent.Spawn, Position: pose.Position, Health: 0}
	_ = c.PublishMapDamage("0123456789abcdef", []EnemyHealth{e})
	for page := 0; page < 3; page++ {
		sources := make([]GroundSource, 32)
		for i := range sources {
			p := parent
			p.Ordinal = uint32(page*32 + i + 1)
			sources[i] = GroundSource{Parent: &p, Scene: pose.Scene, Local: 0x200000000 + uint64(p.Ordinal), NativeDrop: true, Drop: true, Origin: "0123456789abcdef", Actor: "Item_Enemy_00", Spawn: pose.Position, Position: pose.Position}
		}
		if err := c.PublishGround(sources); err != nil {
			t.Fatal(err)
		}
		first, last := sources[0].Local, sources[31].Local
		eventually(t, func() bool {
			v := c.Ground()
			if len(v.Enrolled) != 32 {
				return false
			}
			for _, id := range v.Enrolled {
				if id < first || id > last {
					return false
				}
			}
			return true
		})
		if len(c.Ground().Items) > MaxGroundItems {
			t.Fatal("registration enlarged the native visual window")
		}
	}
	s.mu.Lock()
	count := len(s.groundItems)
	s.mu.Unlock()
	if count != 96 {
		t.Fatal("unseen source enrollment starved", count)
	}
}
