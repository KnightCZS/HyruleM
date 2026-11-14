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
	filenames := map[int]bool{}
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
				filenames[p] = true
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("ASList: ASDefines ausente")
	}
	paramStart := root + int(e.Uint32(src[24:]))*12 + int(e.Uint32(src[28:]))*8
	np := int(e.Uint32(src[32:]))
	dataStart := paramStart + np*8
	stringsAt := dataStart + int(e.Uint32(src[36:]))
	if !inside(paramStart, np*8) || !inside(stringsAt, int(e.Uint32(src[40:]))) || e.Uint32(src[44:]) != 0 || stringsAt+int(e.Uint32(src[40:])) != len(src) {
		return nil, fmt.Errorf("ASList: seções inconsistentes")
	}
	d := append([]byte(nil), src[:stringsAt]...)
	pool := map[string]int{}
	for i := 0; i < np; i++ {
		p := paramStart + i*8
		v := e.Uint32(src[p+4:])
		typ := v >> 24
		if typ != 7 && typ != 8 && typ != 15 && typ != 20 {
			continue
		}
		at := p + int(v&0xffffff)*4
		if at < stringsAt || !inside(at, 1) {
			return nil, fmt.Errorf("ASList: string fora da seção")
		}
		n := bytes.IndexByte(src[at:], 0)
		if n < 0 || n > 4096 {
			return nil, fmt.Errorf("ASList: string inválida")
		}
		value := string(src[at : at+n])
		if filenames[p] {
			value = prefix + value
		}
		limit := 4096
		if typ == 7 {
			limit = 32
		}
		if typ == 8 {
			limit = 64
		}
		if typ == 15 {
			limit = 256
		}
		if len(value) >= limit {
			return nil, fmt.Errorf("ASList: string excessiva")
		}
		target, ok := pool[value]
		if !ok {
			target = len(d)
			pool[value] = target
			d = append(d, []byte(value)...)
			d = append(d, 0)
			for len(d)%4 != 0 {
				d = append(d, 0)
			}
		}
		rel := (target - p) / 4
		if rel > 0xffffff {
			return nil, fmt.Errorf("ASList excessiva")
		}
		e.PutUint32(d[p+4:], uint32(rel)|(v&0xff000000))
	}
	e.PutUint32(d[12:], uint32(len(d)))
	e.PutUint32(d[40:], uint32(len(d)-stringsAt))
	return d, nil
}
