package launcher

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestControlStreamAllowsBoundedAtomicVisualFrameOnly(t *testing.T) {
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	done := make(chan string, 1)
	go func() {
		c, e := l.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		line, e := bufio.NewReader(c).ReadString('\n')
		if e != nil {
			return
		}
		done <- strings.TrimSuffix(line, "\n")
		fmt.Fprintln(c, `{"ok":true,"data":{"queued_sequence":1}}`)
	}()
	s := &ControlStream{Port: l.Addr().(*net.TCPAddr).Port, Timeout: time.Second}
	defer s.Close()
	cmd := "hm_native_remote " + strings.Repeat("x", 2000)
	if _, e := s.Command(context.Background(), cmd); e != nil {
		t.Fatal(e)
	}
	if got := <-done; got != cmd {
		t.Fatal("atomic state truncated")
	}
	for _, bad := range []string{"other " + strings.Repeat("x", 300), "hm_native_remote " + strings.Repeat("x", 4096), "hm_native_remote x\nquit"} {
		if _, e := s.Command(context.Background(), bad); e == nil {
			t.Fatal("invalid local command accepted")
		}
	}
}
