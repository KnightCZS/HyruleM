package gamefmt

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math"
	"strings"
)

// ProxyAS keeps one generic skeletal leaf from the owner's AS resource. The
// Player's selector graph and attack/damage events require a concrete Player;
// running those on a visual NPC can leave it unanimated or invoke gameplay.
// The result is a new canonical archive: counts and data tables describe only
// the selected visual leaf, with no orphaned graph entries.
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
	chosen := ""
	chosenMorph := float32(4)
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
		morph := float32(4)
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
				case crc32.ChecksumIEEE([]byte("Morph")):
					if src[p+7] == 1 {
						v := math.Float32frombits(e.Uint32(src[data:]))
						if !math.IsNaN(float64(v)) && v >= 0 && v <= 12 {
							morph = v
						}
					}
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
			chosen = name
			chosenMorph = morph
		}
	}
	if best < 0 {
		return nil, fmt.Errorf("AS: nenhuma animação esquelética genérica")
	}
	d := skeletalAS(chosen)
	e.PutUint32(d[0x7c:], math.Float32bits(chosenMorph))
	return d, nil
}

func skeletalAS(name string) []byte {
	// Header + three lists + one object + three parameters, followed by the
	// scalar data section and a four-byte-aligned string pool.
	size := (0x80 + len(name) + 1 + 3) &^ 3
	d := make([]byte, size)
	e := binary.LittleEndian
	copy(d, "AAMP")
	for at, value := range map[int]uint32{4: 2, 8: 3, 12: uint32(size), 20: 4, 24: 3, 28: 1, 32: 3, 36: 8, 40: uint32(size - 0x80)} {
		e.PutUint32(d[at:], value)
	}
	copy(d[0x30:], "xml")
	for at, label := range map[int]string{0x34: "param_root", 0x40: "Elements", 0x4c: "Element0", 0x58: "Parameters", 0x60: "TypeIndex", 0x68: "FileName", 0x70: "Morph"} {
		e.PutUint32(d[at:], crc32.ChecksumIEEE([]byte(label)))
	}
	e.PutUint16(d[0x3c:], 9)
	e.PutUint16(d[0x48:], 6)
	e.PutUint16(d[0x50:], 3)
	e.PutUint16(d[0x38:], 3)
	e.PutUint16(d[0x3a:], 1)
	e.PutUint16(d[0x44:], 3)
	e.PutUint16(d[0x46:], 1)
	e.PutUint16(d[0x54:], 3)
	e.PutUint16(d[0x56:], 1)
	e.PutUint16(d[0x5c:], 2)
	e.PutUint16(d[0x5e:], 3)
	e.PutUint32(d[0x64:], 6|2<<24)
	e.PutUint32(d[0x6c:], 6|8<<24)
	e.PutUint32(d[0x74:], 3|1<<24)
	e.PutUint32(d[0x78:], 67)
	copy(d[0x80:], name)
	return d
}
