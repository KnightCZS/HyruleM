package gamefmt

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func testDoc() Document {
	return Document{Little: true, Version: 2, Root: Dict(map[string]*Node{
		"Name": Text("Hateno · João"), "Steps": Arr(Text("Ready"), Text("Finish")), "Active": Boolean(false), "Unsigned": {Kind: Uint, Bits: 0xffffffff}, "Float": {Kind: Float, Bits: uint64(math.Float32bits(1.25))}, "Long": {Kind: 0xd4, Bits: 0xffffffffffffffff}, "Empty": {Kind: Null},
	})}
}
func TestBYMLTypedValuesAndByteOrders(t *testing.T) {
	for _, little := range []bool{true, false} {
		doc := testDoc()
		doc.Little = little
		b, e := WriteBYML(doc)
		if e != nil {
			t.Fatal(e)
		}
		got, e := ReadBYML(b)
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(doc, got) {
			t.Fatalf("typed round trip failed: %#v", got)
		}
	}
}
func TestBYMLRejectsCorruption(t *testing.T) {
	b, _ := WriteBYML(testDoc())
	for _, mutate := range []func([]byte) []byte{
		func(d []byte) []byte { return d[:12] }, func(d []byte) []byte { d[0] = 'X'; return d }, func(d []byte) []byte { binary.LittleEndian.PutUint32(d[12:], 0xffffff00); return d }, func(d []byte) []byte { o := binary.LittleEndian.Uint32(d[12:]); d[o] = 0x11; return d },
	} {
		d := mutate(append([]byte(nil), b...))
		if _, e := ReadBYML(d); e == nil {
			t.Fatal("accepted malformed BYML")
		}
	}
	cyclic := Arr()
	cyclic.List = append(cyclic.List, cyclic)
	if _, e := WriteBYML(Document{Root: cyclic, Little: true, Version: 2}); e == nil {
		t.Fatal("accepted node cycle")
	}
}
func TestYaz0BackReferencesAndBounds(t *testing.T) {
	for _, raw := range [][]byte{nil, []byte("abc"), bytes.Repeat([]byte("abacaba"), 2000), bytes.Repeat([]byte{0}, 20000)} {
		z := PackYaz0(raw)
		d, compressed, e := Unpack(z)
		if e != nil || !compressed || !bytes.Equal(d, raw) {
			t.Fatal("Yaz0 mismatch", e)
		}
	}
	for _, bad := range [][]byte{[]byte("Yaz0"), append([]byte("Yaz0\xff\xff\xff\xff"), make([]byte, 8)...), append([]byte("Yaz0\x00\x00\x00\x05"), make([]byte, 12)...)} {
		if _, _, e := Unpack(bad); e == nil {
			t.Fatal("accepted corrupt Yaz0")
		}
	}
}
func TestSARCReplacementAndRebuiltAlignment(t *testing.T) {
	original, e := WriteSARC(map[string][]byte{"EventFlow/Test.bfevfl": []byte("original"), "Other/Keep": []byte("unaltered")})
	if e != nil {
		t.Fatal(e)
	}
	s, e := ReadSARC(original)
	if e != nil {
		t.Fatal(e)
	}
	big := bytes.Repeat([]byte("new"), 3000)
	if e = s.Replace("EventFlow/Test.bfevfl", big); e != nil {
		t.Fatal(e)
	}
	again, e := ReadSARC(s.Data)
	if e != nil {
		t.Fatal(e)
	}
	got, _ := again.Get("EventFlow/Test.bfevfl")
	if !bytes.Equal(got, big) {
		t.Fatal("replacement not stored")
	}
	keep, _ := again.Get("Other/Keep")
	if string(keep) != "unaltered" {
		t.Fatal("unrelated payload changed")
	}
	prior, _ := ReadSARC(original)
	old, _ := prior.Get("EventFlow/Test.bfevfl")
	if string(old) != "original" {
		t.Fatal("input mutated")
	}
	for _, entry := range again.Entries {
		if entry.Start%4096 != 0 {
			t.Fatal("alignment lost")
		}
	}
}
func TestRSTBCanonicalBudgetsAndUnknownEntries(t *testing.T) {
	r := &RSTB{Hashes: map[uint32]uint32{123: 456}, Names: map[string]uint32{"collision/path": 999999}}
	if e := r.Ensure("Quest/QuestProduct.sbquestpack", 5000); e != nil {
		t.Fatal(e)
	}
	b := r.Bytes()
	got, e := ReadRSTB(b)
	if e != nil {
		t.Fatal(e)
	}
	h := crc32.ChecksumIEEE([]byte("Quest/QuestProduct.bquestpack"))
	if got.Hashes[h] < 5000+0x10000 || got.Hashes[123] != 456 || got.Names["collision/path"] != 999999 {
		t.Fatal("resource registry changed incorrectly")
	}
	if _, e = ReadRSTB(b[:len(b)-1]); e == nil {
		t.Fatal("accepted partial registry")
	}
}
func dicLookup(d []byte, o uint64, name string) int {
	e := binary.LittleEndian
	prev, cur := uint32(0), uint32(e.Uint16(d[o+12:]))
	bitAt := func(i uint32) int32 { return int32(e.Uint32(d[o+8+uint64(i)*16:])) }
	steps := 0
	for bitAt(prev) < bitAt(cur) {
		prev = cur
		at := o + 8 + uint64(cur)*16
		cur = uint32(e.Uint16(d[at+4+uint64(bit(name, int(bitAt(cur))))*2:]))
		steps++
		if steps > 10000 {
			return -1
		}
	}
	at := o + 8 + uint64(cur)*16
	p := e.Uint64(d[at+8:])
	n := e.Uint16(d[p:])
	if string(d[p+2:p+2+uint64(n)]) != name {
		return -1
	}
	return int(cur) - 1
}
func TestNoopFlowHasBoundActionAndValidDictionaries(t *testing.T) {
	names := []string{"Ready", "Finish_Npc_Kakariko002_Talk", "Demo103_0", "Near", "Start", "Finish"}
	b, e := NoopFlow("SandboxTest", names)
	if e != nil {
		t.Fatal(e)
	}
	le := binary.LittleEndian
	flow := le.Uint64(b[le.Uint64(b[0x28:]):])
	dic := le.Uint64(b[flow+0x38:])
	sortNames := append([]string(nil), names...)
	// Do not merely read the DIC sequentially: exercise the lookup the engine uses.
	for _, name := range sortNames {
		if dicLookup(b, dic, name) < 0 {
			t.Fatal("dictionary cannot find", name)
		}
	}
	for _, absent := range []string{"Other", "Read", "Finis", ""} {
		if dicLookup(b, dic, absent) >= 0 {
			t.Fatal("dictionary false positive", absent)
		}
	}
	if le.Uint16(b[flow+0x10:]) != 1 || le.Uint16(b[flow+0x16:]) != 1 {
		t.Fatal("flow must bind one actor and event")
	}
	ev := le.Uint64(b[flow+0x30:])
	if le.Uint16(b[ev+0xa:]) != 0xffff || b[ev+8] != 0 {
		t.Fatal("action must terminate normally")
	}
	reloc := le.Uint32(b[24:])
	count := le.Uint32(b[reloc+36:])
	for i := uint32(0); i < count; i++ {
		p := le.Uint32(b[reloc+40+i*8:])
		if p%8 != 0 || uint64(p)+8 > uint64(reloc) {
			t.Fatal("bad relocation")
		}
		to := le.Uint64(b[p:])
		if to >= uint64(reloc) {
			t.Fatal("pointer outside resource")
		}
	}
	if out := os.Getenv("HYRULE_FORMAT_EXPORT"); out != "" {
		os.MkdirAll(out, 0700)
		os.WriteFile(filepath.Join(out, "noop.bfevfl"), b, 0600)
		os.WriteFile(filepath.Join(out, "noop.bdemo"), NoopDemo(), 0600)
		by, _ := WriteBYML(testDoc())
		os.WriteFile(filepath.Join(out, "sample.byml"), by, 0600)
	}
}
func TestNoopDemoAAMPDefaults(t *testing.T) {
	d := NoopDemo()
	e := binary.LittleEndian
	if string(d[:4]) != "AAMP" || int(e.Uint32(d[12:])) != len(d) {
		t.Fatal("invalid AAMP header")
	}
	root := 0x30 + int(e.Uint32(d[0x14:]))
	obj := root + int(e.Uint16(d[root+8:]))*4
	params := obj + int(e.Uint16(d[obj+4:]))*4
	count := int(e.Uint16(d[obj+6:]))
	seen := map[uint32]string{}
	for i := 0; i < count; i++ {
		at := params + 8*i
		raw := e.Uint32(d[at+4:])
		value := at + int(raw&0xffffff)*4
		if value >= len(d) {
			t.Fatal("AAMP value outside resource")
		}
		name := e.Uint32(d[at:])
		if raw>>24 == 7 {
			seen[name] = strings.TrimRight(string(d[value:value+32]), "\x00")
		} else if e.Uint32(d[value:]) != 0 {
			t.Fatal("unsafe demo boolean")
		}
	}
	for name, want := range map[string]string{"Extension": "evfl", "ProductFormat": "RealTime", "StartType": "Seamless", "StartLocationType": "Player"} {
		if seen[crc32.ChecksumIEEE([]byte(name))] != want {
			t.Fatal("bad descriptor", name)
		}
	}
}
func FuzzBYMLBounds(f *testing.F) {
	b, _ := WriteBYML(testDoc())
	f.Add(b)
	f.Add([]byte("YB\x02\x00"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			return
		}
		_, _ = ReadBYML(b)
	})
}

func TestYaz0FinalRunClipsAtDeclaredSize(t *testing.T) {
	z := make([]byte, 16)
	copy(z, "Yaz0")
	binary.BigEndian.PutUint32(z[4:], 2)
	z = append(z, 0x80, 'A', 0x10, 0)
	raw, _, e := Unpack(z)
	if e != nil || string(raw) != "AA" {
		t.Fatal("did not match game decoder's final-run behavior", e)
	}
}
