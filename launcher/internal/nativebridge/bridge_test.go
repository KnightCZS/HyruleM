package nativebridge

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hyrulelauncher/internal/gen1"
	core "hyrulelauncher/internal/launcher"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func sampleState() gen1.Snapshot {
	return gen1.Snapshot{Sequence: 1, Personal: gen1.Personal{ID: "self", Initialized: true, Health: 10, MaxHealth: 12, Inventory: []gen1.Item{{ID: "own-weapon", Actor: "Weapon_Test", Count: 1}}}, Players: []gen1.PublicPlayer{
		{ID: "self", Alive: true, Frame: gen1.Frame{World: "MainField", Animation: "idle", Ready: true}},
		{ID: "other", Alive: true, Frame: gen1.Frame{World: "MainField", Animation: "run", Ready: true}},
	}, Enemies: []gen1.Enemy{{ID: "map:enemy", Actor: "Enemy_Test", World: "MainField", Health: 20, Owner: "other", Target: "other", Epoch: 2}}}
}
func TestPlanSeparatesActorsAndAuthority(t *testing.T) {
	s := sampleState()
	p, err := BuildPlan("self", s)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Remotes) != 1 || p.Remotes[0].ID != "other" || p.Personal.ID != "self" || p.Enemies[0].SimulateAI {
		t.Fatal("foreign authority or local actor duplicated", p)
	}
	s.Personal.Inventory[0].Count = 99
	if p.Personal.Inventory[0].Count != 1 {
		t.Fatal("plan aliases transport inventory")
	}
	s.Enemies[0].Owner, s.Enemies[0].Target = "self", "self"
	p, err = BuildPlan("self", s)
	if err != nil || !p.Enemies[0].SimulateAI {
		t.Fatal("nearest owner cannot simulate", err)
	}
	s.Players[1].Frame.World = "Dungeon001"
	p, err = BuildPlan("self", s)
	if err != nil || len(p.Remotes) != 0 {
		t.Fatal("remote actor leaked across maps", err)
	}
	s.Personal.ID = "other"
	if _, err = BuildPlan("self", s); err == nil {
		t.Fatal("applied someone else's personal state")
	}
}
func TestUnseededPlanCannotClearInventory(t *testing.T) {
	s := sampleState()
	s.Personal.Initialized = false
	p, err := BuildPlan("self", s)
	if err != nil || p.Personal != nil || p.WorldReady || len(p.Remotes) != 0 || len(p.Enemies) != 0 {
		t.Fatal("unseeded state used to clear game inventory", err)
	}
	s = sampleState()
	s.Enemies[0].Target = "unknown"
	if _, err := BuildPlan("self", s); err == nil {
		t.Fatal("invalid target accepted")
	}
}

type testBackend struct {
	caps    gen1.Capabilities
	applied []Plan
}

func (b *testBackend) Capabilities(context.Context) (gen1.Capabilities, error) { return b.caps, nil }
func (b *testBackend) ApplyOnGameThread(_ context.Context, p Plan) error {
	b.applied = append(b.applied, p)
	return nil
}
func TestGateRequiresRealNativeCapabilities(t *testing.T) {
	b := &testBackend{}
	g := &Gate{Self: "self", Backend: b}
	if g.Apply(context.Background(), sampleState()) == nil || len(b.applied) != 0 {
		t.Fatal("stock runtime mutated")
	}
	b.caps = gen1.Capabilities{ABI: 1, Runtime: "test", GameVersion: "1.9.0", NativeActors: true, Animation: true, Collision: true, PersonalVitals: true, PersonalInventory: true, PickupHook: true, DamageHook: true, EnemyAuthority: true, EnemyTarget: true, WorldIdentity: true, Simulated: true}
	if g.Apply(context.Background(), sampleState()) == nil {
		t.Fatal("simulated backend passed native gate")
	}
	g.AllowSimulation = true
	if err := g.Apply(context.Background(), sampleState()); err != nil {
		t.Fatal(err)
	}
	if err := g.Apply(context.Background(), sampleState()); err == nil || len(b.applied) != 1 {
		t.Fatal("old plan applied twice")
	}
}
func TestProbeCollectsMetadataAndOnlyReadOnlyCommands(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "extracted", "exefs", "main")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	module := make([]byte, 300)
	copy(module, "NSO0")
	copy(module[0x40:0x60], strings.Repeat("B", 32))
	if err := os.WriteFile(path, module, 0600); err != nil {
		t.Fatal(err)
	}
	meta := FileInfo("main", path)
	hash := sha256.Sum256(module)
	if meta.SHA256 != hex.EncodeToString(hash[:]) || meta.BuildID != hex.EncodeToString(module[0x40:0x60]) {
		t.Fatal("metadata does not match module", meta)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	commands := make(chan []string, 1)
	go func() {
		var got []string
		for i := 0; i < 7; i++ {
			conn, err := listener.Accept()
			if err != nil {
				commands <- got
				return
			}
			scan := bufio.NewScanner(conn)
			if scan.Scan() {
				command := scan.Text()
				got = append(got, command)
				if command == "hm_caps" {
					fmt.Fprintln(conn, `{"ok":false,"error":"unknown command"}`)
				} else {
					fmt.Fprintln(conn, `{"ok":true,"data":{"test":true}}`)
				}
			}
			conn.Close()
		}
		commands <- got
	}()
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	n, _ := strconv.Atoi(port)
	report, err := Probe(context.Background(), core.Config{DataDir: dir}, 0, n)
	if err != nil || report.NativeReady || !strings.Contains(report.NativeError, "unknown command") {
		t.Fatal("unsupported native runtime reported ready", report, err)
	}
	want := []string{"version", "help", "sites", "mem regions", "pos", "camera", "hm_caps"}
	if got := <-commands; !reflect.DeepEqual(got, want) {
		t.Fatal("unexpected probe commands", got)
	}
	data, _ := json.Marshal(report)
	if strings.Contains(string(data), dir) || strings.Contains(string(data), strings.Repeat("B", 32)) {
		t.Fatal("report contains local path or module bytes")
	}
	p, err := ControlPort("[control] listening on 127.0.0.1:12000\n[control] listening on 127.0.0.1:12001\n")
	if p != 12001 || err != nil {
		t.Fatal("wrong endpoint", p, err)
	}
}
