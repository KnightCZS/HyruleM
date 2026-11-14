package multiplayer

import (
	"math"
	"strings"
)

// Equipment is presentation metadata only. It never grants an item, changes
// durability or writes another player's inventory/health.
type Equipment struct {
	Slot   int         `json:"slot"`
	Name   string      `json:"name"`
	Matrix [12]float64 `json:"matrix"`
}

func equipmentName(name string, slot int) bool {
	if len(name) >= 48 {
		return false
	}
	for _, r := range name {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
			return false
		}
	}
	if slot < 3 {
		switch slot {
		case 0:
			return strings.HasPrefix(name, "Weapon_Sword_") || strings.HasPrefix(name, "Weapon_Lsword_") || strings.HasPrefix(name, "Weapon_Spear_")
		case 1:
			return strings.HasPrefix(name, "Weapon_Bow_")
		case 2:
			return strings.HasPrefix(name, "Weapon_Shield_")
		}
	}
	if slot < 3 || slot > 5 {
		return false
	}
	suffix := []string{"_Head", "_Upper", "_Lower"}[slot-3]
	return strings.HasPrefix(name, "Armor_") && strings.HasSuffix(name, suffix)
}
func (e Equipment) Valid() bool {
	if e.Slot < 0 || e.Slot > 5 || e.Name != "" && !equipmentName(e.Name, e.Slot) {
		return false
	}
	for i, x := range e.Matrix {
		limit := 8.
		if i%4 == 3 {
			limit = 32
		}
		if !finite(x) || math.Abs(x) > limit {
			return false
		}
	}
	return true
}

func interpolateEquipment(a, b []Equipment, t float64) []Equipment {
	out := append([]Equipment(nil), b...)
	for i, e := range out {
		for _, old := range a {
			if e.Slot != old.Slot {
				continue
			}
			if e.Name != old.Name {
				if t < 1 {
					out[i] = old
				}
				break
			}
			for j := range e.Matrix {
				e.Matrix[j] = old.Matrix[j]*(1-t) + e.Matrix[j]*t
			}
			// Interpolating coefficients can squash a rotating weapon. Rebuild an
			// orthogonal basis; no extra equipment actors or inventory writes.
			x := Vec3{e.Matrix[0], e.Matrix[4], e.Matrix[8]}.Unit()
			y := Vec3{e.Matrix[1], e.Matrix[5], e.Matrix[9]}
			y = y.Sub(x.Mul(x.Dot(y))).Unit()
			z := x.Cross(y)
			if x.Len() > .9 && y.Len() > .9 {
				e.Matrix[0], e.Matrix[4], e.Matrix[8] = x.X, x.Y, x.Z
				e.Matrix[1], e.Matrix[5], e.Matrix[9] = y.X, y.Y, y.Z
				e.Matrix[2], e.Matrix[6], e.Matrix[10] = z.X, z.Y, z.Z
			}
			out[i] = e
			break
		}
	}
	return out
}
