package gen1

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mp "hyrulelauncher/internal/multiplayer"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type savedProfile struct {
	state Personal
	at    time.Time
}
type player struct {
	token       string
	public      PublicPlayer
	private     Personal
	at          time.Time
	lastCommand uint64
	lastResult  Result
	attackAt    time.Time
}
type World struct {
	profiles          map[string]savedProfile
	mu                sync.Mutex
	cfg               Config
	players           map[string]*player
	enemies           map[string]*Enemy
	items             map[string]*WorldItem
	definitions       map[string]ItemDefinition
	enemyDefinitions  map[string]EnemyDefinition
	enemyAttack       map[string]time.Time
	registered        map[string]string
	counter, sequence uint64
}

func NewWorld(cfg Config) (*World, error) {
	if e := cfg.Catalog.Validate(); e != nil {
		return nil, e
	}
	if cfg.MaxHealth == 0 {
		cfg.MaxHealth = 120
	}
	if cfg.MaxHealth < 1 || cfg.MaxHealth > 1000 {
		return nil, fmt.Errorf("vida máxima inválida")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.PoseTTL <= 0 {
		cfg.PoseTTL = 2 * time.Second
	}
	w := &World{cfg: cfg, players: map[string]*player{}, profiles: map[string]savedProfile{}, enemies: map[string]*Enemy{}, items: map[string]*WorldItem{}, definitions: map[string]ItemDefinition{}, enemyDefinitions: map[string]EnemyDefinition{}, enemyAttack: map[string]time.Time{}, registered: map[string]string{}}
	for _, d := range cfg.Catalog.Items {
		w.definitions[d.Actor] = d
	}
	for _, d := range cfg.Catalog.Enemies {
		w.enemyDefinitions[d.Actor] = d
	}
	return w, nil
}
func (w *World) Join(id, name string, caps Capabilities) (PublicPlayer, error) {
	return w.Resume(id, name, caps, "")
}
func (w *World) Resume(id, name string, caps Capabilities, token string) (PublicPlayer, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if e := caps.Require(w.cfg.AllowSimulation); e != nil {
		return PublicPlayer{}, e
	}
	name, e := mp.NormalizeName(name)
	if e != nil {
		return PublicPlayer{}, e
	}
	if !identifier(id) || w.players[id] != nil || len(w.players) >= MaxPlayers {
		return PublicPlayer{}, fmt.Errorf("identidade inválida ou servidor completo")
	}
	slot := 0
	for _, p := range w.players {
		if p.public.Slot == 0 {
			slot = 1
		}
	}
	p := &player{public: PublicPlayer{ID: id, Name: name, Slot: slot, Alive: false}, private: Personal{ID: id, Inventory: []Item{}}, at: w.cfg.Now()}
	if token != "" {
		saved, ok := w.profiles[token]
		if !ok {
			return PublicPlayer{}, fmt.Errorf("sessão para reconectar não existe mais; inicie uma nova sessão explicitamente")
		}
		for _, active := range w.players {
			if active.token == token {
				return PublicPlayer{}, fmt.Errorf("perfil já conectado")
			}
		}
		if w.cfg.Now().Sub(saved.at) > 10*time.Minute {
			return PublicPlayer{}, fmt.Errorf("sessão de reconexão expirada")
		}
		p.private = saved.state
		p.private.ID = id
		p.private.Inventory = append([]Item{}, saved.state.Inventory...)
		p.public.Alive = p.private.Health > 0
		for _, item := range p.private.Inventory {
			if item.ID == p.private.EquippedID {
				p.public.EquippedActor = item.Actor
			}
		}
	} else {
		var raw [24]byte
		if _, e := rand.Read(raw[:]); e != nil {
			return PublicPlayer{}, e
		}
		token = hex.EncodeToString(raw[:])
	}
	p.token = token
	w.players[id] = p
	return p.public, nil
}
func (w *World) Leave(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if p := w.players[id]; p != nil {
		state := p.private
		state.Inventory = append([]Item{}, state.Inventory...)
		w.profiles[p.token] = savedProfile{state, w.cfg.Now()}
		if len(w.profiles) > 32 {
			oldest := ""
			var at time.Time
			for key, saved := range w.profiles {
				if oldest == "" || saved.at.Before(at) {
					oldest = key
					at = saved.at
				}
			}
			delete(w.profiles, oldest)
		}
	}
	delete(w.players, id)
	w.retarget(w.cfg.Now())
}
func (w *World) host() string {
	host := ""
	slot := MaxPlayers
	for id, p := range w.players {
		if p.public.Slot < slot {
			host = id
			slot = p.public.Slot
		}
	}
	return host
}
func (w *World) fresh(p *player, now time.Time) bool {
	return p != nil && p.private.Initialized && p.public.Frame.Ready && p.private.Health > 0 && now.Sub(p.at) <= w.cfg.PoseTTL
}
func (w *World) retarget(now time.Time) {
	ids := make([]string, 0, len(w.players))
	for id := range w.players {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, e := range w.enemies {
		target := ""
		best := math.Inf(1)
		if e.Health > 0 {
			for _, id := range ids {
				p := w.players[id]
				if !w.fresh(p, now) || p.public.Frame.World != e.World {
					continue
				}
				distance := p.public.Frame.Position.Sub(e.Position).Len()
				if distance > w.enemyDefinitions[e.Actor].AggroRange {
					continue
				}
				if distance < best || distance == best && id == e.Target {
					best = distance
					target = id
				}
			}
		}
		// Owner follows the nearest target. A new epoch invalidates every update
		// from the old simulator, including damage packets already in flight.
		if e.Target != target || e.Owner != target {
			e.Target = target
			e.Owner = target
			e.Epoch++
			e.LastPose = 0
		}
	}
}
func (w *World) Apply(id string, c Command) Result {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[id]
	if p == nil {
		return Result{c.Sequence, false, "jogador não conectado"}
	}
	if c.Sequence == p.lastCommand && c.Sequence != 0 {
		return p.lastResult
	}
	if c.Sequence == 0 || c.Sequence < p.lastCommand {
		return Result{c.Sequence, false, "comando antigo ou sem sequência"}
	}
	now := w.cfg.Now()
	w.retarget(now)
	e := w.apply(p, c, now)
	r := Result{Request: c.Sequence, Accepted: e == nil}
	if e != nil {
		r.Error = e.Error()
	}
	p.lastCommand = c.Sequence
	p.lastResult = r
	w.retarget(now)
	return r
}
func (w *World) apply(p *player, c Command, now time.Time) error {
	id := p.public.ID
	if c.Kind == "seed" {
		return w.seed(p, c.Personal)
	}
	if c.Kind == "frame" {
		if c.Frame == nil || !c.Frame.Valid() {
			return fmt.Errorf("frame inválido")
		}
		p.public.Frame = *c.Frame
		p.at = now
		return nil
	}
	if !w.fresh(p, now) {
		return fmt.Errorf("jogador sem mundo carregado, morto ou desatualizado")
	}
	switch c.Kind {
	case "register_item", "register_enemy":
		if id != w.host() {
			return fmt.Errorf("somente o host registra atores do mapa")
		}
		if !identifier(c.Entity) || !strings.HasPrefix(c.Entity, "map:") || c.World != p.public.Frame.World || !c.Position.Valid() || p.public.Frame.Position.Sub(c.Position).Len() > 300 {
			return fmt.Errorf("ator de mapa inválido")
		}
		kindKey := c.Kind + "|" + c.World + "|" + c.Definition
		if previous, exists := w.registered[c.Entity]; exists {
			if previous != kindKey {
				return fmt.Errorf("ID do ator já pertence a outro objeto")
			}
			return nil
		}
		if len(w.registered) >= 512 {
			return fmt.Errorf("limite de IDs de mapa")
		}
		if c.Kind == "register_item" {
			d, ok := w.definitions[c.Definition]
			if !ok || c.Amount < 1 || c.Amount > d.MaxStack {
				return fmt.Errorf("item fora do catálogo")
			}
			if w.items[c.Entity] != nil {
				return nil
			}
			if len(w.items) >= MaxEntities {
				return fmt.Errorf("limite de itens")
			}
			w.items[c.Entity] = &WorldItem{c.Entity, d.Actor, c.World, c.Position, c.Amount, d.Durability}
			w.registered[c.Entity] = kindKey
			return nil
		}
		d, ok := w.enemyDefinitions[c.Definition]
		if !ok {
			return fmt.Errorf("monstro fora do catálogo")
		}
		if w.enemies[c.Entity] != nil {
			return nil
		}
		if len(w.enemies) >= MaxEntities {
			return fmt.Errorf("limite de monstros")
		}
		w.enemies[c.Entity] = &Enemy{ID: c.Entity, Actor: d.Actor, World: c.World, Position: c.Position, Health: d.Health, Animation: "idle"}
		w.registered[c.Entity] = kindKey
		return nil
	case "pickup":
		item := w.items[c.Entity]
		if item == nil {
			return fmt.Errorf("item já recolhido ou inexistente")
		}
		if item.World != p.public.Frame.World || item.Position.Sub(p.public.Frame.Position).Len() > 3 {
			return fmt.Errorf("item fora do alcance")
		}
		if len(p.private.Inventory) >= MaxInventory {
			return fmt.Errorf("inventário cheio")
		}
		w.counter++
		itemID := fmt.Sprintf("item-%d", w.counter)
		// Local saves can already use an item-N key. Never create two entries
		// with the same identity in this player's inventory after bootstrap.
		for {
			found := false
			for _, existing := range p.private.Inventory {
				if existing.ID == itemID {
					found = true
					break
				}
			}
			if !found {
				break
			}
			w.counter++
			itemID = fmt.Sprintf("item-%d", w.counter)
		}
		p.private.Inventory = append(p.private.Inventory, Item{itemID, item.Actor, item.Count, item.Durability})
		delete(w.items, c.Entity)
		return nil
	case "equip":
		for _, i := range p.private.Inventory {
			if i.ID == c.Entity && w.definitions[i.Actor].Kind == "weapon" && i.Durability > 0 {
				p.private.EquippedID = i.ID
				p.public.EquippedActor = i.Actor
				return nil
			}
		}
		return fmt.Errorf("arma não pertence ao jogador")
	case "drop":
		for i, item := range p.private.Inventory {
			if item.ID != c.Entity {
				continue
			}
			if len(w.items) >= MaxEntities {
				return fmt.Errorf("limite de itens")
			}
			w.counter++
			key := fmt.Sprintf("drop-%d", w.counter)
			w.items[key] = &WorldItem{key, item.Actor, p.public.Frame.World, p.public.Frame.Position, item.Count, item.Durability}
			p.private.Inventory = append(p.private.Inventory[:i], p.private.Inventory[i+1:]...)
			if p.private.EquippedID == item.ID {
				p.private.EquippedID = ""
				p.public.EquippedActor = ""
			}
			return nil
		}
		return fmt.Errorf("item não pertence ao jogador")
	case "environment_damage":
		if c.Target != "" && c.Target != id {
			return fmt.Errorf("dano ambiental só pode afetar o próprio jogador")
		}
		if c.Amount < 1 || c.Amount > 10000 {
			return fmt.Errorf("dano inválido")
		}
		w.hurt(p, c.Amount)
		return nil
	case "attack":
		return w.attack(p, c, now)
	case "enemy_pose":
		enemy := w.enemies[c.Entity]
		if enemy == nil || enemy.Owner != id || enemy.Epoch != c.Epoch || enemy.Health <= 0 {
			return fmt.Errorf("autoridade do monstro expirada")
		}
		if c.PoseSequence <= enemy.LastPose || !c.Position.Valid() || !c.Velocity.Valid() || c.Velocity.Len() > 100 || !finite(c.Yaw) || math.Abs(c.Yaw) > 10000 || !identifier(c.Animation) {
			return fmt.Errorf("pose do monstro inválida")
		}
		if c.Position.Sub(enemy.Position).Len() > 10 {
			return fmt.Errorf("salto do monstro rejeitado")
		}
		enemy.Position = c.Position
		enemy.Velocity = c.Velocity
		enemy.Yaw = c.Yaw
		enemy.Animation = c.Animation
		enemy.LastPose = c.PoseSequence
		return nil
	case "enemy_attack":
		enemy := w.enemies[c.Entity]
		if enemy == nil || enemy.Owner != id || enemy.Epoch != c.Epoch || enemy.Target != c.Target || enemy.Health <= 0 {
			return fmt.Errorf("alvo ou autoridade do monstro expirados")
		}
		victim := w.players[c.Target]
		d := w.enemyDefinitions[enemy.Actor]
		if !w.fresh(victim, now) || victim.public.Frame.World != enemy.World || victim.public.Frame.Position.Sub(enemy.Position).Len() > d.AttackRange {
			return fmt.Errorf("alvo fora do alcance")
		}
		if at := w.enemyAttack[enemy.ID]; !at.IsZero() && now.Sub(at) < time.Duration(d.CooldownMS)*time.Millisecond {
			return fmt.Errorf("ataque do monstro em recarga")
		}
		w.enemyAttack[enemy.ID] = now
		w.hurt(victim, d.Damage)
		return nil
	default:
		return fmt.Errorf("comando não suportado")
	}
}
func (w *World) hurt(p *player, damage int) {
	p.private.Health -= damage
	if p.private.Health < 0 {
		p.private.Health = 0
	}
	p.public.Alive = p.private.Health > 0
}
func (w *World) attack(p *player, c Command, now time.Time) error {
	index := -1
	var weapon Item
	for i, item := range p.private.Inventory {
		if item.ID == p.private.EquippedID {
			index = i
			weapon = item
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("nenhuma arma equipada")
	}
	d := w.definitions[weapon.Actor]
	if d.Kind != "weapon" || weapon.Durability <= 0 {
		return fmt.Errorf("arma inválida")
	}
	if !p.attackAt.IsZero() && now.Sub(p.attackAt) < time.Duration(d.CooldownMS)*time.Millisecond {
		return fmt.Errorf("ataque em recarga")
	}
	target := w.players[c.Target]
	enemy := w.enemies[c.Target]
	if target != nil {
		if !w.cfg.AllowPVP || target == p || !w.fresh(target, now) || target.public.Frame.World != p.public.Frame.World || target.public.Frame.Position.Sub(p.public.Frame.Position).Len() > d.Reach {
			return fmt.Errorf("jogador fora do alcance ou PvP desativado")
		}
		w.hurt(target, d.Damage)
	} else if enemy != nil {
		if enemy.Health <= 0 || enemy.World != p.public.Frame.World || enemy.Position.Sub(p.public.Frame.Position).Len() > d.Reach {
			return fmt.Errorf("monstro fora do alcance")
		}
		enemy.Health -= d.Damage
		if enemy.Health < 0 {
			enemy.Health = 0
		}
	} else {
		return fmt.Errorf("alvo inexistente")
	}
	p.attackAt = now
	p.private.Inventory[index].Durability--
	if p.private.Inventory[index].Durability == 0 {
		p.private.Inventory = append(p.private.Inventory[:index], p.private.Inventory[index+1:]...)
		p.private.EquippedID = ""
		p.public.EquippedActor = ""
	}
	return nil
}
func (w *World) Snapshot(id string) (Snapshot, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[id]
	if p == nil {
		return Snapshot{}, fmt.Errorf("jogador não conectado")
	}
	w.retarget(w.cfg.Now())
	w.sequence++
	s := Snapshot{Sequence: w.sequence, Personal: p.private, Players: []PublicPlayer{}, Enemies: []Enemy{}, Items: []WorldItem{}}
	s.Personal.Inventory = append([]Item{}, p.private.Inventory...)
	for _, v := range w.players {
		s.Players = append(s.Players, v.public)
	}
	for _, e := range w.enemies {
		if e.World == p.public.Frame.World {
			s.Enemies = append(s.Enemies, *e)
		}
	}
	for _, i := range w.items {
		if i.World == p.public.Frame.World {
			s.Items = append(s.Items, *i)
		}
	}
	sortSnapshot(&s)
	return s, nil
}

func (w *World) seed(p *player, state *Personal) error {
	if p.private.Initialized {
		return fmt.Errorf("estado pessoal já inicializado")
	}
	if state == nil || state.ID != "" && state.ID != p.public.ID || state.MaxHealth < 1 || state.MaxHealth > w.cfg.MaxHealth || state.Health < 0 || state.Health > state.MaxHealth || len(state.Inventory) > MaxInventory {
		return fmt.Errorf("estado pessoal inicial inválido; inventário local preservado")
	}
	ids := map[string]bool{}
	for _, item := range state.Inventory {
		d, ok := w.definitions[item.Actor]
		if !ok || !identifier(item.ID) || ids[item.ID] || item.Count < 1 || item.Count > d.MaxStack || item.Durability < 0 || item.Durability > d.Durability {
			return fmt.Errorf("item inicial inválido ou ausente no catálogo: %s", item.Actor)
		}
		ids[item.ID] = true
	}
	if state.EquippedID != "" && !ids[state.EquippedID] {
		return fmt.Errorf("arma inicial não pertence ao jogador")
	}
	p.private = *state
	p.private.ID = p.public.ID
	p.private.Initialized = true
	p.private.Inventory = append([]Item{}, state.Inventory...)
	p.public.Alive = state.Health > 0
	if state.EquippedID != "" {
		for _, item := range state.Inventory {
			if item.ID == state.EquippedID {
				if w.definitions[item.Actor].Kind != "weapon" {
					p.private = Personal{ID: p.public.ID}
					p.public.Alive = false
					return fmt.Errorf("item inicial equipado não é arma")
				}
				p.public.EquippedActor = item.Actor
			}
		}
	}
	return nil
}

func (w *World) SessionToken(id string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if p := w.players[id]; p != nil {
		return p.token
	}
	return ""
}
