package gamefmt

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

// PatchModelResource changes only the model archive and unit. The Player's
// animation targets and partial bone masks are kept intact. String values are
// appended rather than overwriting a deduplicated source string.
func PatchModelResource(src []byte, folder, unit string) ([]byte, error) {
	e := binary.LittleEndian
	if len(src) < 0x40 || string(src[:4]) != "AAMP" || e.Uint32(src[4:]) != 2 || int(e.Uint32(src[12:])) != len(src) {
		return nil, fmt.Errorf("ModelList AAMP inválida")
	}
	wanted := map[uint32]string{crc32.ChecksumIEEE([]byte("Folder")): folder, crc32.ChecksumIEEE([]byte("UnitName")): unit}
	positions := map[uint32]int{}
	seen := map[int]bool{}
	inside := func(at, n int) bool { return at >= 0 && n >= 0 && at <= len(src) && n <= len(src)-at }
	var visit func(int) error
	visit = func(at int) error {
		if !inside(at, 12) || seen[at] || len(seen) > 4096 {
			return fmt.Errorf("ModelList: árvore inválida")
		}
		seen[at] = true
		objects, no := at+int(e.Uint16(src[at+8:]))*4, int(e.Uint16(src[at+10:]))
		lists, nl := at+int(e.Uint16(src[at+4:]))*4, int(e.Uint16(src[at+6:]))
		if !inside(objects, no*8) || !inside(lists, nl*12) {
			return fmt.Errorf("ModelList: tabela truncada")
		}
		for i := 0; i < no; i++ {
			o := objects + i*8
			p, np := o+int(e.Uint16(src[o+4:]))*4, int(e.Uint16(src[o+6:]))
			if !inside(p, np*8) {
				return fmt.Errorf("ModelList: parâmetros truncados")
			}
			for j := 0; j < np; j++ {
				x := p + j*8
				hash := e.Uint32(src[x:])
				if _, ok := wanted[hash]; ok {
					if _, dup := positions[hash]; dup {
						return fmt.Errorf("ModelList: múltiplos modelos não suportados")
					}
					positions[hash] = x
				}
			}
		}
		for i := 0; i < nl; i++ {
			if err := visit(lists + i*12); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(0x30 + int(e.Uint32(src[20:]))); err != nil {
		return nil, err
	}
	d := append([]byte(nil), src...)
	for _, key := range []string{"Folder", "UnitName"} {
		h := crc32.ChecksumIEEE([]byte(key))
		p, ok := positions[h]
		value := wanted[h]
		if !ok || value == "" || len(value) > 63 || bytes.IndexByte([]byte(value), 0) >= 0 {
			return nil, fmt.Errorf("ModelList: recurso inválido")
		}
		t := src[p+7]
		if t != 7 && t != 8 && t != 20 {
			return nil, fmt.Errorf("ModelList: tipo de string incompatível")
		}
		old := p + int(e.Uint32(src[p+4:])&0xffffff)*4
		if !inside(old, 1) || bytes.IndexByte(src[old:], 0) < 0 {
			return nil, fmt.Errorf("ModelList: string truncada")
		}
		for len(d)%4 != 0 {
			d = append(d, 0)
		}
		rel := (len(d) - p) / 4
		if rel > 0xffffff {
			return nil, fmt.Errorf("ModelList excessiva")
		}
		e.PutUint32(d[p+4:], uint32(rel)|uint32(t)<<24)
		d = append(d, []byte(value)...)
		d = append(d, 0)
	}
	for len(d)%4 != 0 {
		d = append(d, 0)
	}
	e.PutUint32(d[12:], uint32(len(d)))
	e.PutUint32(d[40:], e.Uint32(src[40:])+uint32(len(d)-len(src)))
	return d, nil
}
