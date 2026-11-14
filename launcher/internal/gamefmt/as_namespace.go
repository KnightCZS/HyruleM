package gamefmt

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

// NamespaceASList leaves public AS names unchanged but gives the private
// visual resources unique cache keys, so they cannot replace Player resources.
func NamespaceASList(src []byte, prefix string) ([]byte, error) {
	e := binary.LittleEndian
	inside := func(at, n int) bool { return at >= 0 && n >= 0 && at <= len(src) && n <= len(src)-at }
	if len(src) < 0x40 || string(src[:4]) != "AAMP" || int(e.Uint32(src[12:])) != len(src) {
		return nil, fmt.Errorf("ASList inválida")
	}
	root := 0x30 + int(e.Uint32(src[20:]))
	if !inside(root, 12) {
		return nil, fmt.Errorf("ASList: raiz inválida")
	}
	lists := root + int(e.Uint16(src[root+4:]))*4
	count := int(e.Uint16(src[root+6:]))
	if !inside(lists, count*12) {
		return nil, fmt.Errorf("ASList: listas inválidas")
	}
	d := append([]byte(nil), src...)
	found := false
	for i := 0; i < count; i++ {
		l := lists + i*12
		if e.Uint32(src[l:]) != crc32.ChecksumIEEE([]byte("ASDefines")) {
			continue
		}
		found = true
		objs := l + int(e.Uint16(src[l+8:]))*4
		no := int(e.Uint16(src[l+10:]))
		if !inside(objs, no*8) {
			return nil, fmt.Errorf("ASList: definições inválidas")
		}
		for j := 0; j < no; j++ {
			o := objs + j*8
			params := o + int(e.Uint16(src[o+4:]))*4
			np := int(e.Uint16(src[o+6:]))
			if !inside(params, np*8) {
				return nil, fmt.Errorf("ASList: parâmetros inválidos")
			}
			for k := 0; k < np; k++ {
				p := params + k*8
				if e.Uint32(src[p:]) != crc32.ChecksumIEEE([]byte("Filename")) {
					continue
				}
				v := e.Uint32(src[p+4:])
				at := p + int(v&0xffffff)*4
				if (v>>24 != 20 && v>>24 != 8) || !inside(at, 1) {
					return nil, fmt.Errorf("ASList: StringRef inválida")
				}
				end := bytes.IndexByte(src[at:], 0)
				if end < 0 || end > 120 {
					return nil, fmt.Errorf("ASList: nome inválido")
				}
				for len(d)%4 != 0 {
					d = append(d, 0)
				}
				rel := (len(d) - p) / 4
				if rel > 0xffffff {
					return nil, fmt.Errorf("ASList excessiva")
				}
				e.PutUint32(d[p+4:], uint32(rel)|(v&0xff000000))
				d = append(d, []byte(prefix)...)
				d = append(d, src[at:at+end]...)
				d = append(d, 0)
				if v>>24 == 8 {
					if len(prefix)+end >= 64 {
						return nil, fmt.Errorf("ASList: String64 excessiva")
					}
					for n := len(prefix) + end + 1; n < 64; n++ {
						d = append(d, 0)
					}
				}
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("ASList: ASDefines ausente")
	}
	for len(d)%4 != 0 {
		d = append(d, 0)
	}
	e.PutUint32(d[12:], uint32(len(d)))
	e.PutUint32(d[40:], e.Uint32(src[40:])+uint32(len(d)-len(src)))
	return d, nil
}
