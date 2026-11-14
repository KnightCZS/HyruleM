package multiplayer

import (
	"fmt"
	"math"
	"time"
)

type GroundDropResult struct {
	Request uint64 `json:"request"`
	Item    uint64 `json:"item"`
	Status  string `json:"status"`
}
type GroundDropFinish struct {
	Request uint64       `json:"request"`
	Item    uint64       `json:"item"`
	Result  int          `json:"result"`
	Source  GroundSource `json:"source"`
}

func (c *Client) LocalPose() (Pose, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.frames) == 0 {
		return Pose{}, false
	}
	for _, p := range c.frames[len(c.frames)-1].Players {
		if p.ID == c.welcome.ID {
			return p.Pose, p.Pose.Ready
		}
	}
	return Pose{}, false
}
func (c *Client) PrepareGroundDrop(request uint64, source GroundSource) error {
	return c.send(Message{Type: "ground_drop_prepare", GroundDrop: &GroundDropFinish{Request: request, Source: source}})
}
func (c *Client) FinishGroundDrop(v GroundDropFinish) error {
	return c.send(Message{Type: "ground_drop_finish", GroundDrop: &v})
}
func (s *Server) prepareGroundDrop(p *serverPeer, req GroundDropFinish, now time.Time) error {
	if req.Request == 0 || req.Request > math.MaxInt64 || req.Source.Local != req.Request || !req.Source.Valid() || !req.Source.Drop || req.Source.Actor != "Item_Fruit_A" {
		return fmt.Errorf("pedido de drop inválido")
	}
	if old := p.groundDrop; old != nil && req.Request <= old.Request {
		return nil
	}
	if old := p.groundDrop; old != nil && old.Status == "prepared" {
		return fmt.Errorf("outro drop ainda está pendente")
	}
	result := &GroundDropResult{Request: req.Request, Status: "rejected"}
	if s.groundFresh(p, now) && req.Source.Position.Sub(p.player.Pose.Position).Len() <= 3.5 && req.Source.Position.Sub(s.cfg.Spawn).Len() <= GroundRadius && len(s.groundItems) < MaxGroundItems {
		key := req.Source.key(p.player.ID)
		if id := s.groundKeys[key]; id != 0 {
			if s.groundItems[id].Owner == p.player.ID && s.groundItems[id].State == "spawning" {
				result.Item = id
				result.Status = "prepared"
			}
		} else {
			s.groundCounter++
			id := s.groundCounter
			s.groundKeys[key] = id
			s.groundItems[id] = GroundItem{ID: id, Source: req.Source, Owner: p.player.ID, State: "spawning"}
			if s.groundItems[id].Owner == p.player.ID && s.groundItems[id].State == "spawning" {
				result.Item = id
				result.Status = "prepared"
			}
		}
	}
	p.groundDrop = result
	return nil
}
func (s *Server) finishGroundDrop(p *serverPeer, req GroundDropFinish) error {
	if req.Request == 0 || req.Item == 0 || req.Item > MaxGroundItems || req.Result < -1 || req.Result > 2 || req.Result == 0 {
		return fmt.Errorf("recibo de drop inválido")
	}
	result := p.groundDrop
	if result == nil || result.Request != req.Request || result.Item != req.Item {
		return nil
	}
	item, ok := s.groundItems[req.Item]
	if !ok || item.Owner != p.player.ID || item.State != "spawning" {
		return nil
	}
	if req.Result == 1 {
		if !req.Source.Valid() || req.Source.key(p.player.ID) != item.Source.key(p.player.ID) || req.Source.Actor != item.Source.Actor || req.Source.Position.Sub(item.Source.Position).Len() > 4 {
			return fmt.Errorf("identidade do drop mudou")
		}
		item.Source = req.Source
		item.State = "available"
		result.Status = "committed"
	} else if req.Result == -1 {
		item.State = "cancelled"
		result.Status = "cancelled"
	} else {
		item.State = "uncertain"
		item.Claimant = p.player.ID
		item.Request = req.Request
		result.Status = "uncertain"
	}
	s.groundItems[item.ID] = item
	return nil
}
