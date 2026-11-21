package multiplayer

import (
	"fmt"
	"math"
	"strings"
)

const MaxEnemyObservations = 64
const MaxSharedEnemies = MaxPlayers * MaxEnemyObservations
const maxSessionDamageActors = 32768

// Map hash + actor + original placement identify the same loaded enemy across
// processes. Native pointers and per-process actor IDs never cross the network.
type EnemyHealth struct {
	Scene    string `json:"scene,omitempty"`
	Hash     uint32 `json:"hash"`
	Actor    string `json:"actor"`
	Spawn    Vec3   `json:"spawn"`
	Position Vec3   `json:"position"`
	Health   int    `json:"health"`
	Damage   uint64 `json:"damage,omitempty"`
}

func (e EnemyHealth) Valid() bool {
	if e.Scene != "" && !validScene(e.Scene) {
		return false
	}
	if e.Hash == 0 || e.Hash == 0xffffffff || !mapDamageActor(e.Actor) || len(e.Actor) > 63 || !e.Spawn.Valid() || !e.Position.Valid() || e.Health < 0 || e.Health > 1000000 || e.Damage > math.MaxInt64 {
		return false
	}
	for _, r := range e.Actor {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
			return false
		}
	}
	return true
}
func (e EnemyHealth) key() string {
	return e.Scene + ":" + fmt.Sprintf("%08x:%s:%.2f:%.2f:%.2f", e.Hash, e.Actor, e.Spawn.X, e.Spawn.Y, e.Spawn.Z)
}
func mergeEnemyHealth(registry map[string]EnemyHealth, records []EnemyHealth, local Pose) error {
	if len(records) > MaxEnemyObservations || !local.Ready {
		return fmt.Errorf("observação de mundo inválida")
	}
	seen := map[string]bool{}
	for _, e := range records {
		key := e.key()
		if !e.Valid() || seen[key] || e.Position.Sub(local.Position).Len() > 160 {
			return fmt.Errorf("monstro inválido, duplicado ou fora da região carregada")
		}
		seen[key] = true
	}
	for _, e := range records {
		key := e.key()
		old, exists := registry[key]
		if exists && old.Health < e.Health {
			e.Health = old.Health
		}
		if exists || len(registry) < maxSessionDamageActors {
			registry[key] = e
		}
	}
	return nil
}
func (c *Client) PublishEnemyHealth(records []EnemyHealth) error {
	if len(records) > MaxEnemyObservations {
		return fmt.Errorf("muitos monstros na observação")
	}
	for _, e := range records {
		if !e.Valid() {
			return fmt.Errorf("estado de monstro inválido")
		}
	}
	return c.send(Message{Type: "enemy_health", Enemies: records})
}
func (c *Client) SharedEnemyHealth() []EnemyHealth {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]EnemyHealth(nil), c.enemies...)
}

func mapDamageActor(name string) bool {
	if strings.HasPrefix(name, "Enemy_") || worldActorKind(name) == 3 {
		return true
	}
	return strings.HasPrefix(name, "Obj_Tree") && !strings.Contains(name, "_Stump") && !strings.Contains(name, "_Trunk") && !strings.Contains(name, "Root") && !strings.Contains(name, "Branch")
}

// Each native instance reports its cumulative locally observed loss. Server
// writes are excluded in the runtime. Replays cannot count the loss twice,
// while two players damaging the same entity contribute independently.
func mergeMapDamage(registry map[string]EnemyHealth, ledger map[string]uint64, origin string, records []EnemyHealth, local Pose) error {
	if !validOrigin(origin) || len(records) > MaxEnemyObservations || !local.Ready {
		return fmt.Errorf("evento de mundo inválido")
	}
	seen := map[string]bool{}
	for _, e := range records {
		key := origin + ":" + e.key()
		if !e.Valid() || seen[key] || e.Damage < ledger[key] {
			return fmt.Errorf("contador de dano inválido")
		}
		seen[key] = true
	}
	additional := 0
	for _, e := range records {
		if _, ok := ledger[origin+":"+e.key()]; !ok {
			additional++
		}
	}
	if len(ledger)+additional > maxSessionDamageActors*2 {
		return fmt.Errorf("limite de origens de mundo atingido")
	}
	for _, e := range records {
		key := e.key()
		event := origin + ":" + key
		before, known := ledger[event]
		// Final zero evidence may be awaiting acknowledgement after fast travel.
		// Accept a distant terminal report only for an identity this peer already observed.
		if (e.Scene != local.Scene || e.Position.Sub(local.Position).Len() > 160) && !(known && e.Health == 0) {
			continue
		}
		old, exists := registry[key]
		if !exists && len(registry) >= maxSessionDamageActors {
			continue
		}
		if exists {
			delta := e.Damage - before
			health := old.Health
			if delta >= uint64(health) {
				health = 0
			} else {
				health -= int(delta)
			}
			if e.Health == 0 || (!known && e.Damage == 0 && e.Health < health) {
				health = e.Health
			}
			e.Health = health
		}
		ledger[event] = e.Damage
		e.Damage = 0
		registry[key] = e
	}
	return nil
}
func (c *Client) PublishMapDamage(origin string, records []EnemyHealth) error {
	if !validOrigin(origin) || len(records) > MaxEnemyObservations {
		return fmt.Errorf("origem de mundo inválida")
	}
	for _, e := range records {
		if !e.Valid() {
			return fmt.Errorf("entidade de mundo inválida")
		}
	}
	return c.send(Message{Type: "map_damage", WorldOrigin: origin, Enemies: records})
}
