package multiplayer

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
)

const MaxPoseBones = 512

// Bone matrices are the final rendered transforms relative to Link's root.
// Names are CRC32 identities, never process addresses or game resources.
type PoseBone struct {
	Name   uint32
	Part   uint32 // 0: body, 4/5/6: equipped head/upper/lower model
	Matrix [12]float32
}

func decodeSkeleton(encoded string) ([]PoseBone, error) {
	if encoded == "" {
		return nil, nil
	}
	if len(encoded) > base64.StdEncoding.EncodedLen(8+56*MaxPoseBones) {
		return nil, fmt.Errorf("pose esquelética excede limite")
	}
	b, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(b) < 8 || (string(b[:4]) != "HMP1" && string(b[:4]) != "HMP2") || binary.LittleEndian.Uint16(b[6:]) != 0 {
		return nil, fmt.Errorf("pose esquelética inválida")
	}
	n := int(binary.LittleEndian.Uint16(b[4:]))
	stride := 52
	if string(b[:4]) == "HMP2" {
		stride = 56
	}
	if n < 1 || n > MaxPoseBones || len(b) != 8+n*stride {
		return nil, fmt.Errorf("tamanho de pose inválido")
	}
	pose := make([]PoseBone, n)
	seen := make(map[[2]uint32]bool, n)
	for i := range pose {
		at := 8 + i*stride
		p := &pose[i]
		p.Name = binary.LittleEndian.Uint32(b[at:])
		if stride == 56 {
			p.Part = binary.LittleEndian.Uint32(b[at+4:])
		}
		key := [2]uint32{p.Part, p.Name}
		if p.Name == 0 || seen[key] || (p.Part != 0 && (p.Part < 4 || p.Part > 6)) {
			return nil, fmt.Errorf("identidade de osso inválida")
		}
		seen[key] = true
		for j := range p.Matrix {
			v := math.Float32frombits(binary.LittleEndian.Uint32(b[at+stride-48+j*4:]))
			limit := float32(8)
			if j%4 == 3 {
				limit = 128
			}
			if !finite(float64(v)) || v > limit || v < -limit {
				return nil, fmt.Errorf("matriz de osso inválida")
			}
			p.Matrix[j] = v
		}
		for row := 0; row < 3; row++ {
			norm := float32(0)
			for j := 0; j < 3; j++ {
				v := p.Matrix[row*4+j]
				norm += v * v
			}
			if norm < .0001 {
				return nil, fmt.Errorf("matriz degenerada")
			}
		}
	}
	return pose, nil
}

func encodeSkeleton(pose []PoseBone) string {
	if len(pose) == 0 {
		return ""
	}
	b := make([]byte, 8+56*len(pose))
	copy(b, "HMP2")
	binary.LittleEndian.PutUint16(b[4:], uint16(len(pose)))
	for i, p := range pose {
		at := 8 + i*56
		binary.LittleEndian.PutUint32(b[at:], p.Name)
		binary.LittleEndian.PutUint32(b[at+4:], p.Part)
		for j, v := range p.Matrix {
			binary.LittleEndian.PutUint32(b[at+8+j*4:], math.Float32bits(v))
		}
	}
	return base64.StdEncoding.EncodeToString(b)
}

func interpolateSkeleton(a, b string, t float64) (string, bool) {
	if a == b || t >= 1 {
		return b, false
	}
	if t <= 0 {
		return a, false
	}
	old, e1 := decodeSkeleton(a)
	next, e2 := decodeSkeleton(b)
	if e1 != nil || e2 != nil || len(old) == 0 || len(next) == 0 {
		return a, true
	}
	byName := make(map[[2]uint32][12]float32, len(old))
	for _, bone := range old {
		byName[[2]uint32{bone.Part, bone.Name}] = bone.Matrix
	}
	for i, bone := range next {
		if m, ok := byName[[2]uint32{bone.Part, bone.Name}]; ok {
			for j := range bone.Matrix {
				bone.Matrix[j] = m[j]*float32(1-t) + bone.Matrix[j]*float32(t)
			}
			next[i] = bone
		}
	}
	encoded := encodeSkeleton(next)
	if _, err := decodeSkeleton(encoded); err != nil {
		return b, true
	}
	return encoded, false
}
