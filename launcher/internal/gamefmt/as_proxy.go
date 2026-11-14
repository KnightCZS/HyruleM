package gamefmt

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"strings"
)

// ProxyAS keeps one generic skeletal leaf from the owner's AS resource. The
// Player's selector graph and attack/damage events require a concrete Player;
// running those on a visual NPC can leave it unanimated or invoke gameplay.
// All original data remains in the private file, but only this leaf is reachable.
func ProxyAS(src []byte, action string) ([]byte, error) {
	e := binary.LittleEndian
	if len(src) < 0x40 || string(src[:4]) != "AAMP" || e.Uint32(src[4:]) != 2 || int(e.Uint32(src[12:])) != len(src) {
		return nil, fmt.Errorf("AS: recurso AAMP inválido")
	}
	inside := func(at, n int) bool { return at >= 0 && n >= 0 && at <= len(src) && n <= len(src)-at }
	root := 0x30 + int(e.Uint32(src[20:]))
	if !inside(root, 12) {
		return nil, fmt.Errorf("AS: raiz inválida")
	}
	lists := root + int(e.Uint16(src[root+4:]))*4
	count := int(e.Uint16(src[root+6:]))
	if !inside(lists, count*12) {
		return nil, fmt.Errorf("AS: listas inválidas")
	}
	elements := -1
	for i := 0; i < count; i++ {
		p := lists + i*12
		if e.Uint32(src[p:]) == crc32.ChecksumIEEE([]byte("Elements")) {
			elements = p
			break
		}
	}
	if elements < 0 {
		return nil, fmt.Errorf("AS: Elements ausente")
	}
	children := elements + int(e.Uint16(src[elements+4:]))*4
	count = int(e.Uint16(src[elements+6:]))
	if count > 4096 || !inside(children, count*12) {
		return nil, fmt.Errorf("AS: elementos inválidos")
	}
	best, score := -1, -100000
	preferred := map[string]string{"Wait": "Nml_Wait", "WaitUpper": "Nml_Wait", "Move": "Nml_Move_Run", "MoveUpper": "Move_Upper_Run", "CutS1": "Sword_Attack_S1", "Jump": "Nml_Wait_Jump"}[action]
	for i := 0; i < count; i++ {
		leaf := children + i*12
		objects := leaf + int(e.Uint16(src[leaf+8:]))*4
		no := int(e.Uint16(src[leaf+10:]))
		if !inside(objects, no*8) {
			return nil, fmt.Errorf("AS: objetos inválidos")
		}
		kind := uint32(0)
		name := ""
		for j := 0; j < no; j++ {
			o := objects + j*8
			if e.Uint32(src[o:]) != crc32.ChecksumIEEE([]byte("Parameters")) {
				continue
			}
			params := o + int(e.Uint16(src[o+4:]))*4
			np := int(e.Uint16(src[o+6:]))
			if !inside(params, np*8) {
				return nil, fmt.Errorf("AS: parâmetros inválidos")
			}
			for k := 0; k < np; k++ {
				p := params + k*8
				data := p + int(e.Uint32(src[p+4:])&0xffffff)*4
				if !inside(data, 4) {
					return nil, fmt.Errorf("AS: valor inválido")
				}
				switch e.Uint32(src[p:]) {
				case crc32.ChecksumIEEE([]byte("TypeIndex")):
					kind = e.Uint32(src[data:])
				case crc32.ChecksumIEEE([]byte("FileName")):
					end := data + 64
					if end > len(src) {
						end = len(src)
					}
					n := bytes.IndexByte(src[data:end], 0)
					if n > 0 {
						name = string(src[data : data+n])
					}
				}
			}
		}
		if kind != 67 || name == "" {
			continue
		}
		s := 0
		if strings.HasPrefix(name, "Nml_") {
			s += 20
		}
		if strings.HasPrefix(name, "Sword_") {
			s += 10
		}
		if strings.Contains(name, "Advance") || strings.HasPrefix(name, "A_Pose") {
			s -= 100
		}
		if name == preferred {
			s += 1000
		}
		if s > score {
			best, score = leaf, s
		}
	}
	if best < 0 {
		return nil, fmt.Errorf("AS: nenhuma animação esquelética genérica")
	}
	if best-elements > 0xffff*4 {
		return nil, fmt.Errorf("AS: referência ao elemento excessiva")
	}
	d := append([]byte(nil), src...)
	e.PutUint16(d[elements+4:], uint16((best-elements)/4))
	e.PutUint16(d[elements+6:], 1)
	e.PutUint32(d[best:], crc32.ChecksumIEEE([]byte("Element0")))
	// No selector children, frame events, trigger events or Player callbacks.
	e.PutUint16(d[best+6:], 0)
	return d, nil
}
