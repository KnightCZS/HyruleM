// Package nativebridge defines the checked boundary to a future game-thread
// actor backend. No BOTW actor implementation is supplied by this package.
package nativebridge

import (
	"context"
	"fmt"
	"hyrulelauncher/internal/gen1"
	"sync"
)

type EnemyPlan struct {
	State      gen1.Enemy `json:"state"`
	SimulateAI bool       `json:"simulate_ai"`
}
type Plan struct {
	Sequence   uint64              `json:"sequence"`
	SelfID     string              `json:"self_id"`
	World      string              `json:"world"`
	WorldReady bool                `json:"world_ready"`
	Personal   *gen1.Personal      `json:"personal,omitempty"`
	Remotes    []gen1.PublicPlayer `json:"remotes"`
	Enemies    []EnemyPlan         `json:"enemies"`
	Items      []gen1.WorldItem    `json:"items"`
}

// BuildPlan never creates a local actor or copies a remote player's vitals or
// inventory. Empty actor sets mean despawn only HyruleM-managed actors.
func BuildPlan(self string, state gen1.Snapshot) (Plan, error) {
	p := Plan{Sequence: state.Sequence, SelfID: self, Remotes: []gen1.PublicPlayer{}, Enemies: []EnemyPlan{}, Items: []gen1.WorldItem{}}
	if self == "" || state.Sequence == 0 || state.Personal.ID != self || len(state.Players) > gen1.MaxPlayers || len(state.Enemies) > gen1.MaxEntities || len(state.Items) > gen1.MaxEntities {
		return Plan{}, fmt.Errorf("snapshot não pertence à sessão local")
	}
	players := map[string]gen1.PublicPlayer{}
	for _, player := range state.Players {
		if player.ID == "" {
			return Plan{}, fmt.Errorf("jogador sem identidade")
		}
		if _, exists := players[player.ID]; exists {
			return Plan{}, fmt.Errorf("jogador duplicado")
		}
		if player.Frame.Ready && !player.Frame.Valid() {
			return Plan{}, fmt.Errorf("pose inválida")
		}
		players[player.ID] = player
	}
	local, ok := players[self]
	if !ok {
		return Plan{}, fmt.Errorf("jogador local ausente")
	}
	p.World = local.Frame.World
	p.WorldReady = local.Frame.Ready && state.Personal.Initialized
	if state.Personal.Initialized {
		x := state.Personal
		if x.MaxHealth < 1 || x.Health < 0 || x.Health > x.MaxHealth || len(x.Inventory) > gen1.MaxInventory {
			return Plan{}, fmt.Errorf("estado pessoal inválido")
		}
		x.Inventory = append([]gen1.Item{}, x.Inventory...)
		p.Personal = &x
	}
	if !p.WorldReady {
		return p, nil
	}
	for _, player := range state.Players {
		if player.ID != self && player.Frame.Ready && player.Frame.World == p.World {
			p.Remotes = append(p.Remotes, player)
		}
	}
	seen := map[string]bool{}
	for _, e := range state.Enemies {
		if e.ID == "" || seen[e.ID] || e.World != p.World || !e.Position.Valid() || !e.Velocity.Valid() || e.Health < 0 || e.Owner != e.Target {
			return Plan{}, fmt.Errorf("estado de monstro inválido")
		}
		seen[e.ID] = true
		if e.Target != "" {
			target, exists := players[e.Target]
			if !exists || !target.Alive || !target.Frame.Ready || target.Frame.World != p.World || e.Health == 0 {
				return Plan{}, fmt.Errorf("alvo do monstro indisponível")
			}
		}
		p.Enemies = append(p.Enemies, EnemyPlan{State: e, SimulateAI: e.Owner == self && e.Health > 0})
	}
	for _, item := range state.Items {
		if item.ID == "" || seen[item.ID] || item.World != p.World || !item.Position.Valid() || item.Count < 1 || item.Durability < 0 {
			return Plan{}, fmt.Errorf("item de mundo inválido")
		}
		seen[item.ID] = true
		p.Items = append(p.Items, item)
	}
	return p, nil
}

// Backend must enqueue a complete plan and commit it on the game thread. Apply
// must either commit the whole plan or report failure without partial mutation.
// The official Savage Wind runtime does not implement this interface yet.
type Backend interface {
	Capabilities(context.Context) (gen1.Capabilities, error)
	ApplyOnGameThread(context.Context, Plan) error
}

type Gate struct {
	mu              sync.Mutex
	Self            string
	Backend         Backend
	AllowSimulation bool
	last            uint64
}

func (g *Gate) Apply(ctx context.Context, state gen1.Snapshot) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Backend == nil {
		return fmt.Errorf("backend nativo não instalado")
	}
	if state.Sequence <= g.last {
		return fmt.Errorf("snapshot antigo")
	}
	caps, err := g.Backend.Capabilities(ctx)
	if err != nil {
		return err
	}
	if err = caps.Require(g.AllowSimulation); err != nil {
		return err
	}
	p, err := BuildPlan(g.Self, state)
	if err != nil {
		return err
	}
	if err = g.Backend.ApplyOnGameThread(ctx, p); err != nil {
		return err
	}
	g.last = state.Sequence
	return nil
}
