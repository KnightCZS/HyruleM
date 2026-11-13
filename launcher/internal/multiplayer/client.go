package multiplayer

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

type Welcome struct {
	ID    string
	Slot  int
	Spawn Vec3
}
type Client struct {
	mu        sync.Mutex
	writeMu   sync.Mutex
	conn      net.Conn
	welcome   Welcome
	frames    []Frame
	latest    uint64
	connected bool
	reason    string
	done      chan struct{}
	once      sync.Once
	onStatus  func(string)
}
type Frame struct {
	At      time.Time
	Players []Player
}

func Connect(ctx context.Context, address, name string, onStatus func(string)) (*Client, error) {
	name, e := NormalizeName(name)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	c, e := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if e != nil {
		return nil, e
	}
	fail := func(e error) (*Client, error) { c.Close(); return nil, e }
	deadline, _ := ctx.Deadline()
	c.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	if e = WriteMessage(c, Message{Type: "hello", Protocol: Protocol, Name: name}); e != nil {
		return fail(e)
	}
	reader := bufio.NewReaderSize(c, MaxLine)
	m, e := ReadMessage(reader)
	if e != nil {
		return fail(e)
	}
	if m.Type == "error" {
		return fail(fmt.Errorf("%s", m.Error))
	}
	if m.Type != "welcome" || m.Protocol != Protocol || m.ID == "" || m.Slot < 0 || m.Slot >= MaxPlayers || m.Spawn == nil || !m.Spawn.Valid() {
		return fail(fmt.Errorf("resposta de conexão inválida"))
	}
	c.SetDeadline(time.Time{})
	cl := &Client{conn: c, welcome: Welcome{m.ID, m.Slot, *m.Spawn}, connected: true, done: make(chan struct{}), onStatus: onStatus}
	go cl.read(reader)
	go cl.heartbeat()
	return cl, nil
}
func (c *Client) Reason() string        { c.mu.Lock(); defer c.mu.Unlock(); return c.reason }
func (c *Client) Welcome() Welcome      { return c.welcome }
func (c *Client) Connected() bool       { c.mu.Lock(); defer c.mu.Unlock(); return c.connected }
func (c *Client) Done() <-chan struct{} { return c.done }
func (c *Client) send(m Message) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if !c.Connected() {
		return fmt.Errorf("jogador desconectado")
	}
	c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	e := WriteMessage(c.conn, m)
	if e != nil {
		c.fail("Conexão encerrada: " + e.Error())
	}
	return e
}
func (c *Client) Publish(p Pose) error {
	if !p.Valid() {
		return fmt.Errorf("posição inválida")
	}
	return c.send(Message{Type: "pose", Pose: &p})
}
func (c *Client) Rename(name string) error {
	n, e := NormalizeName(name)
	if e != nil {
		return e
	}
	return c.send(Message{Type: "rename", Name: n})
}
func (c *Client) fail(reason string) {
	c.once.Do(func() {
		c.mu.Lock()
		c.connected = false
		c.reason = reason
		c.frames = nil
		c.mu.Unlock()
		close(c.done)
		c.conn.Close()
		if c.onStatus != nil {
			c.onStatus(reason)
		}
	})
}
func (c *Client) Close() { _ = c.send(Message{Type: "leave"}); c.fail("Desconectado") }
func (c *Client) read(r *bufio.Reader) {
	for {
		c.conn.SetReadDeadline(time.Now().Add(8 * time.Second))
		m, e := ReadMessage(r)
		if e != nil {
			c.fail("Conexão com o servidor encerrada: " + e.Error())
			return
		}
		if m.Type != "snapshot" || m.Protocol != Protocol || len(m.Players) > MaxPlayers {
			c.fail("Pacote do servidor inválido")
			return
		}
		ids := map[string]bool{}
		slots := map[int]bool{}
		for _, p := range m.Players {
			if p.ID == "" || ids[p.ID] || slots[p.Slot] || p.Slot < 0 || p.Slot >= MaxPlayers || !p.Pose.Valid() {
				c.fail("Estado de jogador inválido")
				return
			}
			ids[p.ID] = true
			slots[p.Slot] = true
			if _, e = NormalizeName(p.Name); e != nil {
				c.fail("Nome inválido no servidor")
				return
			}
		}
		c.mu.Lock()
		if m.Sequence > c.latest {
			c.latest = m.Sequence
			c.frames = append(c.frames, Frame{At: time.Now(), Players: append([]Player(nil), m.Players...)})
			if len(c.frames) > 6 {
				c.frames = c.frames[len(c.frames)-6:]
			}
		}
		c.mu.Unlock()
	}
}
func (c *Client) heartbeat() {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-t.C:
			if c.send(Message{Type: "ping"}) != nil {
				return
			}
		}
	}
}
func (c *Client) Remote(now time.Time) []Player {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.connected || len(c.frames) == 0 {
		return nil
	}
	last := c.frames[len(c.frames)-1]
	if now.Sub(last.At) > time.Second {
		return nil
	}
	target := now.Add(-100 * time.Millisecond)
	a, b := last, last
	for i := 0; i+1 < len(c.frames); i++ {
		if !target.Before(c.frames[i].At) && !target.After(c.frames[i+1].At) {
			a, b = c.frames[i], c.frames[i+1]
			break
		}
	}
	alpha := 1.0
	if dt := b.At.Sub(a.At).Seconds(); dt > 0 {
		alpha = target.Sub(a.At).Seconds() / dt
		if alpha < 0 {
			alpha = 0
		}
		if alpha > 1 {
			alpha = 1
		}
	}
	var out []Player
	for _, p := range b.Players {
		if p.ID == c.welcome.ID || !p.Pose.Ready {
			continue
		}
		for _, old := range a.Players {
			if old.ID == p.ID && old.Pose.Ready && p.Pose.Position.Sub(old.Pose.Position).Len() < 20 {
				p.Pose.Position = old.Pose.Position.Mul(1 - alpha).Add(p.Pose.Position.Mul(alpha))
				break
			}
		}
		out = append(out, p)
	}
	return out
}
