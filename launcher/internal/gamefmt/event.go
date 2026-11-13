package gamefmt

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"sort"
	"strings"
)

type flowWriter struct {
	d       []byte
	ptrs    []uint32
	strings map[string][]uint32
}

func (w *flowWriter) add(n int) uint32 {
	at := uint32(len(w.d))
	w.d = append(w.d, make([]byte, n)...)
	return at
}
func (w *flowWriter) align() {
	for len(w.d)%8 != 0 {
		w.d = append(w.d, 0)
	}
}
func (w *flowWriter) ptr(at, to uint32) {
	binary.LittleEndian.PutUint64(w.d[at:], uint64(to))
	w.ptrs = append(w.ptrs, at)
}
func (w *flowWriter) str(at uint32, s string) {
	w.strings[s] = append(w.strings[s], at)
	w.ptrs = append(w.ptrs, at)
}
func bit(s string, n int) int {
	at := len(s) - 1 - n/8
	if at < 0 {
		return 0
	}
	return int(s[at] >> uint(n%8) & 1)
}
func mismatch(a, b string) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n*8; i++ {
		if bit(a, i) != bit(b, i) {
			return i
		}
	}
	return -1
}

// Radix dictionaries associate insertion order with array indices. Bit zero is
// the least significant bit of the last UTF-8 byte, as in ore::ResDic.
func (w *flowWriter) dic(names []string) (uint32, error) {
	type node struct {
		s     string
		b     int
		child [2]int
	}
	nodes := []node{{b: -1}}
	for _, s := range names {
		if s == "" || strings.ContainsRune(s, 0) || len(s) > 65535 {
			return 0, fmt.Errorf("EVFL: nome inválido")
		}
		parent, cur := 0, nodes[0].child[0]
		for nodes[parent].b < nodes[cur].b {
			parent, cur = cur, nodes[cur].child[bit(s, nodes[cur].b)]
		}
		diff := mismatch(s, nodes[cur].s)
		if diff < 0 {
			return 0, fmt.Errorf("EVFL: nome duplicado")
		}
		parent, cur = 0, nodes[0].child[0]
		for nodes[parent].b < nodes[cur].b && nodes[cur].b < diff {
			parent, cur = cur, nodes[cur].child[bit(s, nodes[cur].b)]
		}
		idx := len(nodes)
		v := node{s: s, b: diff}
		v.child[bit(s, diff)] = idx
		v.child[bit(s, diff)^1] = cur
		nodes = append(nodes, v)
		if parent == 0 {
			nodes[0].child[0] = idx
		} else {
			nodes[parent].child[bit(s, nodes[parent].b)] = idx
		}
	}
	w.align()
	o := w.add(8 + 16*len(nodes))
	copy(w.d[o:], "DIC ")
	binary.LittleEndian.PutUint32(w.d[o+4:], uint32(len(names)))
	for i, n := range nodes {
		at := o + 8 + uint32(i)*16
		binary.LittleEndian.PutUint32(w.d[at:], uint32(n.b))
		binary.LittleEndian.PutUint16(w.d[at+4:], uint16(n.child[0]))
		binary.LittleEndian.PutUint16(w.d[at+6:], uint16(n.child[1]))
		w.str(at+8, n.s)
	}
	return o, nil
}

// NoopFlow runs one ordinary, one-frame EventSystemActor action. An empty
// event graph would produce zero actor bindings and BOTW rejects that setup.
func NoopFlow(name string, entries []string) ([]byte, error) {
	if name == "" || len(name) > 255 || len(entries) == 0 || len(entries) > 4096 {
		return nil, fmt.Errorf("EVFL: fluxo inválido")
	}
	entries = append([]string(nil), entries...)
	sort.Strings(entries)
	w := &flowWriter{d: make([]byte, 0x48), strings: map[string][]uint32{"": nil}}
	e := binary.LittleEndian
	copy(w.d, "BFEVFL\x00\x00")
	e.PutUint16(w.d[8:], 0x300)
	e.PutUint16(w.d[12:], 0xfeff)
	w.d[14] = 3
	e.PutUint16(w.d[0x20:], 1)
	rootArray := w.add(8)
	w.ptr(0x28, rootArray)
	rootDic, err := w.dic([]string{name})
	if err != nil {
		return nil, err
	}
	w.ptr(0x30, rootDic)
	emptyDic, _ := w.dic(nil)
	w.ptr(0x40, emptyDic)
	w.align()
	flow := w.add(0x48)
	w.ptr(rootArray, flow)
	e.PutUint16(w.d[0x16:], uint16(flow))
	copy(w.d[flow:], "EVFL")
	e.PutUint16(w.d[flow+0x10:], 1)
	e.PutUint16(w.d[flow+0x12:], 1)
	e.PutUint16(w.d[flow+0x16:], 1)
	e.PutUint16(w.d[flow+0x18:], uint16(len(entries)))
	w.str(flow+0x20, name)
	actor := w.add(0x38)
	w.ptr(flow+0x28, actor)
	w.str(actor, "EventSystemActor")
	w.str(actor+8, "")
	w.str(actor+16, "")
	e.PutUint16(w.d[actor+0x30:], 1)
	e.PutUint16(w.d[actor+0x34:], 0xffff)
	w.d[actor+0x36] = 1
	event := w.add(0x28)
	w.ptr(flow+0x30, event)
	w.str(event, "SandboxReturn")
	e.PutUint16(w.d[event+0xa:], 0xffff)
	entryDic, err := w.dic(entries)
	if err != nil {
		return nil, err
	}
	w.ptr(flow+0x38, entryDic)
	w.align()
	entryArray := w.add(0x20 * len(entries))
	w.ptr(flow+0x40, entryArray)
	actions := w.add(8)
	w.ptr(actor+0x18, actions)
	w.str(actions, "Demo_WaitFrame")
	w.align()
	container := w.add(32)
	w.ptr(event+0x10, container)
	w.d[container] = 1
	e.PutUint16(w.d[container+2:], 2)
	paramDic, _ := w.dic([]string{"Frame", "IsWaitFinish"})
	w.ptr(container+8, paramDic)
	w.align()
	frame := w.add(24)
	w.ptr(container+16, frame)
	w.d[frame] = 2
	e.PutUint16(w.d[frame+2:], 1)
	e.PutUint32(w.d[frame+16:], 1)
	wait := w.add(24)
	w.ptr(container+24, wait)
	w.d[wait] = 3
	e.PutUint16(w.d[wait+2:], 1)
	e.PutUint32(w.d[wait+16:], 0x80000001)
	w.align()
	pool := w.add(20)
	copy(w.d[pool:], "STR ")
	e.PutUint32(w.d[pool+16:], uint32(len(w.strings)-1))
	e.PutUint32(w.d[flow+4:], pool-flow)
	// Pointer references make pool ordering immaterial to the runtime. Keeping a
	// sorted order makes generated outputs reproducible for diagnostics/caching.
	texts := make([]string, 0, len(w.strings))
	for s := range w.strings {
		texts = append(texts, s)
	}
	sort.Strings(texts)
	var fileName uint32
	for _, s := range texts {
		if len(w.d)%2 != 0 {
			w.d = append(w.d, 0)
		}
		at := w.add(2)
		e.PutUint16(w.d[at:], uint16(len(s)))
		w.d = append(w.d, []byte(s)...)
		w.d = append(w.d, 0)
		for _, p := range w.strings[s] {
			e.PutUint64(w.d[p:], uint64(at))
		}
		if s == name {
			fileName = at + 2
		}
	}
	e.PutUint32(w.d[16:], fileName)
	dataEnd := len(w.d)
	w.align()
	reloc := w.add(40)
	copy(w.d[reloc:], "RELT")
	e.PutUint32(w.d[reloc+4:], reloc)
	e.PutUint32(w.d[reloc+8:], 1)
	e.PutUint32(w.d[reloc+28:], uint32(dataEnd))
	sort.Slice(w.ptrs, func(i, j int) bool { return w.ptrs[i] < w.ptrs[j] })
	e.PutUint32(w.d[reloc+36:], uint32(len(w.ptrs)))
	for _, p := range w.ptrs {
		at := w.add(8)
		e.PutUint32(w.d[at:], p)
		e.PutUint32(w.d[at+4:], 1)
	}
	e.PutUint32(w.d[24:], reloc)
	e.PutUint32(w.d[28:], uint32(len(w.d)))
	return w.d, nil
}

// NoopDemo is an original minimal AAMP descriptor, not a game file/template.
// It prevents a story's old camera, location, movie and world-pause settings
// from being applied before its one-frame replacement flow starts.
func NoopDemo() []byte {
	type param struct {
		name, value string
		kind        byte
	}
	ps := []param{{"StartType", "Seamless", 7}, {"EventMode", "FullPackage", 7}, {"Extension", "evfl", 7}, {"ProductFormat", "RealTime", 7}, {"NoEntryActors", "none", 7}, {"SaveType", "none", 7}, {"StartLocationType", "Player", 7}, {"SkipPolicy", "Auto", 7}, {"IsStartupDemo", "", 0}, {"IsStopChemical", "", 0}, {"WithTransAnimation", "", 0}, {"IsOverwritePlayerPos", "", 0}, {"IsMovePlayerEndPos", "", 0}}
	d := make([]byte, 0x34+12+8+8*len(ps))
	e := binary.LittleEndian
	copy(d, "AAMP")
	e.PutUint32(d[4:], 2)
	e.PutUint32(d[8:], 3)
	e.PutUint32(d[0x14:], 4)
	e.PutUint32(d[0x18:], 1)
	e.PutUint32(d[0x1c:], 1)
	e.PutUint32(d[0x20:], uint32(len(ps)))
	copy(d[0x30:], "xml\x00")
	root := 0x34
	obj := root + 12
	params := obj + 8
	e.PutUint32(d[root:], crc32.ChecksumIEEE([]byte("param_root")))
	e.PutUint32(d[root+8:], uint32((obj-root)/4)|1<<16)
	e.PutUint32(d[obj:], crc32.ChecksumIEEE([]byte("DemoSetting")))
	e.PutUint32(d[obj+4:], uint32((params-obj)/4)|uint32(len(ps))<<16)
	dataStart := len(d)
	for i, p := range ps {
		if p.kind != 0 {
			continue
		}
		at := params + 8*i
		value := len(d)
		d = append(d, 0, 0, 0, 0)
		e.PutUint32(d[at:], crc32.ChecksumIEEE([]byte(p.name)))
		e.PutUint32(d[at+4:], uint32((value-at)/4))
	}
	stringStart := len(d)
	for i, p := range ps {
		if p.kind != 7 {
			continue
		}
		at := params + 8*i
		value := len(d)
		d = append(d, make([]byte, 32)...)
		copy(d[value:], p.value)
		e.PutUint32(d[at:], crc32.ChecksumIEEE([]byte(p.name)))
		e.PutUint32(d[at+4:], 7<<24|uint32((value-at)/4))
	}
	e.PutUint32(d[0x24:], uint32(stringStart-dataStart))
	e.PutUint32(d[0x28:], uint32(len(d)-stringStart))
	e.PutUint32(d[12:], uint32(len(d)))
	return d
}

// FlowEntries reads names without decoding event actions. Internal sub-flow
// entry points need replacement too, even when they have no EventInfo record.
func FlowEntries(d []byte) (string, []string, error) {
	bad := func() (string, []string, error) { return "", nil, fmt.Errorf("EVFL: tabela de entradas inválida") }
	if len(d) < 0x48 || string(d[:8]) != "BFEVFL\x00\x00" {
		return bad()
	}
	e := binary.LittleEndian
	if e.Uint16(d[8:]) != 0x300 || e.Uint16(d[12:]) != 0xfeff || e.Uint16(d[0x20:]) != 1 {
		return bad()
	}
	fits := func(o, n uint64) bool { return o <= uint64(len(d)) && n <= uint64(len(d))-o }
	ptr := func(at uint64) (uint64, bool) {
		if !fits(at, 8) {
			return 0, false
		}
		o := e.Uint64(d[at:])
		return o, fits(o, 1)
	}
	str := func(o uint64) (string, bool) {
		if !fits(o, 2) {
			return "", false
		}
		n := uint64(e.Uint16(d[o:]))
		if n == 0 || !fits(o+2, n+1) || d[o+2+n] != 0 {
			return "", false
		}
		return string(d[o+2 : o+2+n]), true
	}
	a, ok := ptr(0x28)
	if !ok {
		return bad()
	}
	flow, ok := ptr(a)
	if !ok || !fits(flow, 0x48) || string(d[flow:flow+4]) != "EVFL" {
		return bad()
	}
	p, ok := ptr(flow + 0x20)
	if !ok {
		return bad()
	}
	name, ok := str(p)
	if !ok {
		return bad()
	}
	dic, ok := ptr(flow + 0x38)
	if !ok || !fits(dic, 24) || string(d[dic:dic+4]) != "DIC " {
		return bad()
	}
	count := uint64(e.Uint16(d[flow+0x18:]))
	if count == 0 || count > 4096 || uint64(e.Uint32(d[dic+4:])) != count || !fits(dic+24, count*16) {
		return bad()
	}
	entries := make([]string, 0, count)
	for i := uint64(0); i < count; i++ {
		p, ok := ptr(dic + 24 + i*16 + 8)
		if !ok {
			return bad()
		}
		entry, ok := str(p)
		if !ok {
			return bad()
		}
		entries = append(entries, entry)
	}
	return name, entries, nil
}
