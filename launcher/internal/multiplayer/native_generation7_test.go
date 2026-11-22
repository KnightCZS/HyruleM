package multiplayer

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func testBone(name uint32, x float32) PoseBone {
	return PoseBone{Name: name, Matrix: [12]float32{1, 0, 0, x, 0, 1, 0, 0, 0, 0, 1, 0}}
}

func TestFinalSkeletonTransportAndLimits(t *testing.T) {
	bones := make([]PoseBone, MaxPoseBones)
	for i := range bones {
		bones[i] = testBone(uint32(i+1), float32(i%100))
	}
	encoded := encodeSkeleton(bones)
	decoded, err := decodeSkeleton(encoded)
	if err != nil || len(decoded) != MaxPoseBones || decoded[400] != bones[400] {
		t.Fatal("skeletal roundtrip", err)
	}
	p := Pose{Ready: true, Skeleton: encoded}
	var wire bytes.Buffer
	if err := WriteMessage(&wire, Message{Type: "snapshot", Players: []Player{{ID: "a", Pose: p}, {ID: "b", Pose: p}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadMessage(bufio.NewReaderSize(fragmentedReader{&wire}, MaxLine)); err != nil {
		t.Fatal(err)
	}
	cmd, err := nativePoseCommand("0123456789abcdef", 1, p)
	if err != nil || len(cmd) >= 65536 || !strings.HasSuffix(cmd, encoded) {
		t.Fatal("bounded native frame", len(cmd), err)
	}
	for _, mutate := range []func([]PoseBone){
		func(b []PoseBone) { b[1].Name = b[0].Name },
		func(b []PoseBone) { b[0].Matrix[0] = float32(math.NaN()) },
		func(b []PoseBone) { b[0].Matrix[3] = 129 },
		func(b []PoseBone) { b[0].Matrix[0] = 0 },
	} {
		bad := append([]PoseBone(nil), bones...)
		mutate(bad)
		if _, err := decodeSkeleton(encodeSkeleton(bad)); err == nil {
			t.Fatal("invalid bone accepted")
		}
	}
	raw, _ := base64.StdEncoding.DecodeString(encoded)
	raw[6] = 1
	if _, err := decodeSkeleton(base64.StdEncoding.EncodeToString(raw)); err == nil {
		t.Fatal("unknown format flags accepted")
	}
}

func TestSkeletonAndAttachmentShareInterpolation(t *testing.T) {
	a, b := testBone(1, 1), testBone(1, 5)
	got, snap := interpolateSkeleton(encodeSkeleton([]PoseBone{a}), encodeSkeleton([]PoseBone{b}), .25)
	pose, err := decodeSkeleton(got)
	if err != nil || snap || pose[0].Matrix[3] != 2 {
		t.Fatal(pose, snap, err)
	}
	ea := Equipment{Slot: 0, Name: "Weapon_Sword_001"}
	for j := range a.Matrix {
		ea.Matrix[j] = float64(a.Matrix[j])
	}
	eb := Equipment{Slot: 0, Name: ea.Name}
	for j := range b.Matrix {
		eb.Matrix[j] = float64(b.Matrix[j])
	}
	gear := interpolateEquipmentFrame([]Equipment{ea}, []Equipment{eb}, .25, true)
	if len(gear) != 1 || gear[0].Matrix[3] != float64(pose[0].Matrix[3]) {
		t.Fatal("weapon and bone use different clocks", gear, pose)
	}
	b.Matrix[0] = -1
	b.Matrix[10] = -1
	_, snap = interpolateSkeleton(encodeSkeleton([]PoseBone{a}), encodeSkeleton([]PoseBone{b}), .5)
	if !snap {
		t.Fatal("half-turn collapsed into a singular skeleton")
	}
}

func TestBidirectionalDamageIsAdditiveAndIdempotent(t *testing.T) {
	s := testServer(t)
	a, b := testClient(t, s, "Host"), testClient(t, s, "Guest")
	p := Pose{Ready: true, Position: Vec3{3430, 236, 2100}}
	_ = a.Publish(p)
	_ = b.Publish(p)
	e := EnemyHealth{Hash: 37, Actor: "Obj_TreeApple_A_01", Spawn: p.Position, Position: p.Position, Health: 100}
	_ = a.PublishMapDamage("0123456789abcdef", []EnemyHealth{e})
	eventually(t, func() bool { return len(b.SharedEnemyHealth()) == 1 })
	e.Health = 60
	e.Damage = 40
	_ = a.PublishMapDamage("0123456789abcdef", []EnemyHealth{e})
	e.Health = 70
	e.Damage = 30
	_ = b.PublishMapDamage("abcdef0123456789", []EnemyHealth{e})
	eventually(t, func() bool { v := a.SharedEnemyHealth(); return len(v) == 1 && v[0].Health == 30 })
	for i := 0; i < 3; i++ {
		_ = b.PublishMapDamage("abcdef0123456789", []EnemyHealth{e})
	}
	e.Health = 0
	e.Damage = 60
	_ = b.PublishMapDamage("abcdef0123456789", []EnemyHealth{e})
	eventually(t, func() bool { v := a.SharedEnemyHealth(); return len(v) == 1 && v[0].Health == 0 })
	reg := map[string]EnemyHealth{}
	ledger := map[string]uint64{}
	if err := mergeMapDamage(reg, ledger, "0123456789abcdef", []EnemyHealth{e}, p); err != nil {
		t.Fatal(err)
	}
	e.Damage = 1
	if err := mergeMapDamage(reg, ledger, "0123456789abcdef", []EnemyHealth{e}, p); err == nil {
		t.Fatal("counter rewind accepted")
	}
	for _, name := range []string{"GameROMPlayer", "Obj_TreeApple_A_01_Stump", "Obj_TreeBroadleaf_A_L_Trunk"} {
		e.Actor = name
		if e.Valid() {
			t.Fatal("unsafe actor", name)
		}
	}
}

func TestOrdinaryWeaponTransferConditionAndDestruction(t *testing.T) {
	for _, dedicated := range []bool{false, true} {
		t.Run(fmt.Sprint("dedicated=", dedicated), func(t *testing.T) {
			s, err := Listen("127.0.0.1:0", ServerConfig{DedicatedWorld: dedicated, SharedItems: true, Spawn: Vec3{3437.5, 235.9, 2101.5}, Tick: 20 * time.Millisecond})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			a, b := testClient(t, s, "Host"), testClient(t, s, "Guest")
			p := Pose{Ready: true, Scene: "MainField", Position: s.cfg.Spawn}
			_ = a.Publish(p)
			_ = b.Publish(p)
			g := GroundSource{Scene: "MainField", Local: 0x200000001, NativeDrop: true, Drop: true, Origin: "0123456789abcdef", Actor: "Weapon_Sword_001", Value: 900, Modifier: 8, ModifierValue: 7, Position: p.Position, Spawn: p.Position}
			if err := b.PublishGround([]GroundSource{g}); err != nil {
				t.Fatal(err)
			}
			eventually(t, func() bool { return len(a.Ground().Items) == 1 })
			item := a.Ground().Items[0]
			if item.Source != g || item.Owner != b.Welcome().ID {
				t.Fatal("guest weapon lost metadata", item)
			}
			forged := g
			forged.Value++
			_ = a.ClaimGround(GroundClaim{Request: 1, Item: item.ID, Source: forged})
			eventually(t, func() bool { return len(a.Ground().Claims) == 1 })
			if a.Ground().Claims[0].Status != "denied" {
				t.Fatal("durability was invented")
			}
			_ = a.ClaimGround(GroundClaim{Request: 2, Item: item.ID, Source: g})
			eventually(t, func() bool { return len(a.Ground().Claims) == 2 && a.Ground().Claims[1].Status == "granted" })
			_ = b.DestroyGround([]uint64{item.ID}) // reserved pickup wins over late break
			_ = a.ReceiptGround(GroundReceipt{Request: 2, Item: item.ID, Applied: true})
			eventually(t, func() bool { return b.Ground().Items[0].State == "taken" })
			g.Local++
			_ = b.PublishGround([]GroundSource{g})
			eventually(t, func() bool { return len(a.Ground().Items) == 2 })
			_ = a.DestroyGround([]uint64{2}) // only its owner may destroy a weapon
			_ = b.DestroyGround([]uint64{2})
			eventually(t, func() bool { return a.Ground().Items[1].State == "cancelled" })
		})
	}
}
func TestNativeDropCapacityRejectionPreservesLocalIdentity(t *testing.T) {
	s := groundServer(t)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	p := &serverPeer{player: Player{ID: "owner", Pose: Pose{Ready: true, Position: s.cfg.Spawn}}, poseAt: now, groundClaims: map[uint64]GroundClaim{}}
	s.environmentHost = "owner"
	for i := uint64(1); i <= MaxGroundItems; i++ {
		g := GroundSource{Local: i, Hash: uint32(i), Actor: "Item_Fruit_A", Spawn: s.cfg.Spawn, Position: s.cfg.Spawn}
		if err := s.observeGround(p, []GroundSource{g}, now); err != nil {
			t.Fatal(err)
		}
	}
	g := GroundSource{Local: 0x200000001, NativeDrop: true, Drop: true, Origin: "0123456789abcdef", Actor: "Weapon_Sword_001", Value: 12, Position: s.cfg.Spawn, Spawn: s.cfg.Spawn}
	if err := s.observeGround(p, []GroundSource{g}, now); err != nil {
		t.Fatal(err)
	}
	if !p.groundRejected[g.Local] || len(s.groundItems) != MaxGroundItems {
		t.Fatal("drop silently enrolled beyond capacity")
	}
	if err := s.claimGround(p, GroundClaim{Request: 1, Source: g}, now); err != nil {
		t.Fatal(err)
	}
	if p.groundClaims[1].Status != "denied" {
		t.Fatal("rejected weapon can remain granted or waiting")
	}
}
