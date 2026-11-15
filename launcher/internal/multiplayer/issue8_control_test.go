package multiplayer

import (
	"bufio"
	"context"
	"encoding/json"
	core "hyrulelauncher/internal/launcher"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// Use the production loopback transport: a fake GameControl alone bypasses
// ControlStream's command-size gate and missed issue #8 in preview 12.
func gameSocket(t *testing.T, game *fakeNativeGame) *core.ControlStream {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	connections := map[net.Conn]bool{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			connections[conn] = true
			mu.Unlock()
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer conn.Close()
				defer func() { mu.Lock(); delete(connections, conn); mu.Unlock() }()
				reader := bufio.NewReaderSize(conn, 128*1024)
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					reply, err := game.Command(context.Background(), strings.TrimSuffix(line, "\n"))
					if err != nil {
						reply = core.ControlReply{OK: false, Error: json.RawMessage(`"synthetic game error"`)}
					}
					if json.NewEncoder(conn).Encode(reply) != nil {
						return
					}
				}
			}()
		}
	}()
	stream := &core.ControlStream{Port: listener.Addr().(*net.TCPAddr).Port, Timeout: time.Second}
	t.Cleanup(func() {
		stream.Close()
		listener.Close()
		mu.Lock()
		for c := range connections {
			c.Close()
		}
		mu.Unlock()
		wg.Wait()
	})
	return stream
}

func TestIssue8NativeSampleThroughProductionControlStream(t *testing.T) {
	t.Setenv(core.NativePreviewEnv, "1")
	server := testServer(t)
	a, b := testClient(t, server, "A"), testClient(t, server, "B")
	bones := make([]PoseBone, MaxPoseBones)
	for i := range bones {
		bones[i] = testBone(uint32(i+1), float32(i%10))
	}
	skeleton := encodeSkeleton(bones)
	effects := make([]VisualEffect, MaxVisualEffects)
	for i := range effects {
		effects[i] = VisualEffect{Sequence: uint64(i + 1), Slot: i % 4, Active: true, Name: "AttackSpin"}
	}
	ga := &fakeNativeGame{fakeGame: &fakeGame{position: Vec3{3430, 236, 2100}}, skeleton: skeleton, effects: effects}
	gb := &fakeNativeGame{fakeGame: &fakeGame{position: Vec3{3431, 236, 2100}}, skeleton: skeleton, effects: effects}
	ba := NewBridge(context.Background(), a, gameSocket(t, ga), 40, nil)
	bb := NewBridge(context.Background(), b, gameSocket(t, gb), 40, nil)
	t.Cleanup(func() { ba.Close(); bb.Close(); <-ba.Done(); <-bb.Done() })
	received := func(game *fakeNativeGame) bool {
		game.mu.Lock()
		defer game.mu.Unlock()
		for _, cmd := range game.commands {
			if strings.HasPrefix(cmd, "hm_native_sample ") && len(cmd) > 30000 && strings.Count(cmd, "AttackSpin") == MaxVisualEffects && strings.Contains(cmd, skeleton) {
				return true
			}
		}
		return false
	}
	eventually(t, func() bool { return received(ga) && received(gb) })
	if !a.Connected() || !b.Connected() {
		t.Fatal(a.Reason(), b.Reason())
	}
}
