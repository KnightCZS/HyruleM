package launcher

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ControlStream reuses one local socket for continuous telemetry. The runtime
// supports multiple newline requests per connection; one-shot UI actions keep
// using ControlClient. It never connects to a remote game control endpoint.
type ControlStream struct {
	Port    int
	Timeout time.Duration
	mu      sync.Mutex
	conn    net.Conn
	reader  *bufio.Reader
}

func (s *ControlStream) reset() {
	if s.conn != nil {
		s.conn.Close()
	}
	s.conn = nil
	s.reader = nil
}
func (s *ControlStream) Close() { s.mu.Lock(); defer s.mu.Unlock(); s.reset() }
func (s *ControlStream) Command(ctx context.Context, cmd string) (ControlReply, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var r ControlReply
	if s.Port < 1024 || s.Port > 65535 || cmd == "" || len(cmd) > 256 || strings.ContainsAny(cmd, "\r\n\x00") {
		return r, fmt.Errorf("comando local inválido")
	}
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if s.conn == nil {
		conn, e := (&net.Dialer{}).DialContext(ctx, "tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(s.Port)))
		if e != nil {
			return r, e
		}
		s.conn = conn
		s.reader = bufio.NewReaderSize(conn, 128*1024)
	}
	conn := s.conn
	deadline, _ := ctx.Deadline()
	conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if _, e := fmt.Fprintf(conn, "%s\n", cmd); e != nil {
		s.reset()
		return r, e
	}
	line, e := s.reader.ReadSlice('\n')
	if e != nil {
		s.reset()
		return r, e
	}
	if len(line) > 128*1024 {
		s.reset()
		return r, fmt.Errorf("resposta local excede o limite")
	}
	if e = json.Unmarshal(line, &r); e != nil {
		s.reset()
		return r, e
	}
	if !r.OK {
		var msg string
		if json.Unmarshal(r.Error, &msg) != nil {
			msg = string(r.Error)
		}
		return r, fmt.Errorf("%s", msg)
	}
	if len(r.Data) == 0 {
		return r, fmt.Errorf("resposta sem dados")
	}
	return r, nil
}
