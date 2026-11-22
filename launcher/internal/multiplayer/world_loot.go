package multiplayer

import (
	"fmt"
)

// A factory birth belongs to one canonical destroyed parent. Pointers and
// process-local actor handles are never part of this identity.
type LootParent struct {
	Hash    uint32 `json:"hash"`
	Actor   string `json:"actor"`
	Spawn   Vec3   `json:"spawn"`
	Ordinal uint32 `json:"ordinal"`
}

func (p LootParent) valid() bool {
	return p.Ordinal > 0 && p.Ordinal <= 4096 && (EnemyHealth{Hash: p.Hash, Actor: p.Actor, Spawn: p.Spawn}).Valid()
}
func (p LootParent) key(scene string) string {
	return (EnemyHealth{Scene: scene, Hash: p.Hash, Actor: p.Actor, Spawn: p.Spawn}).key()
}
func (p *serverPeer) discardLoot(local uint64) {
	if p.groundDiscarded == nil {
		p.groundDiscarded = map[uint64]bool{}
	}
	if len(p.groundDiscarded) < MaxGroundItems {
		p.groundDiscarded[local] = true
	}
}
func (s *Server) acceptLoot(p *serverPeer, g GroundSource) bool {
	parent := g.Parent
	if parent == nil {
		return true
	}
	if !s.cfg.DedicatedWorld {
		p.discardLoot(g.Local)
		return false
	}
	key := parent.key(g.Scene)
	if origin := s.lootOrigins[key]; origin != "" && origin != g.Origin {
		p.discardLoot(g.Local)
		return false
	}
	dead, known := s.enemies[key]
	// Factory completion may precede delivery of the native zero-life report.
	// Keep retrying the same birth; never turn it into a local-only award.
	return known && dead.Health == 0 && g.Position.Sub(dead.Position).Len() <= 48
}
func (s *Server) pinLoot(g GroundSource) error {
	if g.Parent == nil {
		return nil
	}
	if s.lootOrigins == nil {
		s.lootOrigins = map[string]string{}
	}
	key := g.Parent.key(g.Scene)
	if old := s.lootOrigins[key]; old != "" && old != g.Origin {
		return fmt.Errorf("origens conflitantes para os espólios da entidade")
	}
	s.lootOrigins[key] = g.Origin
	return nil
}
