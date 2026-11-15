package multiplayer

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"
)

const MaxGroundItems = 32
const GroundRadius = 90.0

func GroundWeapon(s string) bool {
	if s == "Weapon_Sword_070" || s == "Weapon_Sword_502" {
		return false
	}
	return equipmentName(s, 0) || equipmentName(s, 1) || equipmentName(s, 2)
}

func GroundActor(s string) bool {
	if GroundWeapon(s) {
		return true
	}
	switch s {
	case "Item_Fruit_A", "Item_Fruit_B", "Item_Fruit_F", "Item_Mushroom_A", "Item_PlantGet_A":
		return true
	}
	return false
}

type GroundSource struct {
	Local         uint64 `json:"local"`
	NativeDrop    bool   `json:"native_drop,omitempty"`
	Value         int    `json:"value,omitempty"`
	Modifier      uint32 `json:"modifier,omitempty"`
	ModifierValue int    `json:"modifier_value,omitempty"`
	Drop          bool   `json:"drop,omitempty"`
	Origin        string `json:"origin,omitempty"`
	Hash          uint32 `json:"hash"`
	Actor         string `json:"actor"`
	Spawn         Vec3   `json:"spawn"`
	Position      Vec3   `json:"position"`
}

func validOrigin(s string) bool {
	if len(s) != 16 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func (g GroundSource) Valid() bool {
	if GroundWeapon(g.Actor) {
		if g.Value <= 0 || g.Value > 1000000 || g.ModifierValue < -100000 || g.ModifierValue > 100000 {
			return false
		}
	} else if g.Value != 0 || g.Modifier != 0 || g.ModifierValue != 0 || g.NativeDrop {
		return false
	}
	if g.NativeDrop && (!g.Drop || g.Local < 0x200000000) {
		return false
	}
	return (g.Hash != 0 && !g.Drop || g.Hash == 0 && g.Drop && validOrigin(g.Origin)) && g.Local > 0 && g.Local <= math.MaxInt64 && GroundActor(g.Actor) && g.Spawn.Valid() && g.Position.Valid() && g.Hash != math.MaxUint32
}
func (g GroundSource) key(owner string) string {
	if g.Hash == 0 {
		return fmt.Sprintf("d:%s:%d", g.Origin, g.Local)
	}
	return fmt.Sprintf("m:%08x:%s:%.3f:%.3f:%.3f", g.Hash, g.Actor, g.Spawn.X, g.Spawn.Y, g.Spawn.Z)
}

type GroundItem struct {
	ID       uint64       `json:"id"`
	Source   GroundSource `json:"source"`
	Owner    string       `json:"owner"`
	State    string       `json:"state"`
	Claimant string       `json:"claimant,omitempty"`
	Request  uint64       `json:"request,omitempty"`
}

func (i GroundItem) Valid() bool {
	if i.ID == 0 || i.ID > MaxGroundItems || !i.Source.Valid() || i.Owner == "" {
		return false
	}
	switch i.State {
	case "available", "spawning", "cancelled":
		return i.Claimant == "" && i.Request == 0
	case "reserved", "taken", "uncertain":
		return i.Claimant != "" && i.Request > 0 && i.Request <= math.MaxInt64
	}
	return false
}

type GroundClaim struct {
	Request uint64       `json:"request"`
	Item    uint64       `json:"item,omitempty"`
	Source  GroundSource `json:"source"`
	Status  string       `json:"status,omitempty"`
}

// Terminal snapshots only need the acknowledgement, not another full source.
// Keep all acknowledgements so a burst cannot strand older native requests.
func (c GroundClaim) MarshalJSON() ([]byte, error) {
	type wire struct {
		Request uint64        `json:"request"`
		Item    uint64        `json:"item,omitempty"`
		Source  *GroundSource `json:"source,omitempty"`
		Status  string        `json:"status,omitempty"`
	}
	v := wire{Request: c.Request, Item: c.Item, Status: c.Status}
	if c.Status == "" || c.Status == "waiting" || c.Status == "granted" {
		v.Source = &c.Source
	}
	return json.Marshal(v)
}

type GroundReceipt struct {
	Request uint64 `json:"request"`
	Item    uint64 `json:"item"`
	Applied bool   `json:"applied"`
}
type GroundView struct {
	Enabled    bool
	World      string
	Zone       Vec3
	Items      []GroundItem
	Claims     []GroundClaim
	At         time.Time
	Rejected   []uint64
	DropResult *GroundDropResult
}

func (c *Client) Ground() GroundView {
	c.mu.Lock()
	defer c.mu.Unlock()
	v := c.ground
	v.Items = append([]GroundItem(nil), v.Items...)
	v.Claims = append([]GroundClaim(nil), v.Claims...)
	v.Rejected = append([]uint64(nil), v.Rejected...)
	if v.DropResult != nil {
		copy := *v.DropResult
		v.DropResult = &copy
	}
	return v
}
func (c *Client) PublishGround(v []GroundSource) error {
	if len(v) > MaxGroundItems {
		return fmt.Errorf("materiais excedem o limite")
	}
	for _, g := range v {
		if !g.Valid() {
			return fmt.Errorf("material nativo inválido")
		}
	}
	return c.send(Message{Type: "ground_observe", GroundSources: v})
}
func (c *Client) ClaimGround(v GroundClaim) error {
	return c.send(Message{Type: "ground_claim", GroundClaim: &v})
}
func (c *Client) ReceiptGround(v GroundReceipt) error {
	return c.send(Message{Type: "ground_receipt", GroundReceipt: &v})
}
func (s *Server) groundFresh(p *serverPeer, now time.Time) bool {
	return s.cfg.SharedItems && p.player.Pose.Ready && now.Sub(p.poseAt) < 2*time.Second && p.player.Pose.Position.Sub(s.cfg.Spawn).Len() <= GroundRadius+5
}

// Called with Server.mu held; a missing observation never means pickup.
func (s *Server) observeGround(p *serverPeer, obs []GroundSource, now time.Time) error {
	if len(obs) > MaxGroundItems {
		return fmt.Errorf("snapshot de materiais excessivo")
	}
	for _, g := range obs {
		if !g.Valid() {
			return fmt.Errorf("material inválido")
		}
	}
	if !s.groundFresh(p, now) {
		return nil
	}
	for _, g := range obs {
		if g.Position.Sub(s.cfg.Spawn).Len() > GroundRadius || g.Position.Sub(p.player.Pose.Position).Len() > 140 {
			continue
		}
		if g.Hash != 0 && p.player.ID != s.environmentHost {
			continue
		}
		if g.Hash == 0 && g.Position.Sub(p.player.Pose.Position).Len() > 140 {
			continue
		}
		key := g.key(p.player.ID)
		id := s.groundKeys[key]
		if id == 0 {
			if g.Drop && !g.NativeDrop {
				continue
			}
			if len(s.groundItems) >= MaxGroundItems {
				if g.NativeDrop {
					if p.groundRejected == nil {
						p.groundRejected = map[uint64]bool{}
					}
					if len(p.groundRejected) < 32 {
						p.groundRejected[g.Local] = true
					}
				}
				continue
			}
			s.groundCounter++
			id = s.groundCounter
			s.groundKeys[key] = id
			s.groundItems[id] = GroundItem{ID: id, Source: g, Owner: p.player.ID, State: "available"}
		}
		item := s.groundItems[id]
		if item.Source.Actor != g.Actor {
			continue
		}
		if item.State == "available" {
			if item.Source.Drop && item.Owner != p.player.ID {
				continue
			}
			if GroundWeapon(g.Actor) && (g.Modifier != item.Source.Modifier || g.ModifierValue != item.Source.ModifierValue || g.Value > item.Source.Value) {
				continue
			}
			item.Source = g
			item.Owner = p.player.ID
			s.groundItems[id] = item
		}
	}
	return nil
}
func (s *Server) claimGround(p *serverPeer, req GroundClaim, now time.Time) error {
	if req.Request == 0 || req.Request > math.MaxInt64 || !req.Source.Valid() || req.Item > MaxGroundItems {
		return fmt.Errorf("pedido de coleta inválido")
	}
	if old, ok := p.groundClaims[req.Request]; ok {
		if old.Source.key(p.player.ID) != req.Source.key(p.player.ID) || old.Source.Actor != req.Source.Actor || (req.Item != 0 && old.Item != req.Item) {
			return fmt.Errorf("identidade do pedido mudou")
		}
		if old.Status != "waiting" {
			return nil
		}
	}
	if len(p.groundClaims) >= 128 {
		if _, exists := p.groundClaims[req.Request]; !exists {
			return fmt.Errorf("limite de pedidos de coleta da conexão atingido")
		}
	}
	if _, exists := p.groundClaims[req.Request]; !exists {
		pending := 0
		for _, c := range p.groundClaims {
			if c.Status == "waiting" || c.Status == "granted" {
				pending++
			}
		}
		if pending >= MaxGroundItems {
			req.Status = "denied"
			p.groundClaims[req.Request] = req
			return nil
		}
	}
	req.Status = "denied"
	if !s.groundFresh(p, now) || req.Source.Position.Sub(p.player.Pose.Position).Len() > 3.5 {
		p.groundClaims[req.Request] = req
		return nil
	}
	id := req.Item
	if id == 0 {
		id = s.groundKeys[req.Source.key(p.player.ID)]
	}
	if id == 0 && req.Source.Hash == 0 {
		if err := s.observeGround(p, []GroundSource{req.Source}, now); err != nil {
			return err
		}
		id = s.groundKeys[req.Source.key(p.player.ID)]
	}
	item, ok := s.groundItems[id]
	if !ok {
		if len(s.groundItems) >= MaxGroundItems {
			p.groundClaims[req.Request] = req
			return nil
		}
		req.Status = "waiting"
		p.groundClaims[req.Request] = req
		return nil
	}
	req.Item = id
	if item.Source.Actor != req.Source.Actor || item.Source.Value != req.Source.Value || item.Source.Modifier != req.Source.Modifier || item.Source.ModifierValue != req.Source.ModifierValue || item.Source.key(item.Owner) != req.Source.key(p.player.ID) || item.Source.Position.Sub(p.player.Pose.Position).Len() > 3.5 {
		p.groundClaims[req.Request] = req
		return nil
	}
	if item.State == "available" {
		item.State = "reserved"
		item.Claimant = p.player.ID
		item.Request = req.Request
		s.groundItems[id] = item
		req.Status = "granted"
	}
	p.groundClaims[req.Request] = req
	return nil
}
func (s *Server) receiptGround(p *serverPeer, receipt GroundReceipt) error {
	item, ok := s.groundItems[receipt.Item]
	if !ok || receipt.Request == 0 {
		return fmt.Errorf("recibo de coleta inválido")
	}
	if item.Claimant != p.player.ID || item.Request != receipt.Request {
		return nil
	}
	if item.State == "taken" {
		return nil
	}
	if item.State != "reserved" && item.State != "uncertain" {
		return nil
	}
	claim := p.groundClaims[receipt.Request]
	if receipt.Applied {
		item.State = "taken"
		claim.Status = "taken"
	} else {
		item.State = "available"
		item.Claimant = ""
		item.Request = 0
		claim.Status = "failed"
	}
	s.groundItems[receipt.Item] = item
	p.groundClaims[receipt.Request] = claim
	return nil
}
func (s *Server) groundSnapshot(p *serverPeer) ([]GroundItem, []GroundClaim) {
	items := make([]GroundItem, 0, len(s.groundItems))
	for _, i := range s.groundItems {
		items = append(items, i)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	var pending, terminal []GroundClaim
	for _, c := range p.groundClaims {
		if c.Status == "granted" || c.Status == "waiting" {
			pending = append(pending, c)
		} else {
			terminal = append(terminal, c)
		}
	}

	pending = append(pending, terminal...)
	sort.Slice(pending, func(i, j int) bool { return pending[i].Request < pending[j].Request })
	return items, pending
}

func (c *Client) DestroyGround(ids []uint64) error {
	return c.send(Message{Type: "ground_destroy", GroundDestroyed: ids})
}
func (s *Server) destroyGround(p *serverPeer, ids []uint64, now time.Time) error {
	if len(ids) > MaxGroundItems {
		return fmt.Errorf("eventos de quebra excedem limite")
	}
	if !s.groundFresh(p, now) {
		return nil
	}
	for _, id := range ids {
		item, ok := s.groundItems[id]
		if !ok || !GroundWeapon(item.Source.Actor) || item.Owner != p.player.ID || item.State != "available" {
			continue
		}
		item.State = "cancelled"
		item.Claimant = ""
		item.Request = 0
		s.groundItems[id] = item
	}
	return nil
}
