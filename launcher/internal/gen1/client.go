package gen1

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

type Client struct {
	mu        sync.Mutex
	sendMu    sync.Mutex
	conn      net.Conn
	id, token string
	state     Snapshot
	next      uint64
	pending   map[uint64]chan Result
	done      chan struct{}
	once      sync.Once
	reason    string
}

func Connect(ctx context.Context, address, name, token string, caps Capabilities) (*Client, error) {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	conn, e := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if e != nil {
		return nil, e
	}
	fail := func(e error) (*Client, error) { conn.Close(); return nil, e }
	deadline, _ := ctx.Deadline()
	conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if e = WritePacket(conn, Packet{Type: "hello", Protocol: Protocol, Name: name, ResumeToken: token, Capabilities: &caps}); e != nil {
		return fail(e)
	}
	reader := bufio.NewReaderSize(conn, MaxPacket)
	p, e := ReadPacket(reader)
	if e != nil {
		return fail(e)
	}
	if p.Type == "error" {
		return fail(fmt.Errorf("%s", p.Error))
	}
	if p.Type != "welcome" || p.Protocol != Protocol || !identifier(p.ID) || len(p.ResumeToken) != 48 {
		return fail(fmt.Errorf("resposta nativa inválida"))
	}
	conn.SetDeadline(time.Time{})
	c := &Client{conn: conn, id: p.ID, token: p.ResumeToken, pending: map[uint64]chan Result{}, done: make(chan struct{})}
	go c.read(reader)
	go c.heartbeat()
	return c, nil
}
func (c *Client) ID() string            { return c.id }
func (c *Client) ResumeToken() string   { return c.token }
func (c *Client) Done() <-chan struct{} { return c.done }
func (c *Client) fail(reason string) {
	c.once.Do(func() { c.mu.Lock(); c.reason = reason; c.mu.Unlock(); close(c.done); c.conn.Close() })
}
func (c *Client) Close() { c.fail("cliente encerrado") }
func (c *Client) write(p Packet) error {
	c.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if e := WritePacket(c.conn, p); e != nil {
		c.fail(e.Error())
		return e
	}
	return nil
}
func (c *Client) Do(ctx context.Context, command Command) (Result, error) {
	c.sendMu.Lock()
	c.mu.Lock()
	c.next++
	command.Sequence = c.next
	response := make(chan Result, 1)
	c.pending[c.next] = response
	c.mu.Unlock()
	e := c.write(Packet{Type: "command", Protocol: Protocol, Command: &command})
	c.sendMu.Unlock()
	defer func() { c.mu.Lock(); delete(c.pending, command.Sequence); c.mu.Unlock() }()
	if e != nil {
		return Result{}, e
	}
	select {
	case r := <-response:
		return r, nil
	case <-ctx.Done():
		return Result{}, ctx.Err()
	case <-c.done:
		c.mu.Lock()
		defer c.mu.Unlock()
		return Result{}, fmt.Errorf("%s", c.reason)
	}
}
func (c *Client) Snapshot() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.state
	s.Personal.Inventory = append([]Item{}, s.Personal.Inventory...)
	s.Players = append([]PublicPlayer{}, s.Players...)
	s.Enemies = append([]Enemy{}, s.Enemies...)
	s.Items = append([]WorldItem{}, s.Items...)
	return s
}
func (c *Client) heartbeat() {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-t.C:
			c.sendMu.Lock()
			e := c.write(Packet{Type: "ping", Protocol: Protocol})
			c.sendMu.Unlock()
			if e != nil {
				return
			}
		}
	}
}
func (c *Client) read(r *bufio.Reader) {
	for {
		c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		p, e := ReadPacket(r)
		if e != nil {
			c.fail(e.Error())
			return
		}
		if p.Protocol != Protocol {
			c.fail("protocolo inválido")
			return
		}
		switch p.Type {
		case "result":
			if p.Result == nil {
				c.fail("resultado inválido")
				return
			}
			c.mu.Lock()
			ch := c.pending[p.Result.Request]
			c.mu.Unlock()
			if ch != nil {
				select {
				case ch <- *p.Result:
				default:
				}
			}
		case "snapshot":
			if p.State == nil || p.State.Personal.ID != c.id || len(p.State.Players) > MaxPlayers || len(p.State.Enemies) > MaxEntities || len(p.State.Items) > MaxEntities || len(p.State.Personal.Inventory) > MaxInventory {
				c.fail("snapshot inválido")
				return
			}
			c.mu.Lock()
			if p.State.Sequence > c.state.Sequence {
				c.state = *p.State
			}
			c.mu.Unlock()
		default:
			c.fail("pacote desconhecido")
			return
		}
	}
}
