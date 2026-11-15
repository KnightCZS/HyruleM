package multiplayer

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"testing"
	"time"
)

func testAppearance(shapes, props int) string {
	raw := make([]byte, 8+16*(shapes+props))
	copy(raw, "HMA1")
	binary.LittleEndian.PutUint16(raw[4:], uint16(shapes))
	binary.LittleEndian.PutUint16(raw[6:], uint16(props))
	for i := 0; i < shapes; i++ {
		p := raw[8+16*i:]
		binary.LittleEndian.PutUint32(p[4:], uint32(i+1))
		binary.LittleEndian.PutUint32(p[8:], uint32(i%2))
	}
	for i := 0; i < props; i++ {
		p := raw[8+16*(shapes+i):]
		binary.LittleEndian.PutUint32(p, uint32(i/64))
		binary.LittleEndian.PutUint32(p[4:], uint32(i%64+1))
		binary.LittleEndian.PutUint32(p[8:], uint32(i%4))
		binary.LittleEndian.PutUint32(p[12:], 0x3f000000)
	}
	return base64.StdEncoding.EncodeToString(raw)
}
func TestAppearanceValidationAndCommandLimit(t *testing.T) {
	app := testAppearance(MaxVisibilityEntries, MaxEffectProperties)
	if !validAppearance(app) {
		t.Fatal("valid maximum frame rejected")
	}
	for _, mutate := range []func([]byte){
		func(b []byte) { b[0] = 'X' }, func(b []byte) { b[8] = 3 }, func(b []byte) { b[20] = 2 }, func(b []byte) { copy(b[24:40], b[8:24]) },
		func(b []byte) { binary.LittleEndian.PutUint32(b[8+16*MaxVisibilityEntries:], 4) },
		func(b []byte) {
			p := b[8+16*MaxVisibilityEntries:]
			binary.LittleEndian.PutUint32(p[8:], 2)
			binary.LittleEndian.PutUint32(p[12:], 0x7fc00000)
		},
	} {
		raw, _ := base64.StdEncoding.DecodeString(app)
		mutate(raw)
		if validAppearance(base64.StdEncoding.EncodeToString(raw)) {
			t.Fatal("invalid appearance accepted")
		}
	}
	if validAppearance(app + "\n") { // Strict base64 ignores newlines: the token itself must exclude them.
		t.Fatal("control separator accepted")
	}
	bones := make([]PoseBone, MaxPoseBones)
	for i := range bones {
		bones[i] = testBone(uint32(i+1), 1)
	}
	effects := make([]VisualEffect, MaxVisualEffects)
	for i := range effects {
		effects[i] = VisualEffect{Sequence: uint64(i + 1), Slot: i % 4, Name: strings.Repeat("A", 63), Active: true, AgeMS: 5000}
	}
	p := Pose{Ready: true, Skeleton: encodeSkeleton(bones), Appearance: app, Effects: effects}
	cmd, err := nativePoseCommand("0123456789abcdef", 1, p)
	if err != nil || len(cmd) > 65535 || !strings.HasSuffix(cmd, " "+app) {
		t.Fatal("full visual frame", len(cmd), err)
	}
	t.Logf("maximum skeletal/appearance/effect frame: %d bytes", len(cmd))
}
func TestAppearanceUsesSameDiscreteFrameAsEffects(t *testing.T) {
	start := time.Unix(100, 0)
	a := Pose{Ready: true, Skeleton: encodeSkeleton([]PoseBone{testBone(1, 0)}), Appearance: testAppearance(1, 1)}
	b := a
	b.Skeleton = encodeSkeleton([]PoseBone{testBone(1, 1)})
	b.Appearance = testAppearance(2, 2)
	c := &Client{connected: true, frames: []Frame{{At: start, Players: []Player{{ID: "other", Pose: a}}}, {At: start.Add(100 * time.Millisecond), Players: []Player{{ID: "other", Pose: b}}}}}
	if r := c.Remote(start.Add(150 * time.Millisecond)); len(r) != 1 || r[0].Pose.Appearance != a.Appearance {
		t.Fatal("appearance ran ahead of effects")
	}
	if r := c.Remote(start.Add(200 * time.Millisecond)); len(r) != 1 || r[0].Pose.Appearance != b.Appearance {
		t.Fatal("appearance update lost")
	}
}
