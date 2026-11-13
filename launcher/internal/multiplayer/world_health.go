package multiplayer

import (
	"fmt"
	"strings"
)

const MaxEnemyObservations = 64
const MaxSharedEnemies = 128

// Map hash + actor + original placement identify the same loaded enemy across
// processes. Native pointers and per-process actor IDs never cross the network.
type EnemyHealth struct {
	Hash     uint32 `json:"hash"`
	Actor    string `json:"actor"`
	Spawn    Vec3   `json:"spawn"`
	Position Vec3   `json:"position"`
	Health   int    `json:"health"`
}

func (e EnemyHealth) Valid() bool {
	if e.Hash == 0 || e.Hash == 0xffffffff || !strings.HasPrefix(e.Actor, "Enemy_") || len(e.Actor) > 63 || !e.Spawn.Valid() || !e.Position.Valid() || e.Health < 0 || e.Health > 1000000 {
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
	return fmt.Sprintf("%08x:%s:%.2f:%.2f:%.2f", e.Hash, e.Actor, e.Spawn.X, e.Spawn.Y, e.Spawn.Z)
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
		if exists || len(registry) < MaxSharedEnemies {
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
