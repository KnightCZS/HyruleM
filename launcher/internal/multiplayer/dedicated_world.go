package multiplayer

import (
	"fmt"
	"math"
	"sort"
	"time"
)

const serverAuthority = "server"
const maxWorldEntities = 32768

type worldEntity struct {
	Settled bool
	State   WorldActor
	Owner   string
	At      time.Time
}
type worldObservation struct {
	State WorldActor
	At    time.Time
}

// The dedicated process owns the registry. Native simulation is leased to a
// client that has the entity loaded. Every ownership change fences old frames.
func (s *Server) acceptDedicatedWorldLocked(p *serverPeer, w *WorldState, now time.Time) error {
	if w == nil || !w.Valid() || w.AgeMS != 0 {
		return fmt.Errorf("observação de mundo inválida")
	}
	if w.Scene != p.player.Pose.Scene || !validScene(w.Scene) {
		return nil
	}
	if w.Epoch != s.environmentEpoch || w.Lease != s.worldLease || w.Sequence <= p.worldSequence {
		return nil
	}
	if !p.player.Pose.Ready || now.Sub(p.poseAt) > 2*time.Second || w.Center.Sub(p.player.Pose.Position).Len() > 20 {
		return nil
	}
	rows, _ := decodeWorldActors(w.Actors)
	p.worldSequence = w.Sequence
	p.worldPaused = w.Paused
	p.worldObserved = make(map[string]worldObservation, len(rows))
	for _, a := range rows {
		a.Scene = w.Scene
		key := a.key()
		p.worldObserved[key] = worldObservation{a, now}
		entity := s.entities[key]
		if entity == nil {
			if len(s.entities) >= maxWorldEntities || s.entityCounter >= math.MaxInt64 {
				continue
			}
			s.entityCounter++
			a.ID = s.entityCounter
			s.entities[key] = &worldEntity{State: a, Owner: p.player.ID, At: now, Settled: true}
			continue
		}
		if entity.Owner == p.player.ID && !w.Paused {
			entity.At = now
			if !entity.Settled {
				if a.position().Sub(entity.State.position()).Len() > 1.5 {
					continue
				}
				entity.Settled = true
			}
			a.ID = entity.State.ID
			entity.State = a
		}
	}
	s.reconcileWorldLocked(now)
	return nil
}

func (s *Server) reconcileWorldLocked(now time.Time) {
	changed := false
	for key, entity := range s.entities {
		owner := ""
		best := math.Inf(1)
		for id, p := range s.peers {
			observation, ok := p.worldObserved[key]
			if !ok || p.player.Pose.Scene != entity.State.Scene || now.Sub(observation.At) > 1500*time.Millisecond || !p.player.Pose.Ready ||
				p.worldPaused || now.Sub(p.poseAt) > 2*time.Second {
				continue
			}
			distance := p.player.Pose.Position.Sub(entity.State.position()).Len()
			if distance < best || distance == best && (id == entity.Owner || owner != entity.Owner && id < owner) {
				best = distance
				owner = id
			}
		}
		if owner != entity.Owner {
			entity.Owner = owner
			entity.Settled = false
			entity.At = now
			changed = true
		}
	}
	if changed {
		s.worldLease++
	}
}

func (s *Server) dedicatedWorldSnapshot(p *serverPeer, sequence uint64, now time.Time) *WorldState {
	if p.worldOutput != nil && p.worldOutput.Scene == p.player.Pose.Scene && p.worldOutput.Lease == s.worldLease && now.Sub(p.worldOutputAt) < 100*time.Millisecond && p.worldOutput.Center.Sub(p.player.Pose.Position).Len() < 20 {
		copy := *p.worldOutput
		copy.AgeMS = uint64(now.Sub(p.worldOutputAt) / time.Millisecond)
		return &copy
	}
	rows := make([]WorldActor, 0, MaxWorldActors)
	for _, entity := range s.entities {
		if entity.State.Scene != p.player.Pose.Scene || entity.Owner == p.player.ID && entity.Settled || entity.Owner == "" || now.Sub(entity.At) > 1500*time.Millisecond ||
			entity.State.position().Sub(p.player.Pose.Position).Len() > WorldRadius {
			continue
		}
		rows = append(rows, entity.State)
	}
	sort.Slice(rows, func(i, j int) bool {
		a := rows[i].position().Sub(p.player.Pose.Position).Len()
		b := rows[j].position().Sub(p.player.Pose.Position).Len()
		return a < b || a == b && rows[i].ID < rows[j].ID
	})
	complete := len(rows) <= MaxWorldActors
	if !complete {
		rows = rows[:MaxWorldActors]
	}
	p.worldOutputAt = now
	p.worldOutput = &WorldState{Scene: p.player.Pose.Scene, Epoch: s.environmentEpoch, Lease: s.worldLease, Sequence: sequence,
		Center: p.player.Pose.Position, Actors: encodeWorldActors(rows), Complete: complete}
	return p.worldOutput
}

func (s *Server) advanceEnvironmentLocked(now time.Time) {
	if s.environment == nil {
		s.environment = &Environment{Epoch: s.environmentEpoch, Hour: 12, Weather: "clear"}
	}
	if !s.environment.Frozen && !s.clockAt.IsZero() {
		s.environment.Hour = math.Mod(s.environment.Hour+now.Sub(s.clockAt).Seconds()*24/s.cfg.DayLength.Seconds(), 24)
	}
	s.clockAt = now
	s.environment.Sequence++
	s.environment.Epoch = s.environmentEpoch
	s.environmentAt = now
}
