package gamefmt

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"testing"
)

func TestASNamespacePreservesSharedPublicNames(t *testing.T) {
	d := make([]byte, 0xa0)
	e := binary.LittleEndian
	copy(d, "AAMP")
	e.PutUint32(d[4:], 2)
	e.PutUint32(d[8:], 3)
	e.PutUint32(d[12:], uint32(len(d)))
	e.PutUint16(d[0x34:], 3)
	e.PutUint16(d[0x36:], 1)
	e.PutUint32(d[0x3c:], crc32.ChecksumIEEE([]byte("ASDefines")))
	e.PutUint16(d[0x44:], 3)
	e.PutUint16(d[0x46:], 1)
	e.PutUint32(d[0x48:], crc32.ChecksumIEEE([]byte("ASDefine_0")))
	e.PutUint16(d[0x4c:], 2)
	e.PutUint16(d[0x4e:], 2)
	for i, key := range []string{"Name", "Filename"} {
		p := 0x50 + i*8
		e.PutUint32(d[p:], crc32.ChecksumIEEE([]byte(key)))
		e.PutUint32(d[p+4:], uint32((0x60-p)/4)|8<<24)
	}
	copy(d[0x60:], "Shared")
	before := append([]byte(nil), d...)
	out, err := NamespaceASList(d, "HM_")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(d, before) {
		t.Fatal("source changed")
	}
	if string(out[0x60:0x66]) != "Shared" {
		t.Fatal("public/shared name overwritten")
	}
	p := 0x58
	at := p + int(e.Uint32(out[p+4:])&0xffffff)*4
	if string(out[at:at+9]) != "HM_Shared" || e.Uint32(out[p+4:])>>24 != 8 {
		t.Fatal("private filename not redirected")
	}
}
