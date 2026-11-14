package multiplayer

import (
	"encoding/json"
	"testing"
	"time"
)

func groundServer(t *testing.T) *Server {
	t.Helper()
	s, e := Listen("127.0.0.1:0", ServerConfig{Spawn: Vec3{3437.5, 235.9, 2101.5}, SharedItems: true, Tick: 20 * time.Millisecond})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func TestGroundPickupSingleWinnerAndReceiptReplay(t *testing.T) {
	s := groundServer(t)
	a, b := testClient(t, s, "Host"), testClient(t, s, "Guest")
	p := Pose{Ready: true, Position: s.cfg.Spawn}
	_ = a.Publish(p)
	_ = b.Publish(p)
	g := GroundSource{Local: 1, Hash: 123, Actor: "Item_Fruit_A", Position: p.Position, Spawn: p.Position}
	_ = b.PublishGround([]GroundSource{g})
	time.Sleep(40 * time.Millisecond)
	if len(a.Ground().Items) != 0 {
		t.Fatal("guest created a host map item")
	}
	_ = a.PublishGround([]GroundSource{g})
	eventually(t, func() bool { return len(b.Ground().Items) == 1 })
	_ = a.ClaimGround(GroundClaim{Request: 1, Source: g})
	_ = b.ClaimGround(GroundClaim{Request: 1, Source: g})
	eventually(t, func() bool { return len(a.Ground().Claims) == 1 && len(b.Ground().Claims) == 1 })
	av, bv := a.Ground(), b.Ground()
	winner := a
	win := av.Claims[0]
	if win.Status != "granted" {
		winner = b
		win = bv.Claims[0]
	}
	if win.Status != "granted" || av.Claims[0].Status == bv.Claims[0].Status {
		t.Fatal("multiple or no winners", av.Claims, bv.Claims)
	}
	receipt := GroundReceipt{Request: 1, Item: win.Item, Applied: true}
	_ = winner.ReceiptGround(receipt)
	_ = winner.ReceiptGround(receipt)
	eventually(t, func() bool { return a.Ground().Items[0].State == "taken" && b.Ground().Items[0].State == "taken" })
	_ = winner.ClaimGround(GroundClaim{Request: 2, Item: win.Item, Source: g})
	eventually(t, func() bool { v := winner.Ground(); return len(v.Claims) == 2 && v.Claims[1].Status == "denied" })
	// Late clients receive tombstones, so entering again cannot resurrect loot.
	b.Close()
	late := testClient(t, s, "Late")
	eventually(t, func() bool { return len(late.Ground().Items) == 1 && late.Ground().Items[0].State == "taken" })
}
func TestGroundDropFailedGrantAndUnknownDisconnect(t *testing.T) {
	s := groundServer(t)
	host, guest := testClient(t, s, "Host"), testClient(t, s, "Guest")
	p := Pose{Ready: true, Position: s.cfg.Spawn}
	_ = host.Publish(p)
	_ = guest.Publish(p)
	drop := GroundSource{Local: 8, Actor: "Item_PlantGet_A", Position: p.Position, Spawn: p.Position}
	_ = guest.PublishGround([]GroundSource{drop})
	eventually(t, func() bool { return len(host.Ground().Items) == 1 })
	id := host.Ground().Items[0].ID
	_ = host.ClaimGround(GroundClaim{Request: 1, Item: id, Source: drop})
	eventually(t, func() bool { return len(host.Ground().Claims) == 1 && host.Ground().Claims[0].Status == "granted" })
	_ = host.ReceiptGround(GroundReceipt{Item: id, Request: 1, Applied: false})
	eventually(t, func() bool { return guest.Ground().Items[0].State == "available" })
	_ = guest.ClaimGround(GroundClaim{Request: 1, Item: id, Source: drop})
	eventually(t, func() bool { return len(guest.Ground().Claims) == 1 && guest.Ground().Claims[0].Status == "granted" })
	guest.Close()
	eventually(t, func() bool { return host.Ground().Items[0].State == "uncertain" })
	_ = host.ClaimGround(GroundClaim{Request: 2, Item: id, Source: drop})
	eventually(t, func() bool { return len(host.Ground().Claims) == 2 && host.Ground().Claims[1].Status == "denied" })
}
func TestGroundBoundsAndSnapshotFitsTransport(t *testing.T) {
	s := groundServer(t)
	s.mu.Lock()
	defer s.mu.Unlock()
	p := &serverPeer{player: Player{ID: "host", Pose: Pose{Ready: true, Position: s.cfg.Spawn}}, poseAt: time.Now(), groundClaims: map[uint64]GroundClaim{}}
	s.environmentHost = "host"
	for i := uint64(1); i <= MaxGroundItems; i++ {
		g := GroundSource{Local: i, Actor: "Item_Fruit_A", Position: s.cfg.Spawn, Spawn: s.cfg.Spawn}
		if e := s.observeGround(p, []GroundSource{g}, time.Now()); e != nil {
			t.Fatal(e)
		}
		if e := s.claimGround(p, GroundClaim{Request: i, Source: g}, time.Now()); e != nil {
			t.Fatal(e)
		}
	}
	if len(s.groundItems) != MaxGroundItems {
		t.Fatal("world cap lost")
	}
	items, claims := s.groundSnapshot(p)
	m := Message{Type: "snapshot", GroundItems: items, GroundClaims: claims, GroundEnabled: true, GroundWorld: s.groundWorld}
	for i := 0; i < MaxSharedEnemies; i++ {
		m.Enemies = append(m.Enemies, EnemyHealth{Hash: uint32(i + 1), Actor: "Enemy_Bokoblin_Junior", Spawn: s.cfg.Spawn, Position: s.cfg.Spawn, Health: 1000000})
	}
	b, e := json.Marshal(m)
	if e != nil || len(b) > MaxLine-5000 {
		t.Fatal("world snapshot can disconnect presence", len(b), e)
	}
	bad := GroundSource{Local: 99, Actor: "GameROMPlayer", Position: s.cfg.Spawn}
	if s.observeGround(p, []GroundSource{bad}, time.Now()) == nil {
		t.Fatal("non-material accepted")
	}
	p.player.Pose.Position.X += 200
	g := GroundSource{Local: 100, Actor: "Item_Fruit_A", Position: s.cfg.Spawn, Spawn: s.cfg.Spawn}
	_ = s.claimGround(p, GroundClaim{Request: 100, Source: g}, time.Now())
	if p.groundClaims[100].Status != "denied" {
		t.Fatal("distant pickup accepted")
	}
}
