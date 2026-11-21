package multiplayer

import (
	"testing"
)

func TestRoamingWeaponWhileHostIsAcrossHyrule(t *testing.T) {
	s := groundServer(t)
	a, b := testClient(t, s, "Rito"), testClient(t, s, "Desert")
	rito := Pose{Ready: true, Position: Vec3{-3607, 286, -1805}}
	desert := Pose{Ready: true, Position: Vec3{-3198, 130, 2544}}
	a.Publish(rito)
	b.Publish(desert)
	eventually(t, func() bool {
		return b.Ground().Roaming && b.Ground().Zone == desert.Position && a.Ground().Zone == rito.Position
	})
	g := GroundSource{Local: 1, Hash: 99, Actor: "Weapon_Sword_001", Value: 17, Modifier: 1, ModifierValue: 3, Spawn: desert.Position, Position: desert.Position}
	if err := b.PublishGround([]GroundSource{g}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(a.Ground().Items) == 1 })
	a.ClaimGround(GroundClaim{Request: 1, Source: g})
	eventually(t, func() bool { return len(a.Ground().Claims) == 1 && a.Ground().Claims[0].Status == "denied" })
	a.Publish(desert)
	eventually(t, func() bool { return a.Ground().Zone == desert.Position })
	a.ClaimGround(GroundClaim{Request: 2, Source: g})
	b.ClaimGround(GroundClaim{Request: 1, Source: g})
	eventually(t, func() bool { return len(a.Ground().Claims) == 2 && len(b.Ground().Claims) == 1 })
	ca, cb := a.Ground().Claims[1], b.Ground().Claims[0]
	winner, claim := a, ca
	if ca.Status != "granted" {
		winner, claim = b, cb
	}
	if claim.Status != "granted" || ca.Status == cb.Status {
		t.Fatalf("reservation: %+v %+v", ca, cb)
	}
	receipt := GroundReceipt{Item: claim.Item, Request: claim.Request, Applied: true}
	winner.ReceiptGround(receipt)
	winner.ReceiptGround(receipt)
	eventually(t, func() bool { return a.Ground().Items[0].State == "taken" && b.Ground().Items[0].State == "taken" })
}

func TestMapDeathSurvivesTravelAndMoreThanOneWireWindow(t *testing.T) {
	s := testServer(t)
	a, b := testClient(t, s, "One"), testClient(t, s, "Two")
	var first EnemyHealth
	for region := 0; region < 4; region++ {
		p := Pose{Ready: true, Position: Vec3{X: float64(region * 1000)}}
		a.Publish(p)
		b.Publish(p)
		rows := make([]EnemyHealth, 64)
		for i := range rows {
			rows[i] = EnemyHealth{Hash: uint32(1 + 64*region + i), Actor: "Enemy_Bokoblin_Junior", Spawn: p.Position, Position: p.Position, Health: 100}
		}
		if region == 0 {
			rows[0].Actor = "Obj_TreeApple_A_01"
			first = rows[0]
		}
		a.PublishMapDamage("0123456789abcdef", rows)
		eventually(t, func() bool {
			for _, v := range b.SharedEnemyHealth() {
				if v.Hash == rows[63].Hash {
					return true
				}
			}
			return false
		})
		if region == 0 {
			dead := first
			dead.Health = 0
			a.PublishMapDamage("0123456789abcdef", []EnemyHealth{dead})
			eventually(t, func() bool { v := b.SharedEnemyHealth(); return len(v) == 1 && v[0].Health == 0 })
		}
	}
	b.Publish(Pose{Ready: true, Position: first.Position})
	b.PublishMapDamage("abcdef0123456789", []EnemyHealth{first})
	eventually(t, func() bool {
		v := b.SharedEnemyHealth()
		for _, e := range v {
			if e.Hash == first.Hash {
				return e.Health == 0 && len(v) <= MaxSharedEnemies
			}
		}
		return false
	})
}
