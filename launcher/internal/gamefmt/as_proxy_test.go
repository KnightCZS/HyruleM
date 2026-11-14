package gamefmt

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"testing"
)

func proxyFixture() []byte {
	d := make([]byte, 0xd0)
	e := binary.LittleEndian
	copy(d, "AAMP")
	e.PutUint32(d[4:], 2)
	e.PutUint32(d[8:], 3)
	e.PutUint32(d[12:], uint32(len(d)))
	e.PutUint16(d[0x34:], 3)
	e.PutUint16(d[0x36:], 1) // root -> Elements
	e.PutUint32(d[0x3c:], crc32.ChecksumIEEE([]byte("Elements")))
	e.PutUint16(d[0x40:], 3)
	e.PutUint16(d[0x42:], 2)
	for i, leaf := range []int{0x48, 0x54} {
		e.PutUint32(d[leaf:], crc32.ChecksumIEEE([]byte("Element"+string(rune('0'+i)))))
		o := 0x60 + i*8
		p := 0x70 + i*16
		e.PutUint16(d[leaf+8:], uint16((o-leaf)/4))
		e.PutUint16(d[leaf+10:], 1)
		e.PutUint32(d[o:], crc32.ChecksumIEEE([]byte("Parameters")))
		e.PutUint16(d[o+4:], uint16((p-o)/4))
		e.PutUint16(d[o+6:], 2)
		e.PutUint32(d[p:], crc32.ChecksumIEEE([]byte("TypeIndex")))
		v := 0x90 + i*20
		e.PutUint32(d[p+4:], uint32((v-p)/4)|2<<24)
		e.PutUint32(d[v:], 67)
		e.PutUint32(d[p+8:], crc32.ChecksumIEEE([]byte("FileName")))
		e.PutUint32(d[p+12:], uint32((v+4-p-8)/4)|20<<24)
		if i == 0 {
			copy(d[v+4:], "Bad_Advance")
		} else {
			copy(d[v+4:], "Nml_Wait")
		}
	}
	e.PutUint16(d[0x58:], uint16((0xc0-0x54)/4))
	e.PutUint16(d[0x5a:], 1) // selected leaf has events to remove
	return d
}
func TestProxyASSelectsVisualLeafAndRemovesEventGraph(t *testing.T) {
	src := proxyFixture()
	before := append([]byte(nil), src...)
	out, err := ProxyAS(src, "Wait")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(src, before) {
		t.Fatal("source AS changed")
	}
	e := binary.LittleEndian
	if e.Uint32(out[24:]) != 3 || e.Uint32(out[28:]) != 1 || e.Uint32(out[32:]) != 3 || e.Uint16(out[0x52:]) != 0 || len(out) != 140 {
		t.Fatal("selector/events still reachable")
	}
	if _, err = ProxyAS(src[:50], "Wait"); err == nil {
		t.Fatal("truncated resource accepted")
	}
	e.PutUint32(src[0x84:], 0xffffff)
	if _, err = ProxyAS(src, "Wait"); err == nil {
		t.Fatal("invalid parameter reference accepted")
	}
}
