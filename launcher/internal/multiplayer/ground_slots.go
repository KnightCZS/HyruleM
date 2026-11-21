package multiplayer

import (
	"context"
	"fmt"
)

// Global item identities survive region changes. Native slots are a bounded
// working set and cannot be reused until the game has acknowledged cleanup.
func (n *groundSync) globalItem(slot uint64) uint64 {
	for id, at := range n.slots {
		if at == slot {
			return id
		}
	}
	return 0
}
func (b *Bridge) updateGroundSlots(ctx context.Context, n *groundSync, view GroundView) error {
	if n.slots == nil {
		n.slots = map[uint64]uint64{}
	}
	if n.retiring == nil {
		n.retiring = map[uint64]bool{}
	}
	wanted := map[uint64]bool{}
	for _, item := range view.Items {
		wanted[item.ID] = true
	}
	budget := 4
	for id, slot := range n.slots {
		if wanted[id] && !n.retiring[id] || id == n.dropItem || budget == 0 {
			continue
		}
		n.retiring[id] = true
		var reply struct{ Ready bool }
		if err := b.command(ctx, fmt.Sprintf("hm_item_forget %s %s %d", b.nativeSession, view.World, slot), &reply); err != nil {
			return err
		}
		budget--
		if reply.Ready {
			delete(n.retiring, id)
			delete(n.slots, id)
			delete(n.sent, id)
		}
	}
	for _, item := range view.Items {
		if n.slots[item.ID] != 0 {
			continue
		}
		for slot := uint64(1); slot <= MaxGroundItems; slot++ {
			if n.globalItem(slot) == 0 {
				n.slots[item.ID] = slot
				break
			}
		}
	}
	return nil
}
