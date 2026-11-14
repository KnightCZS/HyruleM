package nativeassets

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	f "hyrulelauncher/internal/gamefmt"
	"strings"
	"testing"
)

func smallModel() []byte {
	d := make([]byte, 0x60)
	e := binary.LittleEndian
	copy(d, "AAMP")
	for at, v := range map[int]uint32{4: 2, 8: 3, 12: 0x60, 20: 4, 24: 1, 28: 1, 32: 2, 40: 8} {
		e.PutUint32(d[at:], v)
	}
	e.PutUint16(d[0x3c:], 3)
	e.PutUint16(d[0x3e:], 1)
	e.PutUint32(d[0x40:], crc32.ChecksumIEEE([]byte("Base")))
	e.PutUint16(d[0x44:], 2)
	e.PutUint16(d[0x46:], 2)
	e.PutUint32(d[0x48:], crc32.ChecksumIEEE([]byte("Folder")))
	e.PutUint32(d[0x4c:], 4|8<<24)
	e.PutUint32(d[0x50:], crc32.ChecksumIEEE([]byte("UnitName")))
	e.PutUint32(d[0x54:], 2|8<<24)
	copy(d[0x58:], "Link")
	return d
}
func TestEquipmentUsesMetadataAndVisualOnlyControllers(t *testing.T) {
	model := smallModel()
	raw, _ := f.WriteSARC(map[string][]byte{"Actor/ModelList/Player_Link.bmodellist": model})
	player, _ := f.ReadSARC(raw)
	link, err := f.PatchAAMPStrings(syntheticLink(), "LinkTarget", map[string]string{"ProfileUser": "DemoNPC", "ASUser": Actor, "AIProgramUser": "Dummy", "PhysicsUser": "Dummy", "DamageParamUser": "Dummy"})
	if err != nil {
		t.Fatal(err)
	}
	row := f.Dict(map[string]*f.Node{"name": f.Text(Actor), "profile": f.Text("DemoNPC"), "bfres": f.Text("Link"), "mainModel": f.Text("Link")})
	armor := f.Dict(map[string]*f.Node{"name": f.Text("Armor_001_Head"), "bfres": f.Text("Armor_001"), "mainModel": f.Text("Armor_001_Head_A")})
	weapon := f.Dict(map[string]*f.Node{"name": f.Text("Weapon_Sword_001"), "bfres": f.Text("Weapon_Sword_001"), "mainModel": f.Text("Weapon_Sword_001")})
	shared := map[string][]byte{"Actor/AS/HM_Test.bas": []byte("synthetic animation")}
	before := append([]byte(nil), link...)
	packs, rows, err := equipmentResources(player, model, shared, link, []*f.Node{armor, weapon}, row)
	if err != nil || len(packs) != 2 || len(rows) != 2 {
		t.Fatal(err, len(packs), len(rows))
	}
	if !bytes.Equal(link, before) {
		t.Fatal("source controller changed")
	}
	for _, r := range rows {
		name := r.Get("name").Str()
		if r.Get("profile").Str() != "DemoNPC" {
			t.Fatal("gameplay profile used")
		}
		data, _, err := f.Unpack(packs["Actor/Pack/"+name+".sbactorpack"])
		if err != nil {
			t.Fatal(err)
		}
		s, err := f.ReadSARC(data)
		if err != nil {
			t.Fatal(err)
		}
		m, ok := s.Get("Actor/ModelList/" + name + ".bmodellist")
		if !ok {
			t.Fatal("model missing")
		}
		e := binary.LittleEndian
		for at, want := range map[int]string{0x48: r.Get("bfres").Str(), 0x50: r.Get("mainModel").Str()} {
			p := at + int(e.Uint32(m[at+4:])&0xffffff)*4
			n := bytes.IndexByte(m[p:], 0)
			if n < 0 || string(m[p:p+n]) != want {
				t.Fatal("metadata not applied")
			}
		}
		_, hasAS := s.Get("Actor/AS/HM_Test.bas")
		if hasAS == strings.Contains(name, "Weapon_") {
			t.Fatal("rigid weapons should not load player AS")
		}
	}
	table := &f.RSTB{Hashes: map[uint32]uint32{}, Names: map[string]uint32{}}
	if err = reserveEquipment(table, packs, len(model)); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if resourceBudget(table, "Actor/ModelList/"+r.Get("name").Str()+".bmodellist") == 0 {
			t.Fatal("equipment budget missing")
		}
	}
}
