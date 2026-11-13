package gen1

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type Packet struct {
	Type         string        `json:"type"`
	Protocol     int           `json:"protocol,omitempty"`
	Name         string        `json:"name,omitempty"`
	Capabilities *Capabilities `json:"capabilities,omitempty"`
	ResumeToken  string        `json:"resume_token,omitempty"`
	ID           string        `json:"id,omitempty"`
	Command      *Command      `json:"command,omitempty"`
	Result       *Result       `json:"result,omitempty"`
	State        *Snapshot     `json:"state,omitempty"`
	Error        string        `json:"error,omitempty"`
}

func ReadPacket(r *bufio.Reader) (Packet, error) {
	var p Packet
	b, e := r.ReadSlice('\n')
	if e != nil {
		return p, e
	}
	if len(b) > MaxPacket {
		return p, fmt.Errorf("pacote excede o limite")
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&p); e != nil {
		return p, e
	}
	var more any
	if decoder.Decode(&more) != io.EOF {
		return p, fmt.Errorf("dados adicionais no pacote")
	}
	return p, nil
}
func WritePacket(w io.Writer, p Packet) error {
	b, e := json.Marshal(p)
	if e != nil {
		return e
	}
	if len(b)+1 > MaxPacket {
		return fmt.Errorf("pacote excede o limite")
	}
	b = append(b, '\n')
	for len(b) > 0 {
		n, e := w.Write(b)
		if e != nil {
			return e
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}

type netPeer struct {
	conn   net.Conn
	id     string
	events chan Packet
	latest chan Packet
	done   chan struct{}
	once   sync.Once
}

func (p *netPeer) stop() { p.once.Do(func() { close(p.done); p.conn.Close() }) }

type Server struct {
	mu          sync.Mutex
	listener    net.Listener
	world       *World
	peers       map[string]*netPeer
	connections map[net.Conn]bool
	cancel      context.CancelFunc
	closed      bool
	wg          sync.WaitGroup
}

func Listen(address string, cfg Config) (*Server, error) {
	world, e := NewWorld(cfg)
	if e != nil {
		return nil, e
	}
	listener, e := net.Listen("tcp", address)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{listener: listener, world: world, peers: map[string]*netPeer{}, connections: map[net.Conn]bool{}, cancel: cancel}
	s.wg.Add(2)
	go s.accept(ctx)
	go s.broadcast(ctx)
	return s, nil
}
func (s *Server) Addr() net.Addr { return s.listener.Addr() }
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.cancel()
	e := s.listener.Close()
	for c := range s.connections {
		c.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
	return e
}
func (s *Server) accept(ctx context.Context) {
	defer s.wg.Done()
	for {
		c, e := s.listener.Accept()
		if e != nil {
			return
		}
		s.mu.Lock()
		if s.closed || len(s.connections) >= 8 {
			s.mu.Unlock()
			c.Close()
			continue
		}
		s.connections[c] = true
		s.wg.Add(1)
		s.mu.Unlock()
		go s.serve(ctx, c)
	}
}
func (s *Server) serve(ctx context.Context, c net.Conn) {
	defer s.wg.Done()
	defer func() { c.Close(); s.mu.Lock(); delete(s.connections, c); s.mu.Unlock() }()
	c.SetDeadline(time.Now().Add(5 * time.Second))
	reader := bufio.NewReaderSize(c, MaxPacket)
	hello, e := ReadPacket(reader)
	fail := func(msg string) {
		c.SetWriteDeadline(time.Now().Add(time.Second))
		_ = WritePacket(c, Packet{Type: "error", Protocol: Protocol, Error: msg})
	}
	if e != nil {
		return
	}
	if hello.Type != "hello" || hello.Protocol != Protocol || hello.Capabilities == nil {
		fail("protocolo nativo incompatível; clientes visuais 0.4.x usam outro servidor")
		return
	}
	var raw [12]byte
	if _, e = rand.Read(raw[:]); e != nil {
		return
	}
	id := hex.EncodeToString(raw[:])
	if _, e = s.world.Resume(id, hello.Name, *hello.Capabilities, hello.ResumeToken); e != nil {
		fail(e.Error())
		return
	}
	peer := &netPeer{conn: c, id: id, events: make(chan Packet, 16), latest: make(chan Packet, 1), done: make(chan struct{})}
	defer func() { peer.stop(); s.mu.Lock(); delete(s.peers, id); s.mu.Unlock(); s.world.Leave(id) }()
	if e = WritePacket(c, Packet{Type: "welcome", Protocol: Protocol, ID: id, ResumeToken: s.world.SessionToken(id)}); e != nil {
		return
	}
	c.SetDeadline(time.Time{})
	s.mu.Lock()
	s.peers[id] = peer
	s.mu.Unlock()
	written := make(chan struct{})
	go func() {
		defer close(written)
		for {
			var packet Packet
			select {
			case <-ctx.Done():
				peer.stop()
				return
			case <-peer.done:
				return
			case packet = <-peer.events:
			case packet = <-peer.latest:
			}
			c.SetWriteDeadline(time.Now().Add(3 * time.Second))
			if WritePacket(c, packet) != nil {
				peer.stop()
				return
			}
		}
	}()
	defer func() { peer.stop(); <-written }()
	for {
		c.SetReadDeadline(time.Now().Add(10 * time.Second))
		packet, e := ReadPacket(reader)
		if e != nil {
			return
		}
		switch packet.Type {
		case "ping":
		case "leave":
			return
		case "command":
			if packet.Command == nil {
				return
			}
			result := s.world.Apply(id, *packet.Command)
			select {
			case peer.events <- Packet{Type: "result", Protocol: Protocol, Result: &result}:
			default:
				return
			}
		default:
			return
		}
	}
}
func (s *Server) broadcast(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		s.mu.Lock()
		peers := make([]*netPeer, 0, len(s.peers))
		for _, p := range s.peers {
			peers = append(peers, p)
		}
		s.mu.Unlock()
		for _, p := range peers {
			snapshot, e := s.world.Snapshot(p.id)
			if e != nil {
				continue
			}
			packet := Packet{Type: "snapshot", Protocol: Protocol, State: &snapshot}
			select {
			case p.latest <- packet:
			default:
				select {
				case <-p.latest:
				default:
				}
				select {
				case p.latest <- packet:
				default:
				}
			}
		}
	}
}
