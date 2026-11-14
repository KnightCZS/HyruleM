package multiplayer

import (
	"context"
	"fmt"
	"math"
	"time"
)

// The first admitted player owns the environmental clock. The server chooses
// successors and epochs; a client cannot become host by choosing slot zero.
type Environment struct {
	Epoch    uint64  `json:"epoch"`
	Sequence uint64  `json:"sequence"`
	Hour     float64 `json:"hour"`
	Frozen   bool    `json:"frozen"`
	Weather  string  `json:"weather"`
	AgeMS    uint64  `json:"age_ms"`
}

func weatherValid(s string) bool {
	switch s {
	case "clear", "cloudy", "rain", "heavy_rain", "snow", "heavy_snow", "thunderstorm", "thunder_rain", "sun_shower":
		return true
	}
	return false
}
func weatherPolicyValid(s string) bool {
	return s == "game" || s == "no_rain" || s == "no_storms" || weatherValid(s)
}
func (e Environment) Valid() bool {
	return e.Epoch > 0 && e.Sequence > 0 && finite(e.Hour) && e.Hour >= 0 && e.Hour < 24 && weatherValid(e.Weather) && e.AgeMS <= 60000
}

type EnvironmentView struct {
	HostID   string
	HostName string
	Epoch    uint64
	State    *Environment
	At       time.Time
}

func (c *Client) Environment() EnvironmentView {
	c.mu.Lock()
	defer c.mu.Unlock()
	v := c.environment
	if v.State != nil {
		copy := *v.State
		v.State = &copy
	}
	return v
}
func (c *Client) PublishEnvironment(e Environment) error {
	if !e.Valid() || e.AgeMS != 0 {
		return fmt.Errorf("ambiente inválido")
	}
	return c.send(Message{Type: "environment", Environment: &e})
}
func (s *Server) chooseEnvironmentHostLocked() {
	if _, ok := s.peers[s.environmentHost]; ok {
		return
	}
	s.environmentHost = ""
	slot := MaxPlayers
	for id, p := range s.peers {
		if p.player.Slot < slot {
			slot = p.player.Slot
			s.environmentHost = id
		}
	}
	s.environmentEpoch++
	s.environment = nil
	s.environmentAt = time.Time{}
}

type environmentSync struct {
	last                         time.Time
	epoch, sequence              uint64
	following                    bool
	originalWeather, lastWeather string
	originalFrozen, lastFrozen   bool
	announcedHost                string
	hostInitialized              bool
}
type clockReading struct {
	Hour *float64 `json:"hour"`
	Held bool     `json:"hour_held"`
}
type weatherReading struct {
	Policy string `json:"weather"`
	Now    string `json:"now"`
}

func (b *Bridge) readEnvironment(ctx context.Context) (clockReading, weatherReading, error) {
	var c clockReading
	var w weatherReading
	if err := b.command(ctx, "time", &c); err != nil {
		return c, w, err
	}
	if err := b.command(ctx, "weather", &w); err != nil {
		return c, w, err
	}
	return c, w, nil
}
func (b *Bridge) restoreEnvironment(ctx context.Context, n *environmentSync) {
	if !n.following {
		return
	}
	c, w, err := b.readEnvironment(ctx)
	if err == nil {
		// Do not overwrite a setting somebody explicitly changed after our last
		// application. Restore policies, not an old hour from before the session.
		if w.Policy == n.lastWeather && weatherPolicyValid(n.originalWeather) {
			_ = b.command(ctx, "weather "+n.originalWeather, nil)
		}
		if c.Held == n.lastFrozen && c.Held != n.originalFrozen && (!n.originalFrozen || c.Hour != nil) {
			cmd := "time unfreeze"
			if n.originalFrozen {
				cmd = "time freeze"
			}
			_ = b.command(ctx, cmd, nil)
		}
	}
	n.following = false
}
func clockDistance(a, b float64) float64 { d := math.Abs(a - b); return math.Min(d, 24-d) }
func (b *Bridge) updateEnvironment(ctx context.Context, n *environmentSync, now time.Time) error {
	if now.Sub(n.last) < time.Second {
		return nil
	}
	n.last = now
	v := b.Client.Environment()
	if v.HostID != "" && v.HostID != n.announcedHost {
		n.announcedHost = v.HostID
		if v.HostID == b.Client.Welcome().ID {
			b.say("Você é o anfitrião de horário e clima")
		} else {
			b.say("Anfitrião de horário e clima: " + v.HostName)
		}
	}
	if v.Epoch != n.epoch {
		b.restoreEnvironment(ctx, n)
		n.epoch = v.Epoch
		n.sequence = 0
		n.hostInitialized = false
	}
	if v.HostID == "" {
		b.restoreEnvironment(ctx, n)
		return nil
	}
	if v.HostID == b.Client.Welcome().ID {
		b.restoreEnvironment(ctx, n)
		c, w, err := b.readEnvironment(ctx)
		if err != nil {
			return err
		}
		if c.Hour == nil || !weatherValid(w.Now) {
			return nil
		}
		// Set the current hour once without changing it or the freeze policy.
		// This also releases the sandbox pre-paraglider clock gate.
		if !n.hostInitialized {
			if err := b.command(ctx, fmt.Sprintf("time set %.6f", *c.Hour), nil); err != nil {
				return err
			}
			n.hostInitialized = true
		}
		var status struct {
			Paused bool `json:"game_paused"`
		}
		if err = b.command(ctx, "hm_native_status", &status); err != nil {
			return err
		}
		n.sequence++
		h := math.Mod(*c.Hour, 24)
		return b.Client.PublishEnvironment(Environment{Epoch: v.Epoch, Sequence: n.sequence, Hour: h, Frozen: c.Held || status.Paused, Weather: w.Now})
	}
	if v.State == nil || now.Sub(v.At) > 3*time.Second {
		b.restoreEnvironment(ctx, n)
		return nil
	}
	e := v.State
	c, w, err := b.readEnvironment(ctx)
	if err != nil {
		return err
	}
	if c.Hour == nil || !weatherPolicyValid(w.Policy) {
		return nil
	}
	if !n.following {
		n.originalFrozen = c.Held
		n.originalWeather = w.Policy
		n.lastFrozen = c.Held
		n.lastWeather = w.Policy
		n.following = true
		b.say("Horário e clima seguindo o anfitrião")
	}
	if c.Held && !e.Frozen {
		if err = b.command(ctx, "time unfreeze", nil); err != nil {
			return err
		}
		n.lastFrozen = false
	}
	if clockDistance(*c.Hour, e.Hour) > .015 || c.Held != e.Frozen {
		if err = b.command(ctx, fmt.Sprintf("time set %.6f", e.Hour), nil); err != nil {
			return err
		}
	}
	if !c.Held && e.Frozen {
		if err = b.command(ctx, "time freeze", nil); err != nil {
			return err
		}
		n.lastFrozen = true
	}
	if w.Policy != e.Weather {
		if err = b.command(ctx, "weather "+e.Weather, nil); err != nil {
			return err
		}
		n.lastWeather = e.Weather
	}
	return nil
}
