package multiplayer

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"
)

type ServerConfig struct {
	Spawn Vec3
	Tick  time.Duration
	Log   func(string)
}
type serverPeer struct {
	conn   net.Conn
	player Player
	out    chan Message
	done   chan struct{}
	once   sync.Once
}

func (p *serverPeer) stop() { p.once.Do(func() { close(p.done); p.conn.Close() }) }

type Server struct {
	mu          sync.Mutex
	cfg         ServerConfig
	peers       map[string]*serverPeer
	connections map[net.Conn]bool
	listener    net.Listener
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	closed      bool
}

func Listen(address string, cfg ServerConfig) (*Server, error) {
	if !cfg.Spawn.Valid() {
		return nil, fmt.Errorf("posição inicial inválida")
	}
	if cfg.Tick <= 0 {
		cfg.Tick = 50 * time.Millisecond
	}
	if cfg.Tick < 20*time.Millisecond {
		cfg.Tick = 20 * time.Millisecond
	}
	l, e := net.Listen("tcp", address)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{cfg: cfg, peers: map[string]*serverPeer{}, connections: map[net.Conn]bool{}, listener: l, cancel: cancel}
	s.wg.Add(2)
	go s.accept(ctx)
	go s.broadcast(ctx)
	return s, nil
}
func (s *Server) Addr() net.Addr { return s.listener.Addr() }
func (s *Server) Count() int     { s.mu.Lock(); defer s.mu.Unlock(); return len(s.peers) }
func (s *Server) log(msg string) {
	if s.cfg.Log != nil {
		s.cfg.Log(msg)
	}
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
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	reader := bufio.NewReaderSize(c, MaxLine)
	hello, e := ReadMessage(reader)
	if e != nil {
		return
	}
	name, e := NormalizeName(hello.Name)
	if e != nil || hello.Type != "hello" || hello.Protocol != Protocol {
		c.SetWriteDeadline(time.Now().Add(time.Second))
		WriteMessage(c, Message{Type: "error", Error: "nome ou versão de protocolo inválidos"})
		return
	}
	s.mu.Lock()
	if s.closed || len(s.peers) >= MaxPlayers {
		s.mu.Unlock()
		c.SetWriteDeadline(time.Now().Add(time.Second))
		WriteMessage(c, Message{Type: "error", Error: "servidor completo: este protótipo aceita dois jogadores"})
		return
	}
	var idBytes [12]byte
	if _, e = rand.Read(idBytes[:]); e != nil {
		s.mu.Unlock()
		return
	}
	id := hex.EncodeToString(idBytes[:])
	slot := 0
	for _, p := range s.peers {
		if p.player.Slot == 0 {
			slot = 1
		}
	}
	p := &serverPeer{conn: c, player: Player{ID: id, Name: name, Slot: slot}, out: make(chan Message, 2), done: make(chan struct{})}
	s.peers[id] = p
	s.mu.Unlock()
	defer func() { p.stop(); s.mu.Lock(); delete(s.peers, id); s.mu.Unlock(); s.log(name + " saiu") }()
	spawn := s.cfg.Spawn
	spawn.X += float64(slot*2-1) * 2
	spawn.Y += 0.8
	c.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if e = WriteMessage(c, Message{Type: "welcome", Protocol: Protocol, ID: id, Slot: slot, Spawn: &spawn}); e != nil {
		return
	}
	s.log(name + " entrou")
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for {
			select {
			case <-ctx.Done():
				p.stop()
				return
			case <-p.done:
				return
			case m := <-p.out:
				c.SetWriteDeadline(time.Now().Add(3 * time.Second))
				if WriteMessage(c, m) != nil {
					p.stop()
					return
				}
			}
		}
	}()
	defer func() { p.stop(); <-writerDone }()
	for {
		c.SetReadDeadline(time.Now().Add(8 * time.Second))
		m, e := ReadMessage(reader)
		if e != nil {
			return
		}
		switch m.Type {
		case "pose":
			if m.Pose == nil || !m.Pose.Valid() {
				return
			}
			s.mu.Lock()
			p.player.Pose = *m.Pose
			s.mu.Unlock()
		case "rename":
			newName, e := NormalizeName(m.Name)
			if e != nil {
				return
			}
			s.mu.Lock()
			p.player.Name = newName
			s.mu.Unlock()
		case "ping":
		case "leave":
			return
		default:
			return
		}
	}
}
func (s *Server) broadcast(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.cfg.Tick)
	defer ticker.Stop()
	var sequence uint64
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		s.mu.Lock()
		players := make([]Player, 0, len(s.peers))
		targets := make([]*serverPeer, 0, len(s.peers))
		for _, p := range s.peers {
			players = append(players, p.player)
			targets = append(targets, p)
		}
		s.mu.Unlock()
		sort.Slice(players, func(i, j int) bool { return players[i].Slot < players[j].Slot })
		sequence++
		m := Message{Type: "snapshot", Protocol: Protocol, Players: players, Sequence: sequence}
		for _, p := range targets {
			select {
			case <-p.done:
				continue
			default:
			}
			select {
			case p.out <- m:
			default:
				select {
				case <-p.out:
				default:
				}
				select {
				case p.out <- m:
				default:
				}
			}
		}
	}
}
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
	for _, p := range s.peers {
		p.stop()
	}
	s.mu.Unlock()
	s.wg.Wait()
	return e
}
