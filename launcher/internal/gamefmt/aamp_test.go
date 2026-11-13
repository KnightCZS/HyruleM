package gamefmt

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"testing"
)

// Two parameters deliberately share a string, as real AAMP encoders do.
func sharedStringAAMP() []byte {
	d := make([]byte, 0x60)
	copy(d, "AAMP")
	e := binary.LittleEndian
	e.PutUint32(d[4:], 2)
	e.PutUint32(d[8:], 3)
	e.PutUint32(d[12:], uint32(len(d)))
	e.PutUint32(d[24:], 1)
	e.PutUint32(d[28:], 1)
	e.PutUint32(d[32:], 2)
	e.PutUint32(d[40:], 8)
	e.PutUint16(d[0x38:], 3)
	e.PutUint16(d[0x3a:], 1)
	e.PutUint32(d[0x3c:], crc32.ChecksumIEEE([]byte("LinkTarget")))
	e.PutUint16(d[0x40:], 2)
	e.PutUint16(d[0x42:], 2)
	for i, key := range []string{"ProfileUser", "ASUser"} {
		at := 0x44 + i*8
		e.PutUint32(d[at:], crc32.ChecksumIEEE([]byte(key)))
		e.PutUint32(d[at+4:], uint32((0x58-at)/4)|20<<24)
	}
	copy(d[0x58:], "Player\x00")
	return d
}
func TestAAMPPatchPreservesSharedStringsAndSource(t *testing.T) {
	d := sharedStringAAMP()
	before := append([]byte(nil), d...)
	out, err := PatchAAMPStrings(d, "LinkTarget", map[string]string{"ProfileUser": "DemoNPC"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(d, before) {
		t.Fatal("source changed")
	}
	e := binary.LittleEndian
	profile := 0x44 + int(e.Uint32(out[0x48:])&0xffffff)*4
	as := 0x4c + int(e.Uint32(out[0x50:])&0xffffff)*4
	if string(out[profile:profile+7]) != "DemoNPC" || string(out[as:as+6]) != "Player" {
		t.Fatal("shared string was corrupted")
	}
	if int(e.Uint32(out[12:])) != len(out) {
		t.Fatal("size not updated")
	}
}
func TestAAMPRejectsBadReferencesAndWrongTypes(t *testing.T) {
	for _, mutate := range []func([]byte){
		func(d []byte) { binary.LittleEndian.PutUint16(d[0x38:], 0xffff) },
		func(d []byte) { d[0x4b] = 1 },
		func(d []byte) { binary.LittleEndian.PutUint32(d[0x48:], 0x14ffffff) },
		func(d []byte) { binary.LittleEndian.PutUint32(d[20:], 0xffffffff) },
	} {
		d := sharedStringAAMP()
		mutate(d)
		if _, err := PatchAAMPStrings(d, "LinkTarget", map[string]string{"ProfileUser": "DemoNPC"}); err == nil {
			t.Fatal("invalid AAMP accepted")
		}
	}
}
