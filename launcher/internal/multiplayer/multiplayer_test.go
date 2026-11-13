package multiplayer

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	core "hyrulelauncher/internal/launcher"
	"math"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func eventually(t *testing.T, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatal("condition not reached")
}
func testServer(t *testing.T) *Server {
	t.Helper()
	s, e := Listen("127.0.0.1:0", ServerConfig{Spawn: Vec3{3437.5, 235.9, 2101.5}, Tick: 20 * time.Millisecond})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func testClient(t *testing.T, s *Server, name string) *Client {
	t.Helper()
	c, e := Connect(context.Background(), s.Addr().String(), name, nil)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(c.Close)
	return c
}
func TestTwoClientsMovementRenameDisconnectAndSlotReuse(t *testing.T) {
	s := testServer(t)
	a := testClient(t, s, "Link A")
	b := testClient(t, s, "Zelda B")
	if a.Welcome().Slot == b.Welcome().Slot || a.Welcome().ID == b.Welcome().ID {
		t.Fatal("duplicate identity")
	}
	if a.Welcome().Spawn.Sub(b.Welcome().Spawn).Len() != 4 {
		t.Fatal("unsafe spawn overlap")
	}
	p := Pose{Position: Vec3{3439, 236, 2101}, Velocity: Vec3{2, 0, 0}, Yaw: 1, Phase: 2, Ready: true}
	if e := a.Publish(p); e != nil {
		t.Fatal(e)
	}
	eventually(t, func() bool {
		r := b.Remote(time.Now())
		return len(r) == 1 && r[0].Name == "Link A" && r[0].Pose.Position == p.Position
	})
	if len(a.Remote(time.Now())) != 0 {
		t.Fatal("unready peer leaked")
	}
	if e := a.Rename("Henri"); e != nil {
		t.Fatal(e)
	}
	eventually(t, func() bool { r := b.Remote(time.Now()); return len(r) == 1 && r[0].Name == "Henri" })
	if c, e := Connect(context.Background(), s.Addr().String(), "Terceiro", nil); e == nil {
		c.Close()
		t.Fatal("third client accepted")
	}
	a.Close()
	eventually(t, func() bool { return len(b.Remote(time.Now())) == 0 && s.Count() == 1 })
	c := testClient(t, s, "Novo")
	if c.Welcome().Slot != a.Welcome().Slot {
		t.Fatal("slot not reusable")
	}
	s.Close()
	eventually(t, func() bool { return !b.Connected() && !c.Connected() })
}
func TestMalformedAndInvalidState(t *testing.T) {
	for _, line := range []string{"{broken}\n", `{"type":"hello","unknown":1}` + "\n", strings.Repeat("x", MaxLine+1) + "\n", `{"type":"ping"} {}` + "\n"} {
		if _, e := ReadMessage(bufio.NewReaderSize(strings.NewReader(line), MaxLine)); e == nil {
			t.Fatal("accepted malformed input")
		}
	}
	for _, name := range []string{"", "a\nb", strings.Repeat("á", 25), "a\u200bb"} {
		if _, e := NormalizeName(name); e == nil {
			t.Fatal("invalid name accepted")
		}
	}
	if (Pose{Position: Vec3{X: math.NaN()}}).Valid() {
		t.Fatal("NaN accepted")
	}
	s := testServer(t)
	c := testClient(t, s, "One")
	c.send(Message{Type: "pose", Pose: &Pose{Position: Vec3{X: 1e8}}})
	eventually(t, func() bool { return s.Count() == 0 && !c.Connected() })
}
func TestInterpolationAndProjection(t *testing.T) {
	now := time.Now()
	p := Player{ID: "remote", Name: "Remote", Slot: 1, Pose: Pose{Ready: true, Position: Vec3{Z: -10}}}
	old := p
	old.Pose.Position.X = -2
	c := &Client{connected: true, welcome: Welcome{ID: "self"}, frames: []Frame{{now.Add(-150 * time.Millisecond), []Player{old}}, {now.Add(-50 * time.Millisecond), []Player{p}}}}
	r := c.Remote(now)
	if len(r) != 1 || math.Abs(r[0].Pose.Position.X+1) > 1e-6 {
		t.Fatal("interpolation", r)
	}
	cam := Camera{Eye: Vec3{Y: 2}, Forward: Vec3{Z: -1}, Up: Vec3{Y: 1}, FOV: 50, At: now}
	point, _, ok := cam.Project(Vec3{Y: 2, Z: -10}, 1280, 720)
	if !ok || point.X != 640 || point.Y != 360 {
		t.Fatal("projection center")
	}
	point, _, _ = cam.Project(Vec3{X: 1, Y: 2, Z: -10}, 1280, 720)
	if point.X <= 640 {
		t.Fatal("mirrored camera")
	}
	if _, _, ok := cam.Project(Vec3{Z: 10}, 1280, 720); ok {
		t.Fatal("behind eye")
	}
	draw := BuildDrawList(cam, r, 1280, 720, now)
	if len(draw.Triangles) < 100 || len(draw.Labels) != 1 {
		t.Fatal("avatar missing")
	}
	if len(BuildDrawList(cam, r, 1280, 720, now.Add(2*time.Second)).Triangles) != 0 {
		t.Fatal("stale camera draws")
	}
	if out := os.Getenv("HYRULEM_PREVIEW"); out != "" {
		f, e := os.Create(out)
		if e != nil {
			t.Fatal(e)
		}
		WriteSVG(f, draw, 1280, 720)
		f.Close()
	}
}

// Fake control has real runtime JSON shapes; bridges, TCP relay, interpolation
// and renderer are real. This is not a claim that BOTW was run in CI.
type fakeGame struct {
	mu         sync.Mutex
	position   Vec3
	commands   []string
	closed     bool
	notLoaded  bool
	spawnError bool
}

func (f *fakeGame) Close() { f.mu.Lock(); f.closed = true; f.mu.Unlock() }
func (f *fakeGame) Command(ctx context.Context, cmd string) (core.ControlReply, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commands = append(f.commands, cmd)
	if cmd == "pos" && f.notLoaded {
		return core.ControlReply{}, fmt.Errorf("No world loaded")
	}
	var data any
	switch {
	case cmd == "pos":
		data = f.position
	case cmd == "camera":
		data = map[string]float64{"x": f.position.X, "y": f.position.Y + 2, "z": f.position.Z + 10, "dx": 0, "dy": 0, "dz": -1}
	case strings.HasPrefix(cmd, "goto "):
		if f.spawnError {
			return core.ControlReply{}, fmt.Errorf("teleport unavailable")
		}
		fmt.Sscanf(cmd, "goto %f %f %f", &f.position.X, &f.position.Y, &f.position.Z)
		data = map[string]string{"state": "already_there"}
	case strings.HasPrefix(cmd, "set "):
		data = map[string]bool{"applied": true}
	default:
		return core.ControlReply{}, fmt.Errorf("not available")
	}
	b, _ := json.Marshal(data)
	return core.ControlReply{OK: true, Data: b}, nil
}
func TestBridgesTwoGameControlsToAvatars(t *testing.T) {
	s := testServer(t)
	a := testClient(t, s, "A")
	b := testClient(t, s, "B")
	ga := &fakeGame{position: Vec3{3437, 236, 2101}}
	gb := &fakeGame{position: Vec3{3437, 236, 2101}}
	ba := NewBridge(context.Background(), a, ga, 40, nil)
	bb := NewBridge(context.Background(), b, gb, 40, nil)
	defer ba.Close()
	defer bb.Close()
	eventually(t, func() bool { return len(a.Remote(time.Now())) == 1 && len(b.Remote(time.Now())) == 1 })
	ga.mu.Lock()
	ga.position.X += 3
	wanted := ga.position.X
	ga.mu.Unlock()
	eventually(t, func() bool {
		r := b.Remote(time.Now())
		return len(r) == 1 && math.Abs(r[0].Pose.Position.X-wanted) < .05
	})
	now := time.Now()
	draw := BuildDrawList(bb.Camera(), b.Remote(now), 1280, 720, now)
	if len(draw.Triangles) == 0 {
		t.Fatal("second game cannot project first avatar")
	}
	ba.Close()
	select {
	case <-ba.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("bridge did not close")
	}
	eventually(t, func() bool { return len(b.Remote(time.Now())) == 0 })
	ga.mu.Lock()
	defer ga.mu.Unlock()
	if !ga.closed {
		t.Fatal("control stream leaked")
	}
}
func TestServerCloseDuringHandshake(t *testing.T) {
	s := testServer(t)
	c, e := net.Dial("tcp", s.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	done := make(chan struct{})
	go func() { s.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("unidentified socket blocks shutdown")
	}
}

func TestLoadingAndFailedTeleportDoNotDisconnect(t *testing.T) {
	s := testServer(t)
	a := testClient(t, s, "Loading")
	b := testClient(t, s, "Observer")
	game := &fakeGame{position: Vec3{3437, 236, 2101}, notLoaded: true, spawnError: true}
	bridge := NewBridge(context.Background(), a, game, 40, nil)
	defer bridge.Close()
	// Longer than both the hello timeout and the server's idle deadline.
	time.Sleep(8500 * time.Millisecond)
	if !a.Connected() || s.Count() != 2 {
		t.Fatal("loading dropped network", a.Reason())
	}
	game.mu.Lock()
	game.notLoaded = false
	game.mu.Unlock()
	eventually(t, func() bool { return len(b.Remote(time.Now())) == 1 })
	if !a.Connected() {
		t.Fatal("failed teleport disconnected player", a.Reason())
	}
	game.mu.Lock()
	game.spawnError = false
	game.mu.Unlock()
	bridge.RequestSpawn()
	eventually(t, func() bool {
		game.mu.Lock()
		defer game.mu.Unlock()
		return game.position.Sub(a.Welcome().Spawn).Len() < 0.001
	})
}
func TestListenOnComputerInterfaceWithChosenPort(t *testing.T) {
	addresses, e := LocalAddresses()
	if e != nil {
		t.Logf("adapter enumeration unavailable in this runtime: %v; still testing TCP listener", e)
	}
	for _, a := range addresses {
		address, e := BindAddress(a.IP, "5057")
		if e != nil || address != net.JoinHostPort(a.IP, "5057") {
			t.Fatal(address, e)
		}
	}
	if _, e := BindAddress("192.0.2.123", "5057"); e == nil {
		t.Fatal("non-local hosting address accepted")
	}
	s, e := Listen("0.0.0.0:0", ServerConfig{Spawn: Vec3{X: 3437, Y: 236, Z: 2101}})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	port := s.Addr().(*net.TCPAddr).Port
	host := "127.0.0.1"
	if len(addresses) > 0 {
		host = addresses[0].IP
	}
	c, e := Connect(context.Background(), net.JoinHostPort(host, fmt.Sprint(port)), "LAN client", nil)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	if !c.Connected() {
		t.Fatal("could not connect through local network adapter")
	}
}
