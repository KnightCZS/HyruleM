// Package multiplayer implements HyruleM's two-player visual prototype. The
// protocol carries presence and opt-in nearby map-enemy health convergence.
// Inventory, player health and enemy AI are never copied by this protocol.
package multiplayer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const Protocol = 4
const MaxLine = 65536
const MaxPlayers = 2

type Vec3 struct{ X, Y, Z float64 }

func (a Vec3) Add(b Vec3) Vec3    { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) Sub(b Vec3) Vec3    { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a Vec3) Mul(v float64) Vec3 { return Vec3{a.X * v, a.Y * v, a.Z * v} }
func (a Vec3) Dot(b Vec3) float64 { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}
func (a Vec3) Len() float64 { return math.Sqrt(a.Dot(a)) }
func (a Vec3) Unit() Vec3 {
	n := a.Len()
	if n < 1e-9 {
		return Vec3{}
	}
	return a.Mul(1 / n)
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func (a Vec3) Valid() bool {
	return finite(a.X) && finite(a.Y) && finite(a.Z) && math.Abs(a.X) <= 100000 && math.Abs(a.Y) <= 50000 && math.Abs(a.Z) <= 100000
}

type Pose struct {
	Position Vec3           `json:"position"`
	Velocity Vec3           `json:"velocity"`
	Yaw      float64        `json:"yaw"`
	Phase    float64        `json:"phase"`
	Ready    bool           `json:"ready"`
	Upper    AnimationTrack `json:"upper,omitempty"`
	Lower    AnimationTrack `json:"lower,omitempty"`
}
type AnimationTrack struct {
	Name     string `json:"name,omitempty"`
	Sequence uint64 `json:"sequence,omitempty"`
	Force    bool   `json:"force,omitempty"`
}

func (a AnimationTrack) Valid() bool {
	if len(a.Name) > 63 || a.Sequence > math.MaxInt64 {
		return false
	}
	for _, r := range a.Name {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return a.Name != "-"
}

func (p Pose) Valid() bool {
	return p.Position.Valid() && p.Velocity.Valid() && p.Velocity.Len() <= 500 && finite(p.Yaw) && finite(p.Phase) && math.Abs(p.Yaw) <= 10000 && math.Abs(p.Phase) <= 10000 && p.Upper.Valid() && p.Lower.Valid()
}

type Player struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slot int    `json:"slot"`
	Pose Pose   `json:"pose"`
}
type Message struct {
	Type     string        `json:"type"`
	Protocol int           `json:"protocol,omitempty"`
	Name     string        `json:"name,omitempty"`
	ID       string        `json:"id,omitempty"`
	Slot     int           `json:"slot,omitempty"`
	Spawn    *Vec3         `json:"spawn,omitempty"`
	Pose     *Pose         `json:"pose,omitempty"`
	Players  []Player      `json:"players,omitempty"`
	Sequence uint64        `json:"sequence,omitempty"`
	Error    string        `json:"error,omitempty"`
	Enemies  []EnemyHealth `json:"enemies,omitempty"`
}

func NormalizeName(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) < 1 || utf8.RuneCountInString(s) > 24 {
		return "", fmt.Errorf("o nome precisa ter de 1 a 24 caracteres")
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return "", fmt.Errorf("o nome contém um caractere não permitido")
		}
	}
	return s, nil
}
func Address(ip, port string) (string, error) {
	ip = strings.TrimSpace(ip)
	ip = strings.TrimPrefix(strings.TrimSuffix(ip, "]"), "[")
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("informe um IP válido, por exemplo 127.0.0.1")
	}
	p, e := strconv.Atoi(strings.TrimSpace(port))
	if e != nil || p < 1 || p > 65535 {
		return "", fmt.Errorf("a porta precisa estar entre 1 e 65535")
	}
	return net.JoinHostPort(ip, strconv.Itoa(p)), nil
}
func ReadMessage(r *bufio.Reader) (Message, error) {
	var m Message
	line, e := r.ReadSlice('\n')
	if e != nil {
		if e == bufio.ErrBufferFull {
			return m, fmt.Errorf("pacote excede o limite")
		}
		return m, e
	}
	if len(line) > MaxLine {
		return m, fmt.Errorf("pacote excede o limite")
	}
	dec := json.NewDecoder(strings.NewReader(string(line)))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&m); e != nil {
		return m, e
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return m, fmt.Errorf("dados extras no pacote")
	}
	return m, nil
}
func WriteMessage(w io.Writer, m Message) error {
	b, e := json.Marshal(m)
	if e != nil {
		return e
	}
	if len(b)+1 > MaxLine {
		return fmt.Errorf("pacote excede o limite")
	}
	b = append(b, '\n')
	for len(b) > 0 {
		n, e := w.Write(b)
		if e != nil {
			return e
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
