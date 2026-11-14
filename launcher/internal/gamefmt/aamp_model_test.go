package gamefmt

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"testing"
)

func modelFixture()[]byte {
	d:=make([]byte,0x60);e:=binary.LittleEndian;copy(d,"AAMP");e.PutUint32(d[4:],2);e.PutUint32(d[8:],3);e.PutUint32(d[12:],uint32(len(d)));e.PutUint32(d[20:],4);e.PutUint32(d[24:],1);e.PutUint32(d[28:],1);e.PutUint32(d[32:],2);e.PutUint32(d[40:],8)
	e.PutUint32(d[0x34:],crc32.ChecksumIEEE([]byte("param_root")));e.PutUint16(d[0x3c:],3);e.PutUint16(d[0x3e:],1);e.PutUint32(d[0x40:],crc32.ChecksumIEEE([]byte("Base")));e.PutUint16(d[0x44:],2);e.PutUint16(d[0x46:],2)
	e.PutUint32(d[0x48:],crc32.ChecksumIEEE([]byte("Folder")));e.PutUint32(d[0x4c:],4|8<<24);e.PutUint32(d[0x50:],crc32.ChecksumIEEE([]byte("UnitName")));e.PutUint32(d[0x54:],2|8<<24);copy(d[0x58:],"Link");return d
}
func TestModelResourceChangesReferencesWithoutCorruptingSharedStrings(t *testing.T) {
	src:=modelFixture();before:=append([]byte(nil),src...);d,err:=PatchModelResource(src,"Armor_001","Armor_001_Head_A");if err!=nil{t.Fatal(err)}
	if !bytes.Equal(src,before)||string(d[0x58:0x5c])!="Link"{t.Fatal("source/shared string overwritten")}
	e:=binary.LittleEndian
	for at,want:=range map[int]string{0x48:"Armor_001",0x50:"Armor_001_Head_A"}{p:=at+int(e.Uint32(d[at+4:])&0xffffff)*4;n:=bytes.IndexByte(d[p:],0);if n<0||string(d[p:p+n])!=want||d[at+7]!=8{t.Fatal("model reference not redirected",at)}}
	if int(e.Uint32(d[12:]))!=len(d)||int(e.Uint32(d[40:]))!=8+len(d)-len(src){t.Fatal("AAMP size accounting")}
	if _,err=PatchModelResource(src[:60],"a","b");err==nil{t.Fatal("truncated model accepted")}
}
