package multiplayer

import "math"

const MaxVisualEffects = 16

// Only resource keys and bounded lifecycle state cross the network. A key is
// resolved by the receiver's own ELink user, never a pointer or resource file.
type VisualEffect struct {
	Sequence uint64 `json:"sequence"`
	Slot     int    `json:"slot"` // 0 body, 1 sword, 2 bow, 3 shield
	Active   bool   `json:"active"`
	AgeMS    uint32 `json:"age_ms"`
	Name     string `json:"name"`
}

func validEffects(effects []VisualEffect) bool {
	if len(effects) > MaxVisualEffects {
		return false
	}
	seen := map[uint64]bool{}
	for _, e := range effects {
		if e.Sequence == 0 || e.Sequence > math.MaxInt64 || seen[e.Sequence] || e.Slot < 0 || e.Slot > 3 || e.AgeMS > 5000 || e.Name == "" || !((AnimationTrack{Name: e.Name}).Valid()) {
			return false
		}
		seen[e.Sequence] = true
	}
	return true
}
