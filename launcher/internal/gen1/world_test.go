package gen1

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

func testCaps() Capabilities {
	return Capabilities{ABI: 1, Runtime: "test-adapter", GameVersion: "1.9.0", NativeActors: true, Animation: true, Collision: true, PersonalVitals: true, PersonalInventory: true, PickupHook: true, DamageHook: true, EnemyAuthority: true, EnemyTarget: true, WorldIdentity: true, Simulated: true}
}
func testCatalog() Catalog {
	return Catalog{Items: []ItemDefinition{{Actor: "Weapon_Test", Kind: "weapon", MaxStack: 1, Durability: 20, Damage: 2, Reach: 3, CooldownMS: 500}, {Actor: "Material_Test", Kind: "material", MaxStack: 99, CooldownMS: 100}}, Enemies: []EnemyDefinition{{Actor: "Enemy_Test", Health: 20, Damage: 3, AggroRange: 30, AttackRange: 2, CooldownMS: 1000}}}
}

func TestBootstrapItemIDsCannotCollideWithPickups(t *testing.T) {
	w, err := NewWorld(Config{Catalog: testCatalog(), AllowSimulation: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Join("A", "A", testCaps()); err != nil {
		t.Fatal(err)
	}
	commands := []Command{
		{Kind: "seed", Personal: &Personal{Health: 12, MaxHealth: 12, Inventory: []Item{{ID: "item-1", Actor: "Material_Test", Count: 1}}}},
		{Kind: "frame", Frame: &Frame{World: "MainField", Animation: "idle", Ready: true}},
		{Kind: "register_item", Entity: "map:material", Definition: "Material_Test", World: "MainField", Amount: 1},
		{Kind: "pickup", Entity: "map:material"},
	}
	for i, c := range commands {
		c.Sequence = uint64(i + 1)
		if r := w.Apply("A", c); !r.Accepted {
			t.Fatal(r)
		}
	}
	s, _ := w.Snapshot("A")
	if len(s.Personal.Inventory) != 2 || s.Personal.Inventory[0].ID == s.Personal.Inventory[1].ID {
		t.Fatal("bootstrap/pickup IDs collided")
	}
}

type harness struct {
	w        *World
	t        *testing.T
	now      time.Time
	sequence map[string]uint64
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t, now: time.Unix(1000, 0), sequence: map[string]uint64{}}
	w, e := NewWorld(Config{Catalog: testCatalog(), AllowSimulation: true, AllowPVP: true, Now: func() time.Time { return h.now }})
	if e != nil {
		t.Fatal(e)
	}
	h.w = w
	for _, id := range []string{"A", "B"} {
		if _, e = w.Join(id, id, testCaps()); e != nil {
			t.Fatal(e)
		}
		h.must(id, Command{Kind: "seed", Personal: &Personal{Health: 12, MaxHealth: 12, Inventory: []Item{}}})
		h.frame(id, Vec3{}, "MainField")
	}
	return h
}
func (h *harness) send(id string, c Command) Result {
	h.sequence[id]++
	c.Sequence = h.sequence[id]
	return h.w.Apply(id, c)
}
func (h *harness) must(id string, c Command) {
	h.t.Helper()
	if r := h.send(id, c); !r.Accepted {
		h.t.Fatal(c.Kind, r.Error)
	}
}
func (h *harness) frame(id string, pos Vec3, world string) {
	h.t.Helper()
	h.must(id, Command{Kind: "frame", Frame: &Frame{World: world, Position: pos, Animation: "idle", Ready: true}})
}
func (h *harness) snapshot(id string) Snapshot {
	h.t.Helper()
	s, e := h.w.Snapshot(id)
	if e != nil {
		h.t.Fatal(e)
	}
	return s
}
func TestInventoryPickupIsExclusiveAndPrivate(t *testing.T) {
	h := newHarness(t)
	h.must("A", Command{Kind: "register_item", Entity: "map:weapon", Definition: "Weapon_Test", World: "MainField", Amount: 1})
	var wg sync.WaitGroup
	results := make(chan Result, 2)
	for _, id := range []string{"A", "B"} {
		wg.Add(1)
		sequence := h.sequence[id] + 1
		h.sequence[id] = sequence
		go func(id string, sequence uint64) {
			defer wg.Done()
			results <- h.w.Apply(id, Command{Kind: "pickup", Entity: "map:weapon", Sequence: sequence})
		}(id, sequence)
	}
	wg.Wait()
	close(results)
	winners := 0
	for r := range results {
		if r.Accepted {
			winners++
		}
	}
	if winners != 1 {
		t.Fatal("pickup duplicated", winners)
	}
	a, b := h.snapshot("A"), h.snapshot("B")
	if len(a.Personal.Inventory)+len(b.Personal.Inventory) != 1 || len(a.Items) != 0 {
		t.Fatal("item not exclusively transferred")
	}
	h.must("A", Command{Kind: "register_item", Entity: "map:weapon", Definition: "Weapon_Test", World: "MainField", Amount: 1})
	if len(h.snapshot("A").Items) != 0 {
		t.Fatal("repeated map registration resurrected claimed item")
	}
	owner, other := "A", "B"
	if len(a.Personal.Inventory) == 0 {
		owner, other = "B", "A"
	}
	item := h.snapshot(owner).Personal.Inventory[0]
	if h.send(other, Command{Kind: "equip", Entity: item.ID}).Accepted {
		t.Fatal("equipped another player's item")
	}
	if h.send(other, Command{Kind: "drop", Entity: item.ID}).Accepted {
		t.Fatal("dropped another player's item")
	}
	raw, _ := json.Marshal(h.snapshot(other))
	if strings.Contains(string(raw), item.ID) {
		t.Fatal("private inventory item leaked to other client")
	}
	h.must(owner, Command{Kind: "equip", Entity: item.ID})
	h.must(owner, Command{Kind: "drop", Entity: item.ID})
	s := h.snapshot(other)
	if len(s.Items) != 1 {
		t.Fatal("drop missing")
	}
	h.must(other, Command{Kind: "pickup", Entity: s.Items[0].ID})
	if len(h.snapshot(other).Personal.Inventory) != 1 || len(h.snapshot(owner).Personal.Inventory) != 0 {
		t.Fatal("handoff duplicated inventory")
	}
}
func TestDamageOnlyTouchesVictimAndConsumesOwnDurability(t *testing.T) {
	h := newHarness(t)
	h.must("A", Command{Kind: "register_item", Entity: "map:weapon", Definition: "Weapon_Test", World: "MainField", Amount: 1})
	h.must("A", Command{Kind: "pickup", Entity: "map:weapon"})
	item := h.snapshot("A").Personal.Inventory[0]
	h.must("A", Command{Kind: "equip", Entity: item.ID})
	h.must("A", Command{Kind: "attack", Target: "B"})
	if h.snapshot("A").Personal.Health != 12 || h.snapshot("B").Personal.Health != 10 {
		t.Fatal("shared damage")
	}
	if h.snapshot("A").Personal.Inventory[0].Durability != 19 {
		t.Fatal("wrong weapon durability")
	}
	sequence := h.sequence["A"]
	duplicate := h.w.Apply("A", Command{Kind: "attack", Target: "B", Sequence: sequence})
	if !duplicate.Accepted || h.snapshot("B").Personal.Health != 10 {
		t.Fatal("duplicate hit applied twice")
	}
	if h.send("A", Command{Kind: "attack", Target: "B"}).Accepted {
		t.Fatal("ignored cooldown")
	}
	if h.send("A", Command{Kind: "environment_damage", Target: "B", Amount: 7}).Accepted {
		t.Fatal("environment damage forged for another player")
	}
	h.must("A", Command{Kind: "environment_damage", Amount: 1})
	if h.snapshot("A").Personal.Health != 11 || h.snapshot("B").Personal.Health != 10 {
		t.Fatal("environment damage leaked")
	}
	h.frame("B", Vec3{X: 10}, "MainField")
	h.now = h.now.Add(time.Second)
	if h.send("A", Command{Kind: "attack", Target: "B"}).Accepted {
		t.Fatal("hit outside range")
	}
}
func TestMonstersImmediatelySwitchToNearestAndRejectOldOwner(t *testing.T) {
	h := newHarness(t)
	h.frame("A", Vec3{X: 5}, "MainField")
	h.frame("B", Vec3{X: 8}, "MainField")
	h.must("A", Command{Kind: "register_enemy", Entity: "map:enemy", Definition: "Enemy_Test", World: "MainField"})
	first := h.snapshot("A").Enemies[0]
	if first.Target != "A" || first.Owner != "A" {
		t.Fatal(first)
	}
	h.frame("B", Vec3{X: 1}, "MainField")
	next := h.snapshot("A").Enemies[0]
	if next.Target != "B" || next.Owner != "B" || next.Epoch <= first.Epoch {
		t.Fatal("nearest player did not replace old target", next)
	}
	if h.send("A", Command{Kind: "enemy_pose", Entity: next.ID, Epoch: first.Epoch, PoseSequence: 99, Position: Vec3{X: 1}, Animation: "run"}).Accepted {
		t.Fatal("old owner still simulates enemy")
	}
	if h.send("A", Command{Kind: "enemy_attack", Entity: next.ID, Target: "A", Epoch: first.Epoch}).Accepted {
		t.Fatal("stale owner applied damage")
	}
	h.must("B", Command{Kind: "enemy_attack", Entity: next.ID, Target: "B", Epoch: next.Epoch})
	if h.snapshot("A").Personal.Health != 12 || h.snapshot("B").Personal.Health != 9 {
		t.Fatal("enemy damaged both players")
	}
	h.frame("A", Vec3{X: .5}, "MainField")
	third := h.snapshot("A").Enemies[0]
	if third.Target != "A" {
		t.Fatal("target stuck to previous player")
	}
	if h.send("A", Command{Kind: "enemy_attack", Entity: third.ID, Target: "A", Epoch: third.Epoch}).Accepted {
		t.Fatal("authority transfer reset attack cooldown")
	}
	h.now = h.now.Add(time.Second)
	h.must("A", Command{Kind: "enemy_attack", Entity: third.ID, Target: "A", Epoch: third.Epoch})
	h.frame("A", Vec3{X: 50}, "MainField")
	if h.snapshot("B").Enemies[0].Target != "B" {
		t.Fatal("did not switch after leaving range")
	}
	h.w.Leave("B")
	if h.snapshot("A").Enemies[0].Target != "" {
		t.Fatal("enemy targets disconnected/out-of-range player")
	}
}
func TestWorldsDeadPlayersAndStalePlayersAreNotTargets(t *testing.T) {
	h := newHarness(t)
	h.frame("A", Vec3{X: 1}, "MainField")
	h.frame("B", Vec3{X: .5}, "Dungeon_1")
	h.must("A", Command{Kind: "register_enemy", Entity: "map:enemy", Definition: "Enemy_Test", World: "MainField"})
	if h.snapshot("A").Enemies[0].Target != "A" {
		t.Fatal("target crossed world")
	}
	h.must("A", Command{Kind: "environment_damage", Amount: 12})
	if h.snapshot("A").Enemies[0].Target != "" {
		t.Fatal("targeting dead player")
	}
	h.frame("B", Vec3{X: 2}, "MainField")
	if h.snapshot("B").Enemies[0].Target != "B" {
		t.Fatal("alive player ignored")
	}
	h.now = h.now.Add(3 * time.Second)
	if h.snapshot("B").Enemies[0].Target != "" {
		t.Fatal("stale player kept authority")
	}
}
func TestBootstrapAndReconnectDoNotWipeOrShareInventory(t *testing.T) {
	h := newHarness(t)
	if h.send("B", Command{Kind: "seed", Personal: &Personal{ID: "A", Health: 1, MaxHealth: 12}}).Accepted {
		t.Fatal("reseed allowed")
	}
	h.must("A", Command{Kind: "register_item", Entity: "map:weapon", Definition: "Weapon_Test", World: "MainField", Amount: 1})
	h.must("A", Command{Kind: "pickup", Entity: "map:weapon"})
	h.must("A", Command{Kind: "environment_damage", Amount: 4})
	token := h.w.SessionToken("A")
	h.w.Leave("A")
	if _, e := h.w.Resume("A2", "New name", testCaps(), token); e != nil {
		t.Fatal(e)
	}
	state, _ := h.w.Snapshot("A2")
	if state.Personal.Health != 8 || len(state.Personal.Inventory) != 1 {
		t.Fatal("reconnect lost private state", state.Personal)
	}
	raw, _ := json.Marshal(h.snapshot("B"))
	if strings.Contains(string(raw), token) || strings.Contains(string(raw), state.Personal.Inventory[0].ID) {
		t.Fatal("resume token or inventory leaked")
	}
	if _, e := h.w.Resume("C", "Third", testCaps(), token); e == nil {
		t.Fatal("third/duplicate profile accepted")
	}
	state.Personal.Inventory[0].Durability = 0
	again, _ := h.w.Snapshot("A2")
	if again.Personal.Inventory[0].Durability == 0 {
		t.Fatal("snapshot aliases live inventory")
	}
}
func TestCapabilitiesFailClosed(t *testing.T) {
	c := testCaps()
	c.Simulated = false
	c.Runtime = "SavageWind-HyruleM"
	if e := c.Require(false); e != nil {
		t.Fatal(e)
	}
	c.NativeActors = false
	if e := c.Require(false); e == nil {
		t.Fatal("missing native actor port accepted")
	}
	if e := testCaps().Require(false); e == nil {
		t.Fatal("simulator pretended to be native")
	}
	w, _ := NewWorld(Config{Catalog: testCatalog(), AllowSimulation: true})
	if _, e := w.Join("Fresh", "Fresh", testCaps()); e != nil {
		t.Fatal(e)
	}
	s, _ := w.Snapshot("Fresh")
	if s.Personal.Initialized || s.Personal.MaxHealth != 0 {
		t.Fatal("uninitialized game would be overwritten")
	}
	foreign := &Personal{ID: "Other", Health: 12, MaxHealth: 12}
	if w.Apply("Fresh", Command{Sequence: 1, Kind: "seed", Personal: foreign}).Accepted {
		t.Fatal("bootstrap of another player accepted")
	}
}
