package gamefmt

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"sort"
	"strings"
)

// PatchAAMPStrings redirects StringRef parameters in one named object. Appending
// strings preserves deduplicated values used by other parameters (ProfileUser
// and ASUser, for example, can point at the same original string).
func PatchAAMPStrings(src []byte, object string, replacements map[string]string) ([]byte, error) {
	e := binary.LittleEndian
	if len(src) < 0x30 || len(src) > MaxData || string(src[:4]) != "AAMP" || e.Uint32(src[4:]) != 2 || e.Uint32(src[8:]) != 3 || int(e.Uint32(src[12:])) != len(src) || e.Uint32(src[44:]) != 0 {
		return nil, fmt.Errorf("AAMP: esperado formato binário v2 little endian sem seção desconhecida")
	}
	d := append([]byte(nil), src...)
	root := uint64(0x30) + uint64(e.Uint32(d[20:]))
	seen := map[uint64]bool{}
	params := map[uint32]int{}
	found := false
	inside := func(at, n uint64) bool { return at <= uint64(len(src)) && n <= uint64(len(src))-at }
	var visit func(uint64) error
	visit = func(at uint64) error {
		if !inside(at, 12) || seen[at] || len(seen) > 4096 {
			return fmt.Errorf("AAMP: lista inválida ou cíclica")
		}
		seen[at] = true
		lists, nl := at+uint64(e.Uint16(d[at+4:]))*4, uint64(e.Uint16(d[at+6:]))
		objects, no := at+uint64(e.Uint16(d[at+8:]))*4, uint64(e.Uint16(d[at+10:]))
		if !inside(lists, nl*12) || !inside(objects, no*8) {
			return fmt.Errorf("AAMP: tabela truncada")
		}
		for i := uint64(0); i < no; i++ {
			o := objects + i*8
			if e.Uint32(d[o:]) != crc32.ChecksumIEEE([]byte(object)) {
				continue
			}
			if found {
				return fmt.Errorf("AAMP: objeto duplicado")
			}
			found = true
			p, n := o+uint64(e.Uint16(d[o+4:]))*4, uint64(e.Uint16(d[o+6:]))
			if !inside(p, n*8) {
				return fmt.Errorf("AAMP: parâmetros truncados")
			}
			for j := uint64(0); j < n; j++ {
				a := p + j*8
				h := e.Uint32(d[a:])
				if _, exists := params[h]; exists {
					return fmt.Errorf("AAMP: parâmetro duplicado")
				}
				params[h] = int(a)
			}
		}
		for i := uint64(0); i < nl; i++ {
			if err := visit(lists + i*12); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(root); err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("AAMP: objeto %s ausente", object)
	}
	// Stable order makes local overlays reproducible.
	keys := make([]string, 0, len(replacements))
	for key := range replacements {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := replacements[key]
		at, ok := params[crc32.ChecksumIEEE([]byte(key))]
		if !ok || d[at+7] != 20 || len(value) > 255 || strings.ContainsRune(value, 0) {
			return nil, fmt.Errorf("AAMP: StringRef inválida: %s", key)
		}
		old := at + int(e.Uint32(d[at+4:])&0xffffff)*4
		if old < 0 || old >= len(src) || bytes.IndexByte(src[old:], 0) < 0 {
			return nil, fmt.Errorf("AAMP: string original inválida")
		}
		for len(d)%4 != 0 {
			d = append(d, 0)
		}
		rel := (len(d) - at) / 4
		if rel < 0 || rel > 0xffffff {
			return nil, fmt.Errorf("AAMP: referência excessiva")
		}
		e.PutUint32(d[at+4:], uint32(rel)|20<<24)
		d = append(d, value...)
		d = append(d, 0)
	}
	for len(d)%4 != 0 {
		d = append(d, 0)
	}
	if len(d) > MaxData {
		return nil, fmt.Errorf("AAMP: expansão excessiva")
	}
	e.PutUint32(d[12:], uint32(len(d)))
	e.PutUint32(d[40:], e.Uint32(src[40:])+uint32(len(d)-len(src)))
	return d, nil
}
