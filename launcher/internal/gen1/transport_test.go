package gen1

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestTCPPrivacyPickupAndNativeCapabilityNegotiation(t *testing.T) {
	cfg := Config{Catalog: testCatalog(), AllowSimulation: true, AllowPVP: true}
	s, e := Listen("127.0.0.1:0", cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	a, e := Connect(ctx, s.Addr().String(), "A", "", testCaps())
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	b, e := Connect(ctx, s.Addr().String(), "B", "", testCaps())
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	do := func(c *Client, command Command) {
		t.Helper()
		r, e := c.Do(ctx, command)
		if e != nil || !r.Accepted {
			t.Fatal(command.Kind, r, e)
		}
	}
	for _, c := range []*Client{a, b} {
		do(c, Command{Kind: "seed", Personal: &Personal{Health: 12, MaxHealth: 12}})
		do(c, Command{Kind: "frame", Frame: &Frame{World: "MainField", Animation: "idle", Ready: true}})
	}
	do(a, Command{Kind: "register_item", Entity: "map:item", Definition: "Weapon_Test", World: "MainField", Amount: 1})
	do(a, Command{Kind: "pickup", Entity: "map:item"})
	until := time.Now().Add(time.Second)
	for time.Now().Before(until) && len(a.Snapshot().Personal.Inventory) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if len(a.Snapshot().Personal.Inventory) != 1 || len(b.Snapshot().Personal.Inventory) != 0 {
		t.Fatal("TCP leaked or lost inventory")
	}
	token := a.ResumeToken()
	a.Close()
	until = time.Now().Add(time.Second)
	var resumed *Client
	for time.Now().Before(until) {
		resumed, e = Connect(ctx, s.Addr().String(), "A back", token, testCaps())
		if e == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if e != nil {
		t.Fatal("resume failed", e)
	}
	defer resumed.Close()
	until = time.Now().Add(time.Second)
	for time.Now().Before(until) && len(resumed.Snapshot().Personal.Inventory) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if len(resumed.Snapshot().Personal.Inventory) != 1 {
		t.Fatal("reconnect lost pickup")
	}
	strict, e := Listen("127.0.0.1:0", Config{Catalog: testCatalog()})
	if e != nil {
		t.Fatal(e)
	}
	defer strict.Close()
	if c, e := Connect(ctx, strict.Addr().String(), "fake", "", testCaps()); e == nil {
		c.Close()
		t.Fatal("simulator entered native server")
	} else if !strings.Contains(e.Error(), "simulado") {
		t.Fatal(e)
	}
}
