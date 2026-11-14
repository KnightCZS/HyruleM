package multiplayer

import (
	"strings"
	"testing"
	"time"
)

func TestTwoClientsPreserveRepeatedAttackAndMovementState(t *testing.T) {
	s := testServer(t)
	a := testClient(t, s, "A")
	b := testClient(t, s, "B")
	p := Pose{Ready: true, Position: Vec3{X: 10}, Upper: AnimationTrack{"CutS1", 7, true}, Lower: AnimationTrack{"Wait", 3, false}}
	if err := a.Publish(p); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { r := b.Remote(time.Now()); return len(r) == 1 && r[0].Pose.Upper == p.Upper })
	p.Position.X = 15
	p.Upper.Sequence = 8
	p.Lower = AnimationTrack{"Move", 4, false}
	if err := a.Publish(p); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		r := b.Remote(time.Now())
		return len(r) == 1 && r[0].Pose.Upper.Sequence == 8 && r[0].Pose.Lower.Name == "Move" && r[0].Pose.Position.X > 10
	})
	command, err := nativePoseCommand("0123456789abcdef", 5, p)
	if err != nil || !strings.Contains(command, "CutS1 8 1 Move 4 0") {
		t.Fatal("action event lost in native command", command, err)
	}
	p.Upper.Name = "CutS1\nhm_native_clear"
	if a.Publish(p) == nil {
		t.Fatal("invalid animation accepted")
	}
}
