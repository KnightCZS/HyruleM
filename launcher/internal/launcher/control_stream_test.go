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
	for _, prefix := range []string{"hm_native_remote ", "hm_native_sample ", "hm_world_sample "} {
		t.Run(strings.TrimSpace(prefix), func(t *testing.T) {
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
			cmd := prefix + strings.Repeat("x", 65535-len(prefix))
			if _, e := s.Command(context.Background(), cmd); e != nil {
				t.Fatal(e)
			}
			select {
			case got := <-done:
				if got != cmd {
					t.Fatal("atomic state truncated")
				}
			case <-time.After(time.Second):
				t.Fatal("request not received")
			}
			for _, bad := range []string{prefix + strings.Repeat("x", 65536-len(prefix)), prefix + "x\nquit", prefix + "x\rquit", prefix + "x\x00quit", "other " + strings.Repeat("x", 300), "hm_native_sample_extra " + strings.Repeat("x", 300)} {
				if _, e := s.Command(context.Background(), bad); e == nil || e.Error() != "comando local inválido" {
					t.Fatal("invalid local command did not stop at validation", e)
				}
			}
		})
	}
}
