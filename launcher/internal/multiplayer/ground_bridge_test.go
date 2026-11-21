package multiplayer

import (
	"context"
	"encoding/json"
	"fmt"
	core "hyrulelauncher/internal/launcher"
	"strings"
	"testing"
	"time"
)

type groundGame struct {
	scene                 string
	observations          []GroundSource
	events                []GroundClaim
	receipts              []GroundReceipt
	commands              []string
	dropOrigin            string
	dropRequest, dropItem uint64
	dropResult            int
	dropSource            string
}

func (g *groundGame) Close() {}
func (g *groundGame) Command(ctx context.Context, cmd string) (core.ControlReply, error) {
	g.commands = append(g.commands, cmd)
	data := map[string]any{"queued": true, "ready": true}
	if cmd == "hm_items_status" {
		a, _ := json.Marshal(g.observations)
		e, _ := json.Marshal(g.events)
		r, _ := json.Marshal(g.receipts)
		if g.observations == nil {
			a = []byte("[]")
		}
		if g.events == nil {
			e = []byte("[]")
		}
		if g.receipts == nil {
			r = []byte("[]")
		}
		data = map[string]any{"scene": g.scene, "items_json": string(a), "events_json": string(e), "receipts_json": string(r), "drop_origin": g.dropOrigin, "drop_request": g.dropRequest, "drop_item": g.dropItem, "drop_result": g.dropResult, "drop_source_json": g.dropSource}
	}
	b, _ := json.Marshal(data)
	return core.ControlReply{OK: true, Data: b}, nil
}
func TestGroundBridgeGrantsOnlyAfterReservationAndRepeatsReceiptUntilCommit(t *testing.T) {
	s := groundServer(t)
	host, guest := testClient(t, s, "Host"), testClient(t, s, "Guest")
	p := Pose{Ready: true, Position: s.cfg.Spawn}
	_ = host.Publish(p)
	_ = guest.Publish(p)
	eventually(t, func() bool { return host.Ground().Enabled && guest.Ground().Enabled })
	src := GroundSource{Local: 1, Hash: 123, Actor: "Item_Fruit_A", Position: p.Position, Spawn: p.Position}
	hg := &groundGame{observations: []GroundSource{src}}
	gg := &groundGame{}
	a := &Bridge{roamingGround: true, Client: host, control: hg, nativeSession: "0123456789abcdef"}
	b := &Bridge{roamingGround: true, Client: guest, control: gg, nativeSession: "abcdef0123456789"}
	an, bn := &groundSync{}, &groundSync{}
	step := func(b *Bridge, n *groundSync) {
		t.Helper()
		n.last = time.Time{}
		if e := b.updateGround(context.Background(), n, time.Now()); e != nil {
			t.Fatal(e)
		}
	}
	step(a, an)
	eventually(t, func() bool { return len(guest.Ground().Items) == 1 })
	step(b, bn)
	gg.events = []GroundClaim{{Request: 1, Item: 1, Source: src}}
	step(b, bn)
	eventually(t, func() bool { return len(guest.Ground().Claims) == 1 && guest.Ground().Claims[0].Status == "granted" })
	step(b, bn)
	found := false
	for _, cmd := range gg.commands {
		if strings.HasPrefix(cmd, "hm_item_state ") && strings.Contains(cmd, " 1 1 0 -") {
			found = true
		}
	}
	if !found {
		t.Fatal("reserved grant not delivered", gg.commands)
	}
	// Same still-pending native request must not disconnect on server-resolved ID.
	gg.events[0].Item = 0
	step(b, bn)
	eventually(t, func() bool { return guest.Connected() })
	gg.receipts = []GroundReceipt{{Item: 1, Request: 1, Applied: true}}
	step(b, bn)
	eventually(t, func() bool { return host.Ground().Items[0].State == "taken" })
	step(b, bn)
	closed := false
	for _, cmd := range gg.commands {
		if cmd == fmt.Sprintf("hm_item_deny %s %s 1", b.nativeSession, bn.world) {
			closed = true
		}
	}
	if !closed {
		t.Fatal("completed native event not closed")
	}
	if !b.closeGround(context.Background(), bn) || bn.world != "" {
		t.Fatal("ground session retained")
	}
}

func TestGroundBridgePreparesDropBeforeNativeDebitAndCommitsBothViews(t *testing.T) {
	s := groundServer(t)
	host, guest := testClient(t, s, "Host"), testClient(t, s, "Guest")
	p := Pose{Ready: true, Position: s.cfg.Spawn}
	_ = host.Publish(p)
	_ = guest.Publish(p)
	eventually(t, func() bool {
		_, ready := host.LocalPose()
		return ready && host.Ground().Enabled && guest.Ground().Enabled
	})
	game := &groundGame{dropOrigin: "0123456789abcdef"}
	b := &Bridge{roamingGround: true, Client: host, control: game, native: true, nativeSession: game.dropOrigin, dropRequests: make(chan struct{}, 1)}
	n := &groundSync{}
	step := func() {
		t.Helper()
		n.last = time.Time{}
		if err := b.updateGround(context.Background(), n, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	step()
	if !b.RequestAppleDrop() || b.RequestAppleDrop() {
		t.Fatal("drop UI did not enforce one pending request")
	}
	step()
	eventually(t, func() bool { r := host.Ground().DropResult; return r != nil && r.Status == "prepared" })
	step()
	stateIndex, dropIndex := -1, -1
	for i, cmd := range game.commands {
		if strings.HasPrefix(cmd, "hm_item_state ") {
			stateIndex = i
		}
		if strings.HasPrefix(cmd, "hm_item_drop ") {
			dropIndex = i
		}
	}
	if stateIndex < 0 || dropIndex <= stateIndex || !b.DropBusy() {
		t.Fatal("native debit preceded server preparation", game.commands)
	}
	source, _ := json.Marshal(n.dropSource)
	game.dropRequest = n.dropRequest
	game.dropItem = n.dropItem
	game.dropResult = 1
	game.dropSource = string(source)
	step()
	eventually(t, func() bool { v := guest.Ground(); return len(v.Items) == 1 && v.Items[0].State == "available" })
	eventually(t, func() bool { return host.Ground().DropResult.Status == "committed" })
	step()
	if b.DropBusy() || n.dropRequest != 0 || !host.Connected() || !guest.Connected() {
		t.Fatal("committed drop retained pending UI state")
	}
	if !b.closeGround(context.Background(), n) || b.CanDropApple() {
		t.Fatal("drop UI remained enabled after cleanup")
	}
}
