package nativeassets

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	f "hyrulelauncher/internal/gamefmt"
	"testing"
)

func syntheticLink() []byte {
	keys := []string{"ProfileUser", "ASUser", "AIProgramUser", "AIScheduleUser", "AttentionUser", "AwarenessUser", "BoneControlUser", "ChemicalUser", "ElinkUser", "SlinkUser", "XlinkUser", "PhysicsUser", "DamageParamUser", "RgBlendWeightUser", "RgConfigListUser", "ModelUser", "GParamUser", "AnimationInfo"}
	d := make([]byte, 0x44+len(keys)*8+8)
	e := binary.LittleEndian
	copy(d, "AAMP")
	e.PutUint32(d[4:], 2)
	e.PutUint32(d[8:], 3)
	e.PutUint32(d[12:], uint32(len(d)))
	e.PutUint32(d[24:], 1)
	e.PutUint32(d[28:], 1)
	e.PutUint32(d[32:], uint32(len(keys)))
	e.PutUint32(d[40:], 8)
	e.PutUint16(d[0x38:], 3)
	e.PutUint16(d[0x3a:], 1)
	e.PutUint32(d[0x3c:], crc32.ChecksumIEEE([]byte("LinkTarget")))
	e.PutUint16(d[0x40:], 2)
	e.PutUint16(d[0x42:], uint16(len(keys)))
	str := len(d) - 8
	copy(d[str:], "Player")
	for i, key := range keys {
		at := 0x44 + i*8
		e.PutUint32(d[at:], crc32.ChecksumIEEE([]byte(key)))
		e.PutUint32(d[at+4:], uint32((str-at)/4)|20<<24)
	}
	return d
}
func TestPrivateRemoteResourcesPreserveOriginals(t *testing.T) {
	originalLink := syntheticLink()
	pack, _ := f.WriteSARC(map[string][]byte{"Actor/ActorLink/GameROMPlayer.bxml": originalLink, "Actor/Other/Keep.bin": []byte("synthetic animation")})
	title, _ := f.WriteSARC(map[string][]byte{"Actor/Pack/GameROMPlayer.sbactorpack": f.PackYaz0(pack)})
	original := f.Dict(map[string]*f.Node{"name": f.Text("GameROMPlayer"), "profile": f.Text("Player"), "bfres": f.Text("Link"), "mainModel": f.Text("Link")})
	info, _ := f.WriteBYML(f.Document{Little: true, Version: 2, Root: f.Dict(map[string]*f.Node{"Actors": f.Arr(original), "Hashes": f.Arr(&f.Node{Kind: f.Uint, Bits: uint64(crc32.ChecksumIEEE([]byte("GameROMPlayer")))})})})
	table := (&f.RSTB{Hashes: map[uint32]uint32{}, Names: map[string]uint32{}}).Bytes()
	before := append([]byte(nil), title...)
	out, err := BuildRemote(title, f.PackYaz0(info), f.PackYaz0(table))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(title, before) {
		t.Fatal("source archive changed")
	}
	p, _, err := f.Unpack(out[PackPath])
	if err != nil {
		t.Fatal(err)
	}
	archive, err := f.ReadSARC(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := archive.Get("Actor/ActorLink/GameROMPlayer.bxml"); ok {
		t.Fatal("remote pack still names the original actor link")
	}
	legacyData, _, err := f.Unpack(out[FallbackPackPath])
	if err != nil {
		t.Fatal(err)
	}
	legacyArchive, err := f.ReadSARC(legacyData)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := legacyArchive.Get("Actor/ActorLink/" + FallbackActor + ".bxml"); !ok {
		t.Fatal("fallback actor link missing")
	}
	if keep, ok := archive.Get("Actor/Other/Keep.bin"); !ok || string(keep) != "synthetic animation" {
		t.Fatal("visual resource changed")
	}
	b, _, _ := f.Unpack(out[InfoPath])
	doc, err := f.ReadBYML(b)
	if err != nil {
		t.Fatal(err)
	}
	actors := doc.Root.Get("Actors").List
	if len(actors) != 3 {
		t.Fatal("wrong actor count")
	}
	for _, a := range actors {
		if a.Get("name").Str() == "GameROMPlayer" && a.Get("profile").Str() != "Player" {
			t.Fatal("local player changed")
		}
		if a.Get("name").Str() == Actor && a.Get("profile").Str() != "DemoNPC" {
			t.Fatal("remote actor controls the player")
		}
	}
	raw, _, _ := f.Unpack(out[RSTBPath])
	r, err := f.ReadRSTB(raw)
	if err != nil {
		t.Fatal(err)
	}
	if r.Hashes[crc32.ChecksumIEEE([]byte(f.Canonical(PackPath)))] < uint32(len(p)) {
		t.Fatal("remote resource budget missing")
	}
}
