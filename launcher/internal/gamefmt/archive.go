package gamefmt

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
)

func Unpack(d []byte) ([]byte, bool, error) {
	if len(d) < 4 || string(d[:4]) != "Yaz0" {
		if len(d) > MaxData {
			return nil, false, fmt.Errorf("arquivo muito grande")
		}
		return append([]byte(nil), d...), false, nil
	}
	if len(d) < 16 {
		return nil, true, fmt.Errorf("Yaz0: cabeçalho truncado")
	}
	size := uint64(binary.BigEndian.Uint32(d[4:8]))
	if size > MaxData {
		return nil, true, fmt.Errorf("Yaz0: expansão excessiva")
	}
	out := make([]byte, 0, int(size))
	at := 16
	for uint64(len(out)) < size {
		if at >= len(d) {
			return nil, true, fmt.Errorf("Yaz0: stream truncado")
		}
		code := d[at]
		at++
		for bit := byte(0x80); bit != 0 && uint64(len(out)) < size; bit >>= 1 {
			if code&bit != 0 {
				if at >= len(d) {
					return nil, true, fmt.Errorf("Yaz0: literal truncado")
				}
				out = append(out, d[at])
				at++
				continue
			}
			if at+2 > len(d) {
				return nil, true, fmt.Errorf("Yaz0: referência truncada")
			}
			a, b := d[at], d[at+1]
			at += 2
			dist := int(a&15)<<8 | int(b)
			dist++
			count := int(a>>4) + 2
			if a>>4 == 0 {
				if at >= len(d) {
					return nil, true, fmt.Errorf("Yaz0: comprimento truncado")
				}
				count = int(d[at]) + 18
				at++
			}
			if dist > len(out) {
				return nil, true, fmt.Errorf("Yaz0: referência inválida")
			}
			// Match the game's decoder: the declared output size terminates the
			// final run, even when an encoder's last reference is longer.
			if uint64(len(out)+count) > size {
				count = int(size) - len(out)
			}
			for i := 0; i < count; i++ {
				out = append(out, out[len(out)-dist])
			}
		}
	}
	return out, true, nil
}

// PackYaz0 uses a bounded hash chain and the format's 4 KiB window. Output is
// deterministic; callers account for the decompressed resource size separately.
func PackYaz0(d []byte) []byte {
	out := make([]byte, 16, 16+len(d)/2)
	copy(out, "Yaz0")
	binary.BigEndian.PutUint32(out[4:], uint32(len(d)))
	head := make([]int, 65536)
	for i := range head {
		head[i] = -1
	}
	prev := make([]int32, len(d))
	hash := func(i int) int { return (int(d[i])*251 + int(d[i+1])*31 + int(d[i+2])) & 65535 }
	insert := func(i int) {
		if i+2 >= len(d) {
			return
		}
		h := hash(i)
		prev[i] = int32(head[h])
		head[h] = i
	}
	for at := 0; at < len(d); {
		codeAt := len(out)
		out = append(out, 0)
		for bit := byte(0x80); bit != 0 && at < len(d); bit >>= 1 {
			best, back := 0, 0
			if at+2 < len(d) {
				limit := len(d) - at
				if limit > 273 {
					limit = 273
				}
				candidate := head[hash(at)]
				for tries := 0; candidate >= 0 && at-candidate <= 4096 && tries < 256; tries++ {
					n := 0
					for n < limit && d[candidate+n] == d[at+n] {
						n++
					}
					if n > best {
						best = n
						back = at - candidate
						if n == limit {
							break
						}
					}
					candidate = int(prev[candidate])
				}
			}
			if best >= 3 {
				dist := back - 1
				if best >= 18 {
					out = append(out, byte(dist>>8), byte(dist), byte(best-18))
				} else {
					out = append(out, byte((best-2)<<4|dist>>8), byte(dist))
				}
				for k := 0; k < best; k++ {
					insert(at + k)
				}
				at += best
			} else {
				out[codeAt] |= bit
				out = append(out, d[at])
				insert(at)
				at++
			}
		}
	}
	return out
}
func Repack(d []byte, compressed bool) []byte {
	if compressed {
		return PackYaz0(d)
	}
	return d
}

type Entry struct {
	Name       string
	Node       int
	Start, End uint32
}
type SARC struct {
	Data    []byte
	Little  bool
	Entries []Entry
	dataOff uint32
	e       binary.ByteOrder
}

func ReadSARC(data []byte) (*SARC, error) {
	if len(data) < 0x28 || len(data) > MaxData || string(data[:4]) != "SARC" {
		return nil, fmt.Errorf("SARC: assinatura ou tamanho inválido")
	}
	e := binary.ByteOrder(binary.LittleEndian)
	little := true
	switch string(data[6:8]) {
	case "\xff\xfe":
	case "\xfe\xff":
		e = binary.BigEndian
		little = false
	default:
		return nil, fmt.Errorf("SARC: byte order inválido")
	}
	if e.Uint16(data[4:6]) != 0x14 || string(data[0x14:0x18]) != "SFAT" || e.Uint16(data[0x18:0x1a]) != 12 {
		return nil, fmt.Errorf("SARC: cabeçalho inválido")
	}
	size := e.Uint32(data[8:12])
	off := e.Uint32(data[12:16])
	count := int(e.Uint16(data[0x1a:0x1c]))
	names := 0x20 + count*16
	if uint64(size) > uint64(len(data)) || off > size || names+8 > int(off) || string(data[names:names+4]) != "SFNT" {
		return nil, fmt.Errorf("SARC: tabela inválida")
	}
	out := &SARC{Data: append([]byte(nil), data...), Little: little, dataOff: off, e: e}
	seen := map[string]bool{}
	for i := 0; i < count; i++ {
		at := 0x20 + i*16
		attr := e.Uint32(data[at+4:])
		start, end := e.Uint32(data[at+8:]), e.Uint32(data[at+12:])
		if start > end || uint64(off)+uint64(end) > uint64(size) {
			return nil, fmt.Errorf("SARC: entrada fora do arquivo")
		}
		name := ""
		if attr>>24 != 0 {
			p := uint64(names+8) + uint64(attr&0xffffff)*4
			if p >= uint64(off) {
				return nil, fmt.Errorf("SARC: nome inválido")
			}
			n := bytes.IndexByte(data[p:off], 0)
			if n < 0 {
				return nil, fmt.Errorf("SARC: nome sem terminador")
			}
			name = strings.TrimPrefix(string(data[p:p+uint64(n)]), "/")
			if seen[name] {
				return nil, fmt.Errorf("SARC: nome duplicado")
			}
			seen[name] = true
		}
		out.Entries = append(out.Entries, Entry{Name: name, Node: at, Start: off + start, End: off + end})
	}
	return out, nil
}
func (s *SARC) Get(name string) ([]byte, bool) {
	for _, e := range s.Entries {
		if e.Name == name {
			return s.Data[e.Start:e.End], true
		}
	}
	return nil, false
}

// Replace preserves names, ordering, alignment and untouched payloads. Larger
// replacements are appended at 4 KiB alignment; the caller must grow the RSTB.
func (s *SARC) Replace(name string, d []byte) error {
	for i, e := range s.Entries {
		if e.Name != name {
			continue
		}
		start := e.Start
		if len(d) <= int(e.End-e.Start) {
			copy(s.Data[e.Start:e.End], d)
			clear(s.Data[int(e.Start)+len(d) : e.End])
		} else {
			for len(s.Data)%4096 != 0 {
				s.Data = append(s.Data, 0)
			}
			start = uint32(len(s.Data))
			if uint64(len(s.Data))+uint64(len(d)) > MaxData {
				return fmt.Errorf("SARC: saída muito grande")
			}
			s.Data = append(s.Data, d...)
		}
		end := start + uint32(len(d))
		s.e.PutUint32(s.Data[e.Node+8:], start-s.dataOff)
		s.e.PutUint32(s.Data[e.Node+12:], end-s.dataOff)
		s.e.PutUint32(s.Data[8:], uint32(len(s.Data)))
		s.Entries[i].Start = start
		s.Entries[i].End = end
		return nil
	}
	return fmt.Errorf("SARC: entrada %s não encontrada", name)
}
func WriteSARC(files map[string][]byte) ([]byte, error) {
	if len(files) > 65535 {
		return nil, fmt.Errorf("SARC: muitas entradas")
	}
	type named struct {
		name string
		hash uint32
	}
	var names []named
	for n := range files {
		if n == "" || strings.Contains(n, "\x00") {
			return nil, fmt.Errorf("SARC: nome inválido")
		}
		var h uint32
		for _, c := range []byte(n) {
			h = h*0x65 + uint32(c)
		}
		names = append(names, named{n, h})
	}
	sort.Slice(names, func(i, j int) bool {
		if names[i].hash == names[j].hash {
			return names[i].name < names[j].name
		}
		return names[i].hash < names[j].hash
	})
	d := make([]byte, 0x20+16*len(names)+8)
	copy(d, "SARC")
	e := binary.LittleEndian
	e.PutUint16(d[4:], 0x14)
	e.PutUint16(d[6:], 0xfeff)
	e.PutUint16(d[16:], 0x100)
	copy(d[0x14:], "SFAT")
	e.PutUint16(d[0x18:], 12)
	e.PutUint16(d[0x1a:], uint16(len(names)))
	e.PutUint32(d[0x1c:], 0x65)
	sfnt := 0x20 + 16*len(names)
	copy(d[sfnt:], "SFNT")
	e.PutUint16(d[sfnt+4:], 8)
	for i, n := range names {
		for len(d)%4 != 0 {
			d = append(d, 0)
		}
		at := 0x20 + 16*i
		e.PutUint32(d[at:], n.hash)
		e.PutUint32(d[at+4:], 0x01000000|uint32(len(d)-sfnt-8)/4)
		d = append(d, []byte(n.name)...)
		d = append(d, 0)
	}
	for len(d)%4096 != 0 {
		d = append(d, 0)
	}
	dataOff := len(d)
	e.PutUint32(d[12:], uint32(dataOff))
	for i, n := range names {
		for len(d)%4096 != 0 {
			d = append(d, 0)
		}
		at := 0x20 + 16*i
		e.PutUint32(d[at+8:], uint32(len(d)-dataOff))
		d = append(d, files[n.name]...)
		e.PutUint32(d[at+12:], uint32(len(d)-dataOff))
	}
	if len(d) > MaxData {
		return nil, fmt.Errorf("SARC: saída muito grande")
	}
	e.PutUint32(d[8:], uint32(len(d)))
	return d, nil
}
