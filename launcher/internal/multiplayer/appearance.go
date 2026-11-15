package multiplayer

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

const MaxVisibilityEntries = 512
const MaxEffectProperties = 256

// Private resource names are represented only by CRC32 identities. Visibility
// and ELink numeric conditions are discrete state, never interpolated indices.
func validAppearance(encoded string) bool {
	if encoded == "" {
		return true
	}
	if strings.ContainsAny(encoded, "\r\n\x00 \t") {
		return false
	}
	if len(encoded) > base64.StdEncoding.EncodedLen(8+16*(MaxVisibilityEntries+MaxEffectProperties)) {
		return false
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(raw) < 8 || string(raw[:4]) != "HMA1" {
		return false
	}
	shapes, properties := int(binary.LittleEndian.Uint16(raw[4:])), int(binary.LittleEndian.Uint16(raw[6:]))
	if shapes > MaxVisibilityEntries || properties > MaxEffectProperties || len(raw) != 8+16*(shapes+properties) {
		return false
	}
	seen := map[[3]uint32]bool{}
	for i := 0; i < shapes+properties; i++ {
		p := raw[8+i*16:]
		slot, name, kind, value := binary.LittleEndian.Uint32(p), binary.LittleEndian.Uint32(p[4:]), binary.LittleEndian.Uint32(p[8:]), binary.LittleEndian.Uint32(p[12:])
		property := i >= shapes
		group := uint32(0)
		if property {
			group = 1
		}
		key := [3]uint32{group, slot, name}
		if !property {
			key[0] = value + 2
		}
		if name == 0 || seen[key] {
			return false
		}
		seen[key] = true
		if !property {
			if (slot != 0 && (slot < 4 || slot > 6)) || kind > 1 || value > 1 {
				return false
			}
		} else {
			if slot > 3 || kind > 3 {
				return false
			}
			if kind == 2 {
				f := math.Float32frombits(value)
				if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
					return false
				}
			}
		}
	}
	return true
}

func appearanceToken(s string) (string, error) {
	if !validAppearance(s) {
		return "", fmt.Errorf("visibilidade/propriedades visuais inválidas")
	}
	if s == "" {
		return "-", nil
	}
	return s, nil
}
