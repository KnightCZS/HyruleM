package nativebridge

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"testing"
)

type fixtureMemory struct {
	values map[uint64][]byte
	reads  []uint64
}

func (m *fixtureMemory) Read(_ context.Context, a uint64, n int) ([]byte, error) {
	m.reads = append(m.reads, a)
	v := m.values[a]
	if len(v) != n {
		return nil, fmt.Errorf("unmapped fixture address %x/%d", a, n)
	}
	return append([]byte{}, v...), nil
}
func pointerBytes(a uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, a)
	return b
}
func instructionBytes(words ...uint32) []byte {
	b := make([]byte, 4*len(words))
	for i, w := range words {
		binary.LittleEndian.PutUint32(b[4*i:], w)
	}
	return b
}
func nativeFixture() (nativeSiteMap, *fixtureMemory) {
	s := nativeSiteMap{ModuleSHA256: "synthetic-module", MainBase: "0x7000000000", TextSize: 0x5000, Functions: map[string]functionSite{}, Globals: map[string]string{"player_info": "0x7000008000", "actor_creator": "0x7000008010"}, Offsets: map[string]uint64{"player_info_actor": 0x60, "actor_life_vtable_slot": 0x2b8, "creator_list_count": 0x48, "creator_block_spawns": 0x59}}
	m := &fixtureMemory{values: map[uint64][]byte{}}
	// Owned synthetic NOP/RET fixtures, not extracted BOTW code.
	for i, name := range []string{"actor_create_sync", "actor_request_async", "player_life_get"} {
		a := uint64(0x7000001000 + i*0x100)
		b := instructionBytes(0xd503201f, 0xd65f03c0)
		hash := sha256.Sum256(b)
		s.Functions[name] = functionSite{Address: fmt.Sprintf("0x%x", a), Size: len(b), SHA256: hex.EncodeToString(hash[:])}
		m.values[a] = b
	}
	m.values[0x7000008000] = pointerBytes(0x7100001000)
	m.values[0x7100001060] = pointerBytes(0x7100002000)
	m.values[0x7100002000] = pointerBytes(0x7000010000)
	m.values[0x70000102b8] = pointerBytes(0x7000003000)
	m.values[0x7000003000] = instructionBytes(0x91400400, 0x91010000, 0xd65f03c0) // +0x1000 +0x40, RET
	m.values[0x7100003040] = instructionBytes(12)
	m.values[0x7000008010] = pointerBytes(0x7100005000)
	creator := make([]byte, 20)
	binary.LittleEndian.PutUint32(creator, 5)
	binary.LittleEndian.PutUint32(creator[4:], 0x7b8)
	m.values[0x7100005048] = creator
	return s, m
}
func TestNativeInspectionChecksBuildCodeAndReadsOwnObjects(t *testing.T) {
	s, m := nativeFixture()
	r := inspectWithMap(context.Background(), s.ModuleSHA256, s, m)
	if r.Error != "" || !r.FunctionsVerified || r.PlayerActor != "0x7100002000" || r.LifeQuarterHearts == nil || *r.LifeQuarterHearts != 12 || r.RegisteredActors == nil || *r.RegisteredActors != 5 || r.SpawnsBlocked == nil || *r.SpawnsBlocked {
		t.Fatal(r)
	}
	s, m = nativeFixture()
	m.values[0x7000001000][0] ^= 1
	r = inspectWithMap(context.Background(), s.ModuleSHA256, s, m)
	if r.Error == "" || r.FunctionsVerified || len(m.reads) != 1 {
		t.Fatal("tampered code followed as pointers", r, m.reads)
	}
	s, m = nativeFixture()
	r = inspectWithMap(context.Background(), "other-build", s, m)
	if r.Status != "skipped" || len(m.reads) != 0 {
		t.Fatal("wrong module was inspected", r, m.reads)
	}
	s, m = nativeFixture()
	m.values[0x7000008000] = make([]byte, 8)
	r = inspectWithMap(context.Background(), s.ModuleSHA256, s, m)
	if r.Error == "" || r.PlayerActor != "" || len(m.reads) != 4 {
		t.Fatal("followed null player info", r, m.reads)
	}
}
func TestLifeGetterDecoderRejectsCallsAndForeignRegisters(t *testing.T) {
	for _, words := range [][]uint32{{0x94000000, 0xd65f03c0}, {0x91010001, 0xd65f03c0}, {0x91010020, 0xd65f03c0}, {0xd65f03c0}, {0x91010000, 0xd65f03c0}} {
		if _, ok := decodeLifeOffset(instructionBytes(words...)); ok {
			t.Fatal("unsafe getter accepted", words)
		}
	}
	if off, ok := decodeLifeOffset(instructionBytes(0x91400400, 0x91010000, 0xd65f03c0)); !ok || off != 0x1040 {
		t.Fatal("pure getter not decoded", off, ok)
	}
}
func TestUnknownNativeModuleDoesNotIssueMemoryCommands(t *testing.T) {
	m := &fixtureMemory{values: map[uint64][]byte{}}
	r := inspectNative(context.Background(), "unknown-module", m)
	if r.Status != "skipped" || len(m.reads) != 0 || r.FunctionsVerified {
		t.Fatal(r, m.reads)
	}
}
