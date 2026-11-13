// Package gamefmt reads the documented BOTW data formats. It contains no game
// assets. Parsers reject out-of-range references before allocating or editing.
package gamefmt

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"unicode/utf8"
)

const MaxData = 512 << 20
const (
	Array  byte = 0xc0
	Map    byte = 0xc1
	String byte = 0xa0
	Bool   byte = 0xd0
	Int    byte = 0xd1
	Float  byte = 0xd2
	Uint   byte = 0xd3
	Null   byte = 0xff
)

type Node struct {
	Kind   byte
	Bits   uint64
	Text   string
	List   []*Node
	Fields map[string]*Node
	Blob   []byte
}

func Text(s string) *Node { return &Node{Kind: String, Text: s} }
func Boolean(v bool) *Node {
	var b uint64
	if v {
		b = 1
	}
	return &Node{Kind: Bool, Bits: b}
}
func Arr(v ...*Node) *Node          { return &Node{Kind: Array, List: v} }
func Dict(v map[string]*Node) *Node { return &Node{Kind: Map, Fields: v} }
func (n *Node) Get(k string) *Node {
	if n == nil {
		return nil
	}
	return n.Fields[k]
}
func (n *Node) Str() string {
	if n == nil || n.Kind != String {
		return ""
	}
	return n.Text
}
func (n *Node) CloneMap() *Node {
	out := Dict(map[string]*Node{})
	for k, v := range n.Fields {
		out.Fields[k] = v
	}
	return out
}

type Document struct {
	Root    *Node
	Little  bool
	Version uint16
}
type byReader struct {
	d             []byte
	e             binary.ByteOrder
	keys, strings []string
	cache         map[uint32]*Node
	visiting      map[uint32]bool
	count         int
}

func (p *byReader) rangeOK(o, n uint64) bool { return o <= uint64(len(p.d)) && n <= uint64(len(p.d))-o }
func (p *byReader) u32(o uint32) (uint32, error) {
	if !p.rangeOK(uint64(o), 4) {
		return 0, fmt.Errorf("BYML: referência fora do arquivo")
	}
	return p.e.Uint32(p.d[o:]), nil
}
func (p *byReader) u24(b []byte) uint32 {
	if p.e == binary.LittleEndian {
		return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
	}
	return uint32(b[2]) | uint32(b[1])<<8 | uint32(b[0])<<16
}
func (p *byReader) table(o uint32) ([]string, error) {
	if o == 0 {
		return nil, nil
	}
	if !p.rangeOK(uint64(o), 4) || p.d[o] != 0xc2 {
		return nil, fmt.Errorf("BYML: tabela de strings inválida")
	}
	n := p.u24(p.d[o+1 : o+4])
	if n > 1000000 || !p.rangeOK(uint64(o)+4, uint64(n+1)*4) {
		return nil, fmt.Errorf("BYML: tabela de strings truncada")
	}
	out := make([]string, n)
	for i := uint32(0); i < n; i++ {
		rel, _ := p.u32(o + 4 + i*4)
		a := uint64(o) + uint64(rel)
		if !p.rangeOK(a, 1) {
			return nil, fmt.Errorf("BYML: string fora do arquivo")
		}
		end := bytes.IndexByte(p.d[a:], 0)
		if end < 0 || end > 65535 {
			return nil, fmt.Errorf("BYML: string inválida")
		}
		b := p.d[a : a+uint64(end)]
		if !utf8.Valid(b) {
			return nil, fmt.Errorf("BYML: string não UTF-8")
		}
		out[i] = string(b)
	}
	return out, nil
}
func ReadBYML(d []byte) (Document, error) {
	var out Document
	if len(d) < 16 || len(d) > MaxData {
		return out, fmt.Errorf("BYML: tamanho inválido")
	}
	p := &byReader{d: d, cache: map[uint32]*Node{}, visiting: map[uint32]bool{}}
	switch string(d[:2]) {
	case "YB":
		p.e = binary.LittleEndian
		out.Little = true
	case "BY":
		p.e = binary.BigEndian
	default:
		return out, fmt.Errorf("BYML: assinatura inválida")
	}
	out.Version = p.e.Uint16(d[2:4])
	if out.Version < 2 || out.Version > 3 {
		return out, fmt.Errorf("BYML: versão %d não suportada", out.Version)
	}
	var err error
	p.keys, err = p.table(p.e.Uint32(d[4:8]))
	if err != nil {
		return out, err
	}
	p.strings, err = p.table(p.e.Uint32(d[8:12]))
	if err != nil {
		return out, err
	}
	out.Root, err = p.node(p.e.Uint32(d[12:16]), 0)
	return out, err
}
func (p *byReader) value(kind byte, bits uint32, depth int) (*Node, error) {
	switch kind {
	case Array, Map:
		return p.node(bits, depth+1)
	case String:
		if int(bits) >= len(p.strings) {
			return nil, fmt.Errorf("BYML: índice de string inválido")
		}
		return Text(p.strings[bits]), nil
	case Bool, Int, Float, Uint, Null:
		return &Node{Kind: kind, Bits: uint64(bits)}, nil
	case 0xd4, 0xd5, 0xd6:
		if !p.rangeOK(uint64(bits), 8) {
			return nil, fmt.Errorf("BYML: valor de 64 bits truncado")
		}
		return &Node{Kind: kind, Bits: p.e.Uint64(p.d[bits:])}, nil
	case 0xa1:
		n, e := p.u32(bits)
		if e != nil || !p.rangeOK(uint64(bits)+4, uint64(n)) {
			return nil, fmt.Errorf("BYML: blob truncado")
		}
		return &Node{Kind: kind, Blob: append([]byte(nil), p.d[bits+4:uint64(bits)+4+uint64(n)]...)}, nil
	default:
		return nil, fmt.Errorf("BYML: tipo 0x%02x não suportado", kind)
	}
}
func (p *byReader) node(o uint32, depth int) (*Node, error) {
	if depth > 64 {
		return nil, fmt.Errorf("BYML: profundidade excessiva")
	}
	if p.visiting[o] {
		return nil, fmt.Errorf("BYML: ciclo de referências")
	}
	if n := p.cache[o]; n != nil {
		return n, nil
	}
	if !p.rangeOK(uint64(o), 4) {
		return nil, fmt.Errorf("BYML: nó truncado")
	}
	kind := p.d[o]
	count := p.u24(p.d[o+1 : o+4])
	if count > 1000000 || p.count > 1000000 {
		return nil, fmt.Errorf("BYML: muitos nós")
	}
	p.count += int(count)
	p.visiting[o] = true
	defer delete(p.visiting, o)
	n := &Node{Kind: kind}
	var err error
	switch kind {
	case Array:
		base := uint64(o) + 4 + (uint64(count)+3)&^uint64(3)
		if !p.rangeOK(base, uint64(count)*4) || !p.rangeOK(uint64(o)+4, uint64(count)) {
			return nil, fmt.Errorf("BYML: array truncado")
		}
		n.List = make([]*Node, count)
		for i := uint32(0); i < count; i++ {
			bits := p.e.Uint32(p.d[base+uint64(i)*4:])
			n.List[i], err = p.value(p.d[o+4+i], bits, depth)
			if err != nil {
				return nil, err
			}
		}
	case Map:
		if !p.rangeOK(uint64(o)+4, uint64(count)*8) {
			return nil, fmt.Errorf("BYML: mapa truncado")
		}
		n.Fields = make(map[string]*Node, count)
		for i := uint32(0); i < count; i++ {
			at := o + 4 + i*8
			k := p.u24(p.d[at : at+3])
			if int(k) >= len(p.keys) {
				return nil, fmt.Errorf("BYML: chave inválida")
			}
			key := p.keys[k]
			if _, ok := n.Fields[key]; ok {
				return nil, fmt.Errorf("BYML: chave duplicada")
			}
			n.Fields[key], err = p.value(p.d[at+3], p.e.Uint32(p.d[at+4:]), depth)
			if err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("BYML: contêiner esperado")
	}
	p.cache[o] = n
	return n, nil
}

type byWriter struct {
	d             []byte
	e             binary.ByteOrder
	keys, strings []string
	ki, si        map[string]uint32
	cache         map[*Node]uint32
	visiting      map[*Node]bool
}

func (w *byWriter) align(n int) {
	for len(w.d)%n != 0 {
		w.d = append(w.d, 0)
	}
}
func (w *byWriter) add(n int) uint32 {
	at := uint32(len(w.d))
	w.d = append(w.d, make([]byte, n)...)
	return at
}
func (w *byWriter) u24(o uint32, v uint32) {
	if w.e == binary.LittleEndian {
		w.d[o], w.d[o+1], w.d[o+2] = byte(v), byte(v>>8), byte(v>>16)
	} else {
		w.d[o], w.d[o+1], w.d[o+2] = byte(v>>16), byte(v>>8), byte(v)
	}
}
func (w *byWriter) table(values []string) uint32 {
	if len(values) == 0 {
		return 0
	}
	w.align(4)
	o := w.add(4 + 4*(len(values)+1))
	w.d[o] = 0xc2
	w.u24(o+1, uint32(len(values)))
	for i, s := range values {
		w.e.PutUint32(w.d[o+4+uint32(i)*4:], uint32(len(w.d))-o)
		w.d = append(w.d, []byte(s)...)
		w.d = append(w.d, 0)
	}
	w.e.PutUint32(w.d[o+4+uint32(len(values))*4:], uint32(len(w.d))-o)
	return o
}
func WriteBYML(doc Document) ([]byte, error) {
	w := &byWriter{d: make([]byte, 16), e: binary.LittleEndian, ki: map[string]uint32{}, si: map[string]uint32{}, cache: map[*Node]uint32{}, visiting: map[*Node]bool{}}
	if !doc.Little {
		w.e = binary.BigEndian
	}
	seen := map[*Node]bool{}
	var collect func(*Node, int) error
	collect = func(n *Node, depth int) error {
		if n == nil || depth > 64 {
			return fmt.Errorf("BYML: nó inválido")
		}
		if w.visiting[n] {
			return fmt.Errorf("BYML: ciclo")
		}
		if seen[n] {
			return nil
		}
		seen[n] = true
		w.visiting[n] = true
		defer delete(w.visiting, n)
		if n.Kind == String {
			if len(n.Text) > 65535 || !utf8.ValidString(n.Text) || bytes.IndexByte([]byte(n.Text), 0) >= 0 {
				return fmt.Errorf("BYML: string inválida")
			}
			w.si[n.Text] = 0
		}
		for k, v := range n.Fields {
			w.ki[k] = 0
			if e := collect(v, depth+1); e != nil {
				return e
			}
		}
		for _, v := range n.List {
			if e := collect(v, depth+1); e != nil {
				return e
			}
		}
		return nil
	}
	if e := collect(doc.Root, 0); e != nil {
		return nil, e
	}
	for k := range w.ki {
		w.keys = append(w.keys, k)
	}
	for s := range w.si {
		w.strings = append(w.strings, s)
	}
	sort.Strings(w.keys)
	sort.Strings(w.strings)
	for i, k := range w.keys {
		w.ki[k] = uint32(i)
	}
	for i, s := range w.strings {
		w.si[s] = uint32(i)
	}
	if doc.Little {
		copy(w.d, "YB")
	} else {
		copy(w.d, "BY")
	}
	ver := doc.Version
	if ver == 0 {
		ver = 2
	}
	w.e.PutUint16(w.d[2:], ver)
	ko := w.table(w.keys)
	so := w.table(w.strings)
	root, e := w.node(doc.Root)
	if e != nil {
		return nil, e
	}
	w.e.PutUint32(w.d[4:], ko)
	w.e.PutUint32(w.d[8:], so)
	w.e.PutUint32(w.d[12:], root)
	if len(w.d) > MaxData {
		return nil, fmt.Errorf("BYML: saída muito grande")
	}
	return w.d, nil
}
func (w *byWriter) val(n *Node) (uint32, error) {
	switch n.Kind {
	case Map, Array:
		return w.node(n)
	case String:
		return w.si[n.Text], nil
	case Bool, Int, Float, Uint, Null:
		return uint32(n.Bits), nil
	case 0xd4, 0xd5, 0xd6:
		w.align(8)
		o := w.add(8)
		w.e.PutUint64(w.d[o:], n.Bits)
		return o, nil
	case 0xa1:
		w.align(4)
		o := w.add(4)
		w.e.PutUint32(w.d[o:], uint32(len(n.Blob)))
		w.d = append(w.d, n.Blob...)
		return o, nil
	default:
		return 0, fmt.Errorf("BYML: tipo de saída não suportado")
	}
}
func (w *byWriter) node(n *Node) (uint32, error) {
	if o, ok := w.cache[n]; ok {
		return o, nil
	}
	w.align(4)
	switch n.Kind {
	case Array:
		if len(n.List) > 1000000 {
			return 0, fmt.Errorf("BYML: array muito grande")
		}
		o := w.add(4 + ((len(n.List) + 3) &^ 3) + 4*len(n.List))
		w.cache[n] = o
		w.d[o] = Array
		w.u24(o+1, uint32(len(n.List)))
		base := o + 4 + uint32((len(n.List)+3)&^3)
		for i, v := range n.List {
			w.d[o+4+uint32(i)] = v.Kind
			bits, e := w.val(v)
			if e != nil {
				return 0, e
			}
			w.e.PutUint32(w.d[base+uint32(i)*4:], bits)
		}
		return o, nil
	case Map:
		keys := make([]string, 0, len(n.Fields))
		for k := range n.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		o := w.add(4 + 8*len(keys))
		w.cache[n] = o
		w.d[o] = Map
		w.u24(o+1, uint32(len(keys)))
		for i, k := range keys {
			v := n.Fields[k]
			at := o + 4 + uint32(i)*8
			w.u24(at, w.ki[k])
			w.d[at+3] = v.Kind
			bits, e := w.val(v)
			if e != nil {
				return 0, e
			}
			w.e.PutUint32(w.d[at+4:], bits)
		}
		return o, nil
	default:
		return 0, fmt.Errorf("BYML: raiz precisa ser mapa ou array")
	}
}
