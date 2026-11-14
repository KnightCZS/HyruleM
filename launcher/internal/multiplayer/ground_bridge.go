package multiplayer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type groundSync struct {
	last, heartbeat time.Time
	world           string
	sent            map[uint64]string
	closed          map[uint64]bool
	receiptAt       map[uint64]time.Time
}

func (b *Bridge) closeGround(ctx context.Context, n *groundSync) bool {
	if n.world != "" {
		if b.command(ctx, "hm_items_close "+b.nativeSession+" "+n.world, nil) != nil {
			return false
		}
	}
	n.world = ""
	return true
}

// The active native session is supplied explicitly; no identity travels as a
// process address. Separate fields allow ordinary presence without this layer.

func groundCommand(session, world, self string, item GroundItem) (string, error) {
	if !item.Valid() || len(session) != 16 || len(world) != 24 {
		return "", fmt.Errorf("estado de material inválido")
	}
	state := map[string]int{"available": 0, "reserved": 1, "taken": 2, "uncertain": 3}[item.State]
	request := uint64(0)
	if item.Claimant == self && item.State == "reserved" {
		request = item.Request
	}
	g := item.Source
	cmd := fmt.Sprintf("hm_item_state %s %s %d %s %d %.3f %.3f %.3f %.3f %.3f %.3f %d %d %d %d", session, world, item.ID, g.Actor, g.Hash, g.Spawn.X, g.Spawn.Y, g.Spawn.Z, g.Position.X, g.Position.Y, g.Position.Z, g.Local, boolInt(item.Owner == self), state, request)
	if len(cmd) > 511 {
		return "", fmt.Errorf("estado de material excede limite")
	}
	return cmd, nil
}
func (b *Bridge) updateGround(ctx context.Context, n *groundSync, now time.Time) error {
	if now.Sub(n.last) < 250*time.Millisecond {
		return nil
	}
	n.last = now
	v := b.Client.Ground()
	if !v.Enabled || now.Sub(v.At) > 2*time.Second {
		b.closeGround(ctx, n)
		return nil
	}
	session := b.nativeSession
	if n.world != v.World {
		if !b.closeGround(ctx, n) {
			return fmt.Errorf("limpeza dos materiais pendente")
		}
		n.sent = map[uint64]string{}
		n.closed = map[uint64]bool{}
		n.receiptAt = map[uint64]time.Time{}
		n.heartbeat = time.Time{}
	}
	if n.world != v.World || now.Sub(n.heartbeat) >= time.Second {
		if err := b.command(ctx, fmt.Sprintf("hm_items_open %s %s %.3f %.3f %.3f", session, v.World, v.Zone.X, v.Zone.Y, v.Zone.Z), nil); err != nil {
			return err
		}
		n.world = v.World
		n.heartbeat = now
	}
	var status struct {
		Items    string `json:"items_json"`
		Events   string `json:"events_json"`
		Receipts string `json:"receipts_json"`
	}
	if err := b.command(ctx, "hm_items_status", &status); err != nil {
		return err
	}
	var observations []GroundSource
	var events []GroundClaim
	var receipts []GroundReceipt
	if err := json.Unmarshal([]byte(status.Items), &observations); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(status.Events), &events); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(status.Receipts), &receipts); err != nil {
		return err
	}
	if len(events) > MaxGroundItems || len(receipts) > MaxGroundItems {
		return fmt.Errorf("fila nativa de materiais excessiva")
	}
	if err := b.Client.PublishGround(observations); err != nil {
		return err
	}
	for _, event := range events {
		if err := b.Client.ClaimGround(event); err != nil {
			return err
		}
	}
	for _, receipt := range receipts {
		pending := false
		for _, item := range v.Items {
			if item.ID == receipt.Item && item.Claimant == b.Client.Welcome().ID && item.Request == receipt.Request && (item.State == "reserved" || item.State == "uncertain") {
				pending = true
				break
			}
		}
		if pending && now.Sub(n.receiptAt[receipt.Request]) >= time.Second {
			if err := b.Client.ReceiptGround(receipt); err != nil {
				return err
			}
			n.receiptAt[receipt.Request] = now
		}
	}
	budget := 8
	for _, claim := range v.Claims {
		if claim.Status != "taken" && claim.Status != "denied" && claim.Status != "failed" {
			continue
		}
		if !n.closed[claim.Request] && budget > 0 {
			if err := b.command(ctx, fmt.Sprintf("hm_item_deny %s %s %d", session, v.World, claim.Request), nil); err != nil {
				return err
			}
			n.closed[claim.Request] = true
			budget--
		}
	}
	items := append([]GroundItem(nil), v.Items...)
	self := b.Client.Welcome().ID
	sort.SliceStable(items, func(i, j int) bool { return items[i].Claimant == self && items[j].Claimant != self })
	for _, item := range items {
		cmd, err := groundCommand(session, v.World, self, item)
		if err != nil {
			return err
		}
		if n.sent[item.ID] == cmd {
			continue
		}
		if budget == 0 {
			break
		}
		if err = b.command(ctx, cmd, nil); err != nil {
			return err
		}
		n.sent[item.ID] = cmd
		budget--
	}
	return nil
}
