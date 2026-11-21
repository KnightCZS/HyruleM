package multiplayer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type worldActorSync struct {
	last     time.Time
	world    string
	epoch    uint64
	role     int
	sequence uint64
	captured uint64
}

func (b *Bridge) closeWorldActors(ctx context.Context, n *worldActorSync) bool {
	if n.world == "" {
		return true
	}
	cmd := fmt.Sprintf("hm_world_sample %s %s %d 0 0 0 0 0 0 -", b.nativeSession, n.world, n.epoch)
	if b.command(ctx, cmd, nil) != nil {
		return false
	}
	last := n.last
	*n = worldActorSync{last: last}
	return true
}
func worldActorCommand(session, world string, epoch uint64, role int, w *WorldState) (string, error) {
	if !validOrigin(session) || len(world) != 24 || epoch == 0 || role < 1 || role > 3 {
		return "", fmt.Errorf("sessão de mundo inválida")
	}
	for _, ch := range world {
		if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f') {
			return "", fmt.Errorf("identidade de mundo inválida")
		}
	}
	state := WorldState{}
	blob := "-"
	if role >= 2 {
		if w == nil || !w.Valid() || w.Epoch != epoch {
			return "", fmt.Errorf("quadro autoritativo inválido")
		}
		state = *w
		blob = state.Actors
	}
	cmd := fmt.Sprintf("hm_world_sample %s %s %d %d %d %d %.4f %.4f %.4f %s", session, world, epoch, role, state.Sequence, boolInt(state.Paused), state.Center.X, state.Center.Y, state.Center.Z, blob)
	if role == 3 && state.Scene != "" {
		cmd += " " + state.Scene
	}
	if len(cmd) > 65535 || strings.ContainsAny(cmd, "\r\n\x00") {
		return "", fmt.Errorf("quadro de mundo excessivo")
	}
	return cmd, nil
}
func (b *Bridge) updateWorldActors(ctx context.Context, n *worldActorSync, now time.Time) error {
	if now.Sub(n.last) < 100*time.Millisecond {
		return nil
	}
	n.last = now
	v := b.Client.WorldActors()
	if v.Dedicated {
		return b.updateDedicatedWorld(ctx, n, v, now)
	}
	if v.WorldID == "" || v.HostID == "" || v.Epoch == 0 {
		b.closeWorldActors(ctx, n)
		return nil
	}
	role := 2
	if v.HostID == b.Client.Welcome().ID {
		role = 1
	}
	if role == 2 && (v.State == nil || now.Sub(v.At) > 1200*time.Millisecond) {
		b.closeWorldActors(ctx, n)
		return nil
	}
	if n.world != v.WorldID || n.epoch != v.Epoch || n.role != role {
		if !b.closeWorldActors(ctx, n) {
			return fmt.Errorf("liberação do mundo anterior pendente")
		}
		n.world = v.WorldID
		n.epoch = v.Epoch
		n.role = role
		n.sequence = 0
	}
	if role == 2 && n.sequence == v.State.Sequence {
		return nil
	}
	cmd, err := worldActorCommand(b.nativeSession, v.WorldID, v.Epoch, role, v.State)
	if err != nil {
		return err
	}
	var reply struct {
		Actors                     string `json:"actors_b64"`
		Sequence                   uint64
		Ready, Paused, Complete    bool
		X, Y, Z                    float64
		Captured, Matched, Missing int
	}
	if err = b.command(ctx, cmd, &reply); err != nil {
		return err
	}
	if role == 2 {
		n.sequence = v.State.Sequence
	}
	if role == 1 && reply.Ready && reply.Sequence > n.sequence {
		frame := WorldState{Epoch: v.Epoch, Sequence: reply.Sequence, Center: Vec3{reply.X, reply.Y, reply.Z}, Paused: reply.Paused, Complete: reply.Complete, Actors: reply.Actors}
		if err = b.Client.PublishWorldActors(frame); err != nil {
			return err
		}
		n.sequence = reply.Sequence
	}
	return nil
}

// Role 3 captures native actors and applies only entities leased to another
// client. Native epoch == server lease revision fences frames at the barrier.
func (b *Bridge) updateDedicatedWorld(ctx context.Context, n *worldActorSync, v WorldView, now time.Time) error {
	if !b.distributedWorld {
		return fmt.Errorf("o servidor dedicado exige o runtime da prévia 19")
	}
	if v.State == nil || now.Sub(v.At) > 1200*time.Millisecond {
		b.closeWorldActors(ctx, n)
		return nil
	}
	if !validScene(v.State.Scene) {
		b.closeWorldActors(ctx, n)
		return nil
	}
	if v.State.Lease == 0 {
		return fmt.Errorf("servidor enviou mundo sem revisão de autoridade")
	}
	if n.world != v.WorldID {
		if !b.closeWorldActors(ctx, n) {
			return fmt.Errorf("liberação da autoridade anterior pendente")
		}
		n.world = v.WorldID
		n.role = 3
	}
	// A lease change fences the native frame directly. Closing the world here
	// would destroy reconstructions while unrelated entities change owners.
	n.epoch = v.State.Lease
	frame := *v.State
	frame.Epoch = v.State.Lease
	cmd, err := worldActorCommand(b.nativeSession, v.WorldID, frame.Epoch, 3, &frame)
	if err != nil {
		return err
	}
	var reply struct {
		Actors                  string `json:"actors_b64"`
		Replicas                string `json:"replicas_json"`
		Sequence                uint64
		Ready, Paused, Complete bool
		X, Y, Z                 float64
	}
	if err = b.command(ctx, cmd, &reply); err != nil {
		return err
	}
	if reply.Ready && reply.Sequence > n.captured {
		var replicas []uint64
		if reply.Replicas != "" {
			if err = json.Unmarshal([]byte(reply.Replicas), &replicas); err != nil {
				return fmt.Errorf("identidades de cópias nativas inválidas: %w", err)
			}
		}
		observation := WorldState{Scene: v.State.Scene, Epoch: v.Epoch, Lease: v.State.Lease, Sequence: reply.Sequence,
			Center: Vec3{reply.X, reply.Y, reply.Z}, Paused: reply.Paused, Complete: reply.Complete, Actors: reply.Actors, Replicas: replicas}
		if err = b.Client.PublishWorldActors(observation); err != nil {
			return err
		}
		n.captured = reply.Sequence
	}
	return nil
}
