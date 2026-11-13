//go:build windows

package launcher

import (
	"encoding/binary"
	"os"
	"syscall"
	"unicode/utf16"
)

// NTFS directory junctions do not require symlink developer mode/admin rights.
func directoryLink(link, target string) error {
	if err := os.Mkdir(link, 0700); err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			os.Remove(link)
		}
	}()
	name, e := syscall.UTF16PtrFromString(link)
	if e != nil {
		return e
	}
	h, e := syscall.CreateFile(name, 0x40000000, 0, nil, 3, 0x00200000|0x02000000, 0)
	if e != nil {
		return e
	}
	defer syscall.CloseHandle(h)
	sub := utf16.Encode([]rune(`\??\` + target))
	print := utf16.Encode([]rune(target))
	b := make([]byte, 16+(len(sub)+len(print)+2)*2)
	binary.LittleEndian.PutUint32(b, 0xa0000003)
	binary.LittleEndian.PutUint16(b[4:], uint16(len(b)-8))
	binary.LittleEndian.PutUint16(b[10:], uint16(len(sub)*2))
	binary.LittleEndian.PutUint16(b[12:], uint16((len(sub)+1)*2))
	binary.LittleEndian.PutUint16(b[14:], uint16(len(print)*2))
	for i, v := range sub {
		binary.LittleEndian.PutUint16(b[16+i*2:], v)
	}
	for i, v := range print {
		binary.LittleEndian.PutUint16(b[16+(len(sub)+1+i)*2:], v)
	}
	var n uint32
	e = syscall.DeviceIoControl(h, 0x000900a4, &b[0], uint32(len(b)), nil, 0, &n, nil)
	ok = e == nil
	return e
}
