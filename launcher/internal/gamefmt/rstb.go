package gamefmt

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"sort"
	"strings"
)

type RSTB struct {
	Hashes map[uint32]uint32
	Names  map[string]uint32
}

func ReadRSTB(d []byte) (*RSTB, error) {
	if len(d) < 12 || string(d[:4]) != "RSTB" {
		return nil, fmt.Errorf("RSTB: assinatura inválida")
	}
	e := binary.LittleEndian
	a, b := e.Uint32(d[4:]), e.Uint32(d[8:])
	size := uint64(12) + uint64(a)*8 + uint64(b)*132
	if size > uint64(len(d)) || a > 1000000 || b > 100000 {
		return nil, fmt.Errorf("RSTB: tabela truncada ou incompatível com Switch")
	}
	r := &RSTB{Hashes: map[uint32]uint32{}, Names: map[string]uint32{}}
	for i := uint32(0); i < a; i++ {
		at := 12 + i*8
		h, v := e.Uint32(d[at:]), e.Uint32(d[at+4:])
		if _, ok := r.Hashes[h]; ok {
			return nil, fmt.Errorf("RSTB: hash duplicado")
		}
		r.Hashes[h] = v
	}
	for i := uint32(0); i < b; i++ {
		at := 12 + a*8 + i*132
		name := d[at : at+128]
		n := bytes.IndexByte(name, 0)
		if n < 0 {
			return nil, fmt.Errorf("RSTB: nome inválido")
		}
		r.Names[string(name[:n])] = e.Uint32(d[at+128:])
	}
	return r, nil
}
func Canonical(path string) string {
	i := strings.LastIndexByte(path, '.')
	if i >= 0 && i+2 < len(path) && path[i+1] == 's' {
		return path[:i+1] + path[i+2:]
	}
	return path
}
func (r *RSTB) Ensure(path string, uncompressed int) error {
	if uncompressed < 0 || uncompressed > MaxData {
		return fmt.Errorf("RSTB: recurso muito grande")
	}
	path = Canonical(path)
	budget := uint32((uncompressed+31)&^31) + 0x10000
	if v, ok := r.Names[path]; ok {
		if v < budget {
			r.Names[path] = budget
		}
		return nil
	}
	h := crc32.ChecksumIEEE([]byte(path))
	if r.Hashes[h] < budget {
		r.Hashes[h] = budget
	}
	return nil
}
func (r *RSTB) Bytes() []byte {
	hashes := make([]uint32, 0, len(r.Hashes))
	for h := range r.Hashes {
		hashes = append(hashes, h)
	}
	sort.Slice(hashes, func(i, j int) bool { return hashes[i] < hashes[j] })
	names := make([]string, 0, len(r.Names))
	for s := range r.Names {
		names = append(names, s)
	}
	sort.Strings(names)
	d := make([]byte, 12+8*len(hashes)+132*len(names))
	e := binary.LittleEndian
	copy(d, "RSTB")
	e.PutUint32(d[4:], uint32(len(hashes)))
	e.PutUint32(d[8:], uint32(len(names)))
	for i, h := range hashes {
		e.PutUint32(d[12+i*8:], h)
		e.PutUint32(d[16+i*8:], r.Hashes[h])
	}
	for i, s := range names {
		at := 12 + 8*len(hashes) + 132*i
		copy(d[at:at+128], s)
		e.PutUint32(d[at+128:], r.Names[s])
	}
	return d
}
