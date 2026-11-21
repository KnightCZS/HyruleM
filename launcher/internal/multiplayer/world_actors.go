package multiplayer

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"time"
)

// Bounded loaded-world window; stable identities live in the dedicated registry.
const MaxWorldActors = 64
const WorldRadius = 180
const worldActorStride = 300

type WorldActor struct {
	Scene  string `json:"scene,omitempty"`
	ID     uint64
	Hash   uint32
	Kind   uint32 // enemy, NPC, movable prop, loose map equipment
	Actor  string
	Spawn  Vec3
	Matrix [12]float32
	Tracks [2]AnimationTrack
}
type WorldState struct {
	Replicas []uint64 `json:"replicas,omitempty"`
	Scene    string   `json:"scene,omitempty"`
	Lease    uint64   `json:"lease,omitempty"`
	Epoch    uint64   `json:"epoch"`
	Sequence uint64   `json:"sequence"`
	Center   Vec3     `json:"center"`
	Paused   bool     `json:"paused"`
	Complete bool     `json:"complete"`
	Actors   string   `json:"actors"`
	AgeMS    uint64   `json:"age_ms"`
}
type WorldView struct {
	Dedicated       bool
	WorldID, HostID string
	Epoch           uint64
	State           *WorldState
	At              time.Time
}

// A reference identifies one full sample already delivered on this TCP connection.
func referenceWorld(m Message, previous *WorldView) Message {
	m.Protocol = WorldReferenceProtocol
	w := m.WorldState
	if w != nil && previous.State != nil && previous.WorldID == m.WorldID && previous.HostID == m.HostID &&
		previous.Epoch == m.Epoch && previous.State.Sequence == w.Sequence {
		m.WorldRef, m.WorldState = w.Sequence, nil
	}
	*previous = WorldView{WorldID: m.WorldID, HostID: m.HostID, Epoch: m.Epoch, State: w}
	return m
}

func receivedWorld(m Message, previous WorldView, now time.Time) (WorldView, error) {
	view := WorldView{WorldID: m.WorldID, HostID: m.HostID, Dedicated: m.DedicatedWorld, Epoch: m.Epoch, State: m.WorldState, At: now}
	if m.WorldID != "" {
		raw, err := hex.DecodeString(m.WorldID)
		if err != nil || len(raw) != 12 {
			return WorldView{}, fmt.Errorf("identidade de mundo inválida")
		}
	}
	if m.WorldRef != 0 {
		if m.Protocol != WorldReferenceProtocol || m.WorldState != nil || previous.State == nil ||
			previous.WorldID != m.WorldID || previous.HostID != m.HostID || previous.Epoch != m.Epoch ||
			previous.State.Sequence != m.WorldRef {
			return WorldView{}, fmt.Errorf("referência de mundo sem quadro correspondente")
		}
		return previous, nil // A repeated sample must never renew native authority.
	}
	if w := m.WorldState; w != nil {
		if m.WorldID == "" || m.HostID == "" || w.Epoch != m.Epoch || !w.Valid() {
			return WorldView{}, fmt.Errorf("mundo autoritativo inválido")
		}
		view.At = now.Add(-time.Duration(w.AgeMS) * time.Millisecond)
	}
	return view, nil
}

func worldActorKind(s string) uint32 {
	if strings.HasPrefix(s, "Enemy_") {
		for _, x := range []string{"Ganon", "SiteBoss", "Giant", "Dragon", "Assassin_Senior", "Arrow", "Bullet", "Beam", "Projectile", "Tornado"} {
			if strings.Contains(s, x) {
				return 0
			}
		}
		return 1
	}
	if strings.HasPrefix(s, "Npc_") {
		return 2
	}
	for _, x := range []string{"Obj_Barrel", "Obj_Box", "Obj_LiftRock", "Obj_Rock", "FldObj_KorokStone", "FldObj_KorokBlock", "DgnObj_Small_Box", "DgnObj_IronBall", "Kibako", "Barrel", "DgnObj_StoneBarrel", "Test_MetalBarrel", "FldObj_SnowBallDoor_A_02"} {
		if strings.HasPrefix(s, x) {
			return 3
		}
	}
	if GroundWeapon(s) {
		return 4
	}
	return 0
}
func (a WorldActor) key() string {
	return a.Scene + ":" + fmt.Sprintf("%08x:%s:%.3f:%.3f:%.3f", a.Hash, a.Actor, a.Spawn.X, a.Spawn.Y, a.Spawn.Z)
}
func (a WorldActor) valid() bool {
	if a.Scene != "" && !validScene(a.Scene) {
		return false
	}
	if a.ID == 0 || a.ID > math.MaxInt64 || a.Hash == 0 || a.Hash == math.MaxUint32 || len(a.Actor) > 63 || worldActorKind(a.Actor) != a.Kind || a.Kind == 0 || !a.Spawn.Valid() {
		return false
	}
	for _, ch := range a.Actor {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '_') {
			return false
		}
	}
	for i, v := range a.Matrix {
		limit := float32(8)
		if i%4 == 3 {
			limit = 100000
		}
		if !finite(float64(v)) || v > limit || v < -limit {
			return false
		}
	}
	if !a.position().Valid() {
		return false
	}
	for row := 0; row < 3; row++ {
		var norm float32
		for col := 0; col < 3; col++ {
			v := a.Matrix[row*4+col]
			norm += v * v
		}
		if norm < .0001 {
			return false
		}
	}
	return a.Tracks[0].Valid() && a.Tracks[1].Valid()
}
func (a WorldActor) position() Vec3 {
	return Vec3{float64(a.Matrix[3]), float64(a.Matrix[7]), float64(a.Matrix[11])}
}
func decodeWorldActors(s string) ([]WorldActor, error) {
	fail := func() ([]WorldActor, error) { return nil, fmt.Errorf("quadro de atores do mundo inválido") }
	if len(s) > base64.StdEncoding.EncodedLen(8+MaxWorldActors*worldActorStride) || strings.ContainsAny(s, "\r\n\x00 \t") {
		return fail()
	}
	b, e := base64.StdEncoding.Strict().DecodeString(s)
	if e != nil || len(b) < 8 || string(b[:4]) != "HMW1" || binary.LittleEndian.Uint16(b[6:]) != 0 {
		return fail()
	}
	n := int(binary.LittleEndian.Uint16(b[4:]))
	if n > MaxWorldActors || len(b) != 8+n*worldActorStride {
		return fail()
	}
	rows := make([]WorldActor, n)
	ids := map[uint64]bool{}
	keys := map[string]bool{}
	for i := range rows {
		p := b[8+i*worldActorStride:]
		a := &rows[i]
		a.ID = binary.LittleEndian.Uint64(p)
		a.Hash = binary.LittleEndian.Uint32(p[8:])
		a.Kind = binary.LittleEndian.Uint32(p[12:])
		end := strings.IndexByte(string(p[16:80]), 0)
		if end < 0 {
			return fail()
		}
		a.Actor = string(p[16 : 16+end])
		for _, v := range p[16+end : 80] {
			if v != 0 {
				return fail()
			}
		}
		var spawn [3]float64
		for j := range spawn {
			spawn[j] = float64(math.Float32frombits(binary.LittleEndian.Uint32(p[80+4*j:])))
		}
		a.Spawn = Vec3{spawn[0], spawn[1], spawn[2]}
		for j := range a.Matrix {
			a.Matrix[j] = math.Float32frombits(binary.LittleEndian.Uint32(p[92+4*j:]))
		}
		for j := range a.Tracks {
			t := p[140+80*j:]
			end := strings.IndexByte(string(t[:64]), 0)
			if end < 0 {
				return fail()
			}
			a.Tracks[j].Name = string(t[:end])
			for _, v := range t[end:64] {
				if v != 0 {
					return fail()
				}
			}
			a.Tracks[j].Sequence = binary.LittleEndian.Uint64(t[64:])
			force := binary.LittleEndian.Uint32(t[72:])
			if force > 1 {
				return fail()
			}
			a.Tracks[j].Force = force == 1
			a.Tracks[j].Slot = int(binary.LittleEndian.Uint32(t[76:]))
		}
		if !a.valid() || ids[a.ID] || keys[a.key()] {
			return fail()
		}
		ids[a.ID] = true
		keys[a.key()] = true
	}
	return rows, nil
}
func encodeWorldActors(rows []WorldActor) string {
	b := make([]byte, 8+len(rows)*worldActorStride)
	copy(b, "HMW1")
	binary.LittleEndian.PutUint16(b[4:], uint16(len(rows)))
	for i, a := range rows {
		p := b[8+i*worldActorStride:]
		binary.LittleEndian.PutUint64(p, a.ID)
		binary.LittleEndian.PutUint32(p[8:], a.Hash)
		binary.LittleEndian.PutUint32(p[12:], a.Kind)
		copy(p[16:80], a.Actor)
		for j, v := range []float64{a.Spawn.X, a.Spawn.Y, a.Spawn.Z} {
			binary.LittleEndian.PutUint32(p[80+4*j:], math.Float32bits(float32(v)))
		}
		for j, v := range a.Matrix {
			binary.LittleEndian.PutUint32(p[92+4*j:], math.Float32bits(v))
		}
		for j, t := range a.Tracks {
			q := p[140+80*j:]
			copy(q[:64], t.Name)
			binary.LittleEndian.PutUint64(q[64:], t.Sequence)
			binary.LittleEndian.PutUint32(q[72:], uint32(boolInt(t.Force)))
			binary.LittleEndian.PutUint32(q[76:], uint32(t.Slot))
		}
	}
	return base64.StdEncoding.EncodeToString(b)
}
func (w WorldState) Valid() bool {
	if w.Scene != "" && !validScene(w.Scene) {
		return false
	}
	if w.Epoch == 0 || w.Sequence == 0 || w.Epoch > math.MaxInt64 || w.Sequence > math.MaxInt64 || !w.Center.Valid() || w.AgeMS > 60000 {
		return false
	}
	actors, e := decodeWorldActors(w.Actors)
	if e != nil {
		return false
	}
	if len(w.Replicas) > len(actors) {
		return false
	}
	ids := make(map[uint64]bool, len(actors))
	for _, a := range actors {
		ids[a.ID] = true
	}
	for _, id := range w.Replicas {
		if !ids[id] {
			return false
		}
		delete(ids, id)
	}
	for _, a := range actors {
		if a.position().Sub(w.Center).Len() > WorldRadius+1 {
			return false
		}
	}
	return true
}
func (c *Client) WorldActors() WorldView {
	c.mu.Lock()
	defer c.mu.Unlock()
	v := c.world
	if v.State != nil {
		copy := *v.State
		v.State = &copy
	}
	return v
}
func (c *Client) PublishWorldActors(w WorldState) error {
	if !w.Valid() || w.AgeMS != 0 {
		return fmt.Errorf("estado do mundo inválido")
	}
	return c.send(Message{Type: "world_actors", WorldState: &w})
}
func (s *Server) acceptWorldLocked(p *serverPeer, w *WorldState, now time.Time) error {
	if s.cfg.DedicatedWorld {
		return s.acceptDedicatedWorldLocked(p, w, now)
	}
	if p.player.ID != s.environmentHost {
		return fmt.Errorf("somente o anfitrião pode publicar o mundo")
	}
	if w == nil || !w.Valid() || w.AgeMS != 0 {
		return fmt.Errorf("estado do mundo inválido")
	}
	if w.Epoch != s.environmentEpoch {
		return nil
	}
	if !p.player.Pose.Ready || now.Sub(p.poseAt) > 2*time.Second {
		return nil
	}
	if w.Center.Sub(p.player.Pose.Position).Len() > 20 {
		return nil // A capture from before a teleport is not a protocol violation.
	}
	if s.worldState != nil && w.Sequence <= s.worldState.Sequence {
		return nil
	}
	copy := *w
	s.worldState = &copy
	s.worldAt = now
	return nil
}
