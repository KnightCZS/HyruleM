// Package gen1 implements the server authority for HyruleM's native-world
// development branch. It does not create BOTW actors; the native adapter is a
// separate, required capability. It never falls back to a desktop avatar.
package gen1

import (
	"fmt"
	mp "hyrulelauncher/internal/multiplayer"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
)

const Protocol = 2
const MaxPlayers = 2
const MaxPacket = 262144
const MaxEntities = 64
const MaxInventory = 128
const RequiredABI = 1

type Vec3 = mp.Vec3
type Capabilities struct {
	ABI               int    `json:"abi"`
	Runtime           string `json:"runtime"`
	GameVersion       string `json:"game_version"`
	NativeActors      bool   `json:"native_actors"`
	Animation         bool   `json:"animation"`
	Collision         bool   `json:"collision"`
	PersonalVitals    bool   `json:"personal_vitals"`
	PersonalInventory bool   `json:"personal_inventory"`
	PickupHook        bool   `json:"pickup_hook"`
	DamageHook        bool   `json:"damage_hook"`
	EnemyAuthority    bool   `json:"enemy_authority"`
	EnemyTarget       bool   `json:"enemy_target"`
	WorldIdentity     bool   `json:"world_identity"`
	Simulated         bool   `json:"simulated"`
}

func (c Capabilities) Missing() []string {
	var out []string
	if c.ABI != RequiredABI {
		out = append(out, "abi=1")
	}
	for _, v := range []struct {
		name string
		ok   bool
	}{{"native_actors", c.NativeActors}, {"animation", c.Animation}, {"collision", c.Collision}, {"personal_vitals", c.PersonalVitals}, {"personal_inventory", c.PersonalInventory}, {"pickup_hook", c.PickupHook}, {"damage_hook", c.DamageHook}, {"enemy_authority", c.EnemyAuthority}, {"enemy_target", c.EnemyTarget}, {"world_identity", c.WorldIdentity}} {
		if !v.ok {
			out = append(out, v.name)
		}
	}
	return out
}
func (c Capabilities) Require(simulation bool) error {
	if m := c.Missing(); len(m) > 0 {
		return fmt.Errorf("runtime sem suporte nativo: faltam %s", strings.Join(m, ", "))
	}
	if c.Simulated && !simulation {
		return fmt.Errorf("adaptador simulado recusado: este servidor exige integração nativa")
	}
	if !c.Simulated && (c.Runtime != "SavageWind-HyruleM" || c.GameVersion != "1.9.0") {
		return fmt.Errorf("runtime/versão do adaptador nativo incompatível")
	}
	return nil
}
func identifier(s string) bool {
	if len(s) < 1 || len(s) > 96 {
		return false
	}
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_:-./", r)) {
			return false
		}
	}
	return !strings.Contains(s, "..")
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

type Frame struct {
	World     string  `json:"world"`
	Position  Vec3    `json:"position"`
	Velocity  Vec3    `json:"velocity"`
	Yaw       float64 `json:"yaw"`
	Animation string  `json:"animation"`
	Ready     bool    `json:"ready"`
}

func (f Frame) Valid() bool {
	return identifier(f.World) && f.Position.Valid() && f.Velocity.Valid() && f.Velocity.Len() <= 150 && finite(f.Yaw) && math.Abs(f.Yaw) < 10000 && identifier(f.Animation)
}

type ItemDefinition struct {
	Actor      string  `json:"actor"`
	Kind       string  `json:"kind"`
	MaxStack   int     `json:"max_stack"`
	Durability int     `json:"durability"`
	Damage     int     `json:"damage"`
	Reach      float64 `json:"reach"`
	CooldownMS int     `json:"cooldown_ms"`
}
type EnemyDefinition struct {
	Actor       string  `json:"actor"`
	Health      int     `json:"health"`
	Damage      int     `json:"damage"`
	AggroRange  float64 `json:"aggro_range"`
	AttackRange float64 `json:"attack_range"`
	CooldownMS  int     `json:"cooldown_ms"`
}
type Catalog struct {
	Items   []ItemDefinition  `json:"items"`
	Enemies []EnemyDefinition `json:"enemies"`
}

func (c Catalog) Validate() error {
	if len(c.Items) > 512 || len(c.Enemies) > 128 {
		return fmt.Errorf("catálogo excede o limite")
	}
	seen := map[string]bool{}
	for _, i := range c.Items {
		if !identifier(i.Actor) || seen[i.Actor] || i.MaxStack < 1 || i.MaxStack > 999 || i.Damage < 0 || i.Damage > 10000 || i.Durability < 0 || i.Durability > 100000 || !finite(i.Reach) || i.Reach < 0 || i.Reach > 20 || i.CooldownMS < 100 || i.CooldownMS > 30000 {
			return fmt.Errorf("definição de item inválida: %s", i.Actor)
		}
		if i.Kind != "weapon" && i.Kind != "material" {
			return fmt.Errorf("tipo de item inválido")
		}
		if i.Kind == "weapon" && (i.MaxStack != 1 || i.Durability < 1 || i.Damage < 1 || i.Reach <= 0) {
			return fmt.Errorf("arma inválida")
		}
		seen[i.Actor] = true
	}
	seen = map[string]bool{}
	for _, e := range c.Enemies {
		if !identifier(e.Actor) || seen[e.Actor] || e.Health < 1 || e.Health > 100000 || e.Damage < 1 || e.Damage > 10000 || !finite(e.AggroRange) || e.AggroRange <= 0 || e.AggroRange > 500 || !finite(e.AttackRange) || e.AttackRange <= 0 || e.AttackRange > e.AggroRange || e.CooldownMS < 100 || e.CooldownMS > 30000 {
			return fmt.Errorf("definição de monstro inválida: %s", e.Actor)
		}
		seen[e.Actor] = true
	}
	return nil
}

type Item struct {
	ID         string `json:"id"`
	Actor      string `json:"actor"`
	Count      int    `json:"count"`
	Durability int    `json:"durability"`
}
type PublicPlayer struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Slot          int    `json:"slot"`
	Frame         Frame  `json:"frame"`
	EquippedActor string `json:"equipped_actor,omitempty"`
	Alive         bool   `json:"alive"`
}
type Personal struct {
	Initialized bool   `json:"initialized"`
	ID          string `json:"id"`
	Health      int    `json:"health"`
	MaxHealth   int    `json:"max_health"`
	Inventory   []Item `json:"inventory"`
	EquippedID  string `json:"equipped_id,omitempty"`
}
type Enemy struct {
	ID        string  `json:"id"`
	Actor     string  `json:"actor"`
	World     string  `json:"world"`
	Position  Vec3    `json:"position"`
	Velocity  Vec3    `json:"velocity"`
	Yaw       float64 `json:"yaw"`
	Animation string  `json:"animation"`
	Health    int     `json:"health"`
	Target    string  `json:"target"`
	Owner     string  `json:"owner"`
	Epoch     uint64  `json:"epoch"`
	LastPose  uint64  `json:"last_pose"`
}
type WorldItem struct {
	ID         string `json:"id"`
	Actor      string `json:"actor"`
	World      string `json:"world"`
	Position   Vec3   `json:"position"`
	Count      int    `json:"count"`
	Durability int    `json:"durability"`
}
type Snapshot struct {
	Sequence uint64         `json:"sequence"`
	Players  []PublicPlayer `json:"players"`
	Personal Personal       `json:"personal"`
	Enemies  []Enemy        `json:"enemies"`
	Items    []WorldItem    `json:"items"`
}
type Result struct {
	Request  uint64 `json:"request"`
	Accepted bool   `json:"accepted"`
	Error    string `json:"error,omitempty"`
}

// Only one transport method exists for mutations. Sender identity is bound to
// the accepted TCP session; clients never supply another inventory owner.
type Command struct {
	Sequence     uint64    `json:"sequence"`
	Kind         string    `json:"kind"`
	Frame        *Frame    `json:"frame,omitempty"`
	Personal     *Personal `json:"personal,omitempty"`
	Entity       string    `json:"entity,omitempty"`
	Definition   string    `json:"definition,omitempty"`
	World        string    `json:"world,omitempty"`
	Position     Vec3      `json:"position,omitempty"`
	Velocity     Vec3      `json:"velocity,omitempty"`
	Yaw          float64   `json:"yaw,omitempty"`
	Animation    string    `json:"animation,omitempty"`
	Target       string    `json:"target,omitempty"`
	Epoch        uint64    `json:"epoch,omitempty"`
	PoseSequence uint64    `json:"pose_sequence,omitempty"`
	Amount       int       `json:"amount,omitempty"`
}
type Config struct {
	Catalog         Catalog
	MaxHealth       int
	AllowPVP        bool
	AllowSimulation bool
	PoseTTL         time.Duration
	Now             func() time.Time
}

func sortSnapshot(s *Snapshot) {
	sort.Slice(s.Players, func(i, j int) bool { return s.Players[i].Slot < s.Players[j].Slot })
	sort.Slice(s.Enemies, func(i, j int) bool { return s.Enemies[i].ID < s.Enemies[j].ID })
	sort.Slice(s.Items, func(i, j int) bool { return s.Items[i].ID < s.Items[j].ID })
	sort.Slice(s.Personal.Inventory, func(i, j int) bool { return s.Personal.Inventory[i].ID < s.Personal.Inventory[j].ID })
}
