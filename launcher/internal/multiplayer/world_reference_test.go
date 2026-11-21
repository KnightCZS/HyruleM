package multiplayer

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestWorldReferencesUseDeliveredBaseAndOriginalTime(t *testing.T) {
	frame := testWorldFrame(1, 7)
	actors := make([]WorldActor, MaxWorldActors)
	for i := range actors {
		actors[i] = testWorldActor(uint64(i+1), "Npc_Test", 3430+float32(i))
	}
	frame.Actors = encodeWorldActors(actors)
	full := Message{Type: "snapshot", Protocol: Protocol, WorldID: strings.Repeat("a", 24), HostID: "host", Epoch: 1, WorldState: &frame}
	now := time.Now()
	var writer, reader WorldView
	var legacy, optimized bytes.Buffer
	for i := 0; i < 5; i++ {
		full.Sequence = uint64(i + 1)
		compact := referenceWorld(full, &writer)
		if err := WriteMessage(&legacy, full); err != nil {
			t.Fatal(err)
		}
		if err := WriteMessage(&optimized, compact); err != nil {
			t.Fatal(err)
		}
		var err error
		reader, err = receivedWorld(compact, reader, now.Add(time.Duration(i)*20*time.Millisecond))
		if err != nil || reader.State.Actors != frame.Actors || !reader.At.Equal(now) {
			t.Fatal(reader.At, err)
		}
	}
	t.Logf("5 world snapshots, 64 actors: legacy=%d bytes, negotiated=%d bytes", legacy.Len(), optimized.Len())
	if optimized.Len()*4 > legacy.Len() {
		t.Fatal("duplicate world bytes")
	}
	ref := referenceWorld(full, &writer)
	for _, mutate := range []func(*Message){func(m *Message) { m.WorldRef++ }, func(m *Message) { m.WorldState = &frame }, func(m *Message) { m.Epoch++ }, func(m *Message) { m.HostID = "other" }, func(m *Message) { m.WorldID = strings.Repeat("b", 24) }, func(m *Message) { m.Protocol = Protocol }} {
		invalid := ref
		mutate(&invalid)
		if _, err := receivedWorld(invalid, reader, now); err == nil {
			t.Fatal("accepted invalid reference")
		}
	}
	empty := full
	empty.WorldState = nil
	reset := referenceWorld(empty, &writer)
	reader, err := receivedWorld(reset, reader, now)
	if err != nil || reader.State != nil {
		t.Fatal("world survived withdrawal", err)
	}
	if _, err = receivedWorld(ref, reader, now); err == nil {
		t.Fatal("reference resurrected withdrawn frame")
	}
	if referenceWorld(full, &writer).WorldState == nil {
		t.Fatal("missing fresh baseline")
	}
}

func TestWorldReferenceOptInOverTCP(t *testing.T) {
	s := testServer(t)
	host := testClient(t, s, "Host")
	conn, err := net.DialTimeout("tcp", s.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(4 * time.Second))
	r := bufio.NewReaderSize(conn, MaxLine)
	write := func(m Message) {
		t.Helper()
		if err := WriteMessage(conn, m); err != nil {
			t.Fatal(err)
		}
	}
	read := func() Message {
		t.Helper()
		m, err := ReadMessage(r)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	write(Message{Type: "hello", Name: "Legacy", Protocol: Protocol})
	if m := read(); m.Type != "welcome" || m.Protocol != Protocol {
		t.Fatal(m)
	}
	eventually(t, func() bool { return host.WorldActors().Epoch != 0 })
	w := testWorldFrame(host.WorldActors().Epoch, 1)
	if err := host.Publish(Pose{Ready: true, Position: w.Center}); err != nil {
		t.Fatal(err)
	}
	if err := host.PublishWorldActors(w); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; {
		m := read()
		if m.WorldRef != 0 || m.Protocol != Protocol {
			t.Fatal("legacy client received extension")
		}
		if m.WorldState != nil {
			i++
		}
	}
	write(Message{Type: "ping", Protocol: WorldReferenceProtocol})
	first := false
	for {
		m := read()
		if m.Protocol != WorldReferenceProtocol {
			continue
		}
		if m.WorldRef != 0 {
			if !first || m.WorldRef != 1 {
				t.Fatal("no delivered baseline")
			}
			break
		}
		if m.WorldState != nil {
			first = true
		}
	}
	if err := host.Publish(Pose{}); err != nil {
		t.Fatal(err)
	}
	for m := read(); m.WorldRef != 0 || m.WorldState != nil; m = read() {
	}
}

func TestWorldBridgeSkipsDuplicateAndReleasesOnce(t *testing.T) {
	now := time.Now()
	w := testWorldFrame(1, 1)
	g := &fakeNativeGame{fakeGame: &fakeGame{}}
	c := &Client{connected: true, welcome: Welcome{ID: "follower"}, world: WorldView{HostID: "host", WorldID: strings.Repeat("a", 24), Epoch: 1, At: now, State: &w}}
	b := &Bridge{Client: c, control: g, nativeSession: "0123456789abcdef"}
	var n worldActorSync
	for _, d := range []time.Duration{0, 200 * time.Millisecond, 2 * time.Second, 3 * time.Second} {
		if err := b.updateWorldActors(context.Background(), &n, now.Add(d)); err != nil {
			t.Fatal(err)
		}
	}
	if len(g.commands) != 2 || !strings.HasSuffix(g.commands[0], w.Actors) || !strings.HasSuffix(g.commands[1], " 0 0 0 0 0 0 -") {
		t.Fatal("duplicate native calls", len(g.commands))
	}
}

func TestServerNamesAndHostInterface(t *testing.T) {
	for host, want := range map[string]string{"localhost": "localhost:5050", "hyrule-rp.example.": "hyrule-rp.example.:5050", "192.0.2.1": "192.0.2.1:5050", "[::1]": "[::1]:5050", "fe80::1%Wi-Fi": "[fe80::1%Wi-Fi]:5050"} {
		if got, err := Address(host, "5050"); err != nil || got != want {
			t.Fatal(host, got, err)
		}
	}
	for _, host := range []string{"[]", "[localhost]", "https://rp.example", "host:5050", "a..b", "-bad", "999.1.2.3", "a/b", "[::1"} {
		if _, err := Address(host, "5050"); err == nil {
			t.Fatal("accepted", host)
		}
	}
	if _, err := BindAddress("rp.example", "5050"); err == nil {
		t.Fatal("bound DNS as a local interface")
	}
}
