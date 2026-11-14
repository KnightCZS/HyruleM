package multiplayer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	core "hyrulelauncher/internal/launcher"
	"io"
	"strings"
	"testing"
	"time"
)

type fragmentedReader struct{ reader io.Reader }

func (r fragmentedReader) Read(p []byte) (int, error) {
	if len(p) > 3 {
		p = p[:3]
	}
	return r.reader.Read(p)
}
func TestWorldMessagesSurviveTCPFragmentation(t *testing.T) {
	e := EnemyHealth{Hash: 42, Actor: "Enemy_Bokoblin_Junior", Spawn: Vec3{1, 2, 3}, Position: Vec3{1, 2, 3}, Health: 30}
	var wire bytes.Buffer
	if err := WriteMessage(&wire, Message{Type: "enemy_health", Enemies: []EnemyHealth{e}}); err != nil {
		t.Fatal(err)
	}
	if err := WriteMessage(&wire, Message{Type: "ping"}); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReaderSize(fragmentedReader{&wire}, MaxLine)
	a, err := ReadMessage(reader)
	if err != nil || len(a.Enemies) != 1 || a.Enemies[0] != e {
		t.Fatal("fragmented state corrupted", a, err)
	}
	b, err := ReadMessage(reader)
	if err != nil || b.Type != "ping" {
		t.Fatal("framing consumed following message", b, err)
	}
}

func TestSharedMapEnemyHealthConvergesAcrossTwoClients(t *testing.T) {
	s := testServer(t)
	a := testClient(t, s, "A")
	b := testClient(t, s, "B")
	pose := Pose{Ready: true, Position: Vec3{3430, 236, 2100}}
	if err := a.Publish(pose); err != nil {
		t.Fatal(err)
	}
	if err := b.Publish(pose); err != nil {
		t.Fatal(err)
	}
	e := EnemyHealth{Hash: 42, Actor: "Enemy_Bokoblin_Junior", Spawn: pose.Position, Position: pose.Position, Health: 100}
	if err := a.PublishEnemyHealth([]EnemyHealth{e}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { r := b.SharedEnemyHealth(); return len(r) == 1 && r[0].Health == 100 })
	e.Health = 60
	if err := b.PublishEnemyHealth([]EnemyHealth{e}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { r := a.SharedEnemyHealth(); return len(r) == 1 && r[0].Health == 60 })
	e.Health = 100
	_ = a.PublishEnemyHealth([]EnemyHealth{e}) // delayed pre-hit observation cannot heal
	eventually(t, func() bool { r := b.SharedEnemyHealth(); return len(r) == 1 && r[0].Health == 60 })
	e.Health = 0
	_ = a.PublishEnemyHealth([]EnemyHealth{e})
	eventually(t, func() bool { r := b.SharedEnemyHealth(); return len(r) == 1 && r[0].Health == 0 })
}

func TestEnemyIdentityAndBoundsPreventCrossActorWrites(t *testing.T) {
	p := Pose{Ready: true, Position: Vec3{10, 20, 30}}
	e := EnemyHealth{Hash: 22, Actor: "Enemy_Bokoblin_Junior", Spawn: p.Position, Position: p.Position, Health: 50}
	registry := map[string]EnemyHealth{}
	if err := mergeEnemyHealth(registry, []EnemyHealth{e}, p); err != nil {
		t.Fatal(err)
	}
	other := e
	other.Spawn.X += 10
	other.Health = 100
	if err := mergeEnemyHealth(registry, []EnemyHealth{other}, p); err != nil || len(registry) != 2 {
		t.Fatal("placement identity lost", err)
	}
	bad := e
	bad.Actor = "GameROMPlayer"
	if err := mergeEnemyHealth(registry, []EnemyHealth{bad}, p); err == nil {
		t.Fatal("local player life accepted")
	}
	bad = e
	bad.Position.X += 1000
	if err := mergeEnemyHealth(registry, []EnemyHealth{bad}, p); err == nil {
		t.Fatal("distant unobserved enemy accepted")
	}
	if err := mergeEnemyHealth(registry, []EnemyHealth{e, e}, p); err == nil {
		t.Fatal("duplicate map actor accepted")
	}
	if len(registry) != 2 {
		t.Fatal("failed validation partially modified registry")
	}
}

type worldControl struct {
	commands []string
	health   int
	enemies  []EnemyHealth
}

func (g *worldControl) Close() {}
func (g *worldControl) Command(ctx context.Context, cmd string) (core.ControlReply, error) {
	g.commands = append(g.commands, cmd)
	e := EnemyHealth{Hash: 42, Actor: "Enemy_Bokoblin_Junior", Spawn: Vec3{1, 2, 3}, Position: Vec3{1, 2, 3}, Health: g.health}
	enemies := g.enemies
	if enemies == nil {
		enemies = []EnemyHealth{e}
	}
	b, _ := json.Marshal(enemies)
	data, _ := json.Marshal(map[string]string{"enemies_json": string(b)})
	return core.ControlReply{OK: true, Data: data}, nil
}
func TestNativeWorldBridgeWritesOnlyLowerLifeForMatchingLoadedEnemy(t *testing.T) {
	s := testServer(t)
	a := testClient(t, s, "A")
	other := testClient(t, s, "B")
	p := Pose{Ready: true, Position: Vec3{1, 2, 3}}
	_ = a.Publish(p)
	_ = other.Publish(p)
	e := EnemyHealth{Hash: 42, Actor: "Enemy_Bokoblin_Junior", Spawn: p.Position, Position: p.Position, Health: 20}
	_ = other.PublishEnemyHealth([]EnemyHealth{e})
	eventually(t, func() bool { return len(a.SharedEnemyHealth()) == 1 })
	g := &worldControl{health: 100}
	bridge := &Bridge{Client: a, control: g}
	n := &nativePreview{session: "0123456789abcdef"}
	if err := bridge.updateWorldHealth(context.Background(), n, time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(g.commands) != 2 || !strings.HasPrefix(g.commands[1], "hm_enemy_health 0123456789abcdef 42 Enemy_Bokoblin_Junior ") || !strings.HasSuffix(g.commands[1], " 20") {
		t.Fatal(g.commands)
	}
	g.commands = nil
	g.health = 10
	n.lastWorld = time.Time{}
	if err := bridge.updateWorldHealth(context.Background(), n, time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(g.commands) != 1 {
		t.Fatal("stale snapshot tried to heal enemy", g.commands)
	}
}

func TestHealthControlBudgetDoesNotBlockVisualFrames(t *testing.T) {
	s := testServer(t)
	a, b := testClient(t, s, "A"), testClient(t, s, "B")
	p := Pose{Ready: true, Position: Vec3{1, 2, 3}}
	_ = a.Publish(p)
	_ = b.Publish(p)
	shared := []EnemyHealth{}
	local := []EnemyHealth{}
	for i := 1; i <= 7; i++ {
		e := EnemyHealth{Hash: uint32(i), Actor: "Enemy_Bokoblin_Junior", Spawn: p.Position, Position: p.Position, Health: 20}
		shared = append(shared, e)
		e.Health = 100
		local = append(local, e)
	}
	_ = b.PublishEnemyHealth(shared)
	eventually(t, func() bool { return len(a.SharedEnemyHealth()) == 7 })
	g := &worldControl{enemies: local}
	bridge := &Bridge{Client: a, control: g}
	n := &nativePreview{session: "0123456789abcdef"}
	for cycle, want := range []int{5, 4, 1} {
		g.commands = nil
		n.lastWorld = time.Time{}
		if err := bridge.updateWorldHealth(context.Background(), n, time.Now()); err != nil {
			t.Fatal(err)
		}
		if len(g.commands) != want {
			t.Fatalf("cycle %d commands=%v", cycle, g.commands)
		}
	}
}
