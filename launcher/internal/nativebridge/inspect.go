package nativebridge

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	core "hyrulelauncher/internal/launcher"
	"strconv"
)

//go:embed native_190_sites.json
var native190JSON []byte

type functionSite struct {
	Address string `json:"address"`
	Size    int    `json:"size"`
	SHA256  string `json:"sha256"`
}
type nativeSiteMap struct {
	ModuleSHA256 string                  `json:"module_sha256"`
	MainBase     string                  `json:"main_base"`
	TextSize     uint64                  `json:"text_size"`
	Functions    map[string]functionSite `json:"functions"`
	Globals      map[string]string       `json:"globals"`
	Offsets      map[string]uint64       `json:"offsets"`
}
type NativeInspection struct {
	Status                string  `json:"status"`
	Error                 string  `json:"error,omitempty"`
	FunctionsVerified     bool    `json:"functions_verified"`
	PlayerInfo            string  `json:"player_info,omitempty"`
	PlayerActor           string  `json:"player_actor,omitempty"`
	PlayerVTable          string  `json:"player_vtable,omitempty"`
	LifeGetter            string  `json:"life_getter,omitempty"`
	LifeQuarterHearts     *int32  `json:"life_quarter_hearts,omitempty"`
	ActorCreator          string  `json:"actor_creator,omitempty"`
	RegisteredActors      *uint32 `json:"registered_actors,omitempty"`
	CreatorListNodeOffset uint32  `json:"creator_list_node_offset,omitempty"`
	SpawnsBlocked         *bool   `json:"spawns_blocked,omitempty"`
}
type memoryReader interface {
	Read(context.Context, uint64, int) ([]byte, error)
}
type controlMemory struct{ client core.ControlClient }

func guestPointer(a uint64) bool         { return a >= 0x7000000000 && a < 0x8000000000 && a%4 == 0 }
func addressOf(s string) (uint64, error) { return strconv.ParseUint(s, 0, 64) }
func (m controlMemory) Read(ctx context.Context, address uint64, size int) ([]byte, error) {
	if !guestPointer(address) || size < 1 || size > 512 || address+uint64(size) >= 0x8000000000 {
		return nil, fmt.Errorf("leitura fora do intervalo limitado")
	}
	r, err := m.client.Command(ctx, fmt.Sprintf("mem read 0x%x %d", address, size))
	if err != nil {
		return nil, err
	}
	var data struct {
		Hex   string `json:"hex"`
		Bytes int    `json:"bytes"`
	}
	if err = json.Unmarshal(r.Data, &data); err != nil {
		return nil, err
	}
	b, err := hex.DecodeString(data.Hex)
	if err != nil || data.Bytes != size || len(b) != size {
		return nil, fmt.Errorf("resposta de leitura inválida")
	}
	return b, nil
}
func readPointer(ctx context.Context, m memoryReader, address uint64) (uint64, error) {
	if !guestPointer(address) || address%8 != 0 {
		return 0, fmt.Errorf("origem do ponteiro inválida")
	}
	b, err := m.Read(ctx, address, 8)
	if err != nil {
		return 0, err
	}
	if len(b) != 8 {
		return 0, fmt.Errorf("ponteiro incompleto")
	}
	p := binary.LittleEndian.Uint64(b)
	if !guestPointer(p) || p%8 != 0 {
		return 0, fmt.Errorf("objeto nativo ausente ou ponteiro inválido")
	}
	return p, nil
}

// Accept only a pure getter: one/two ADD X0,X0,#imm and RET. Never execute it.
func decodeLifeOffset(code []byte) (uint64, bool) {
	var offset uint64
	for i := 0; i < 3 && i*4+4 <= len(code); i++ {
		w := binary.LittleEndian.Uint32(code[i*4:])
		if w == 0xd65f03c0 {
			return offset, i > 0 && offset >= 0x840 && offset < 0x10000 && offset%4 == 0
		}
		if i == 2 || w&0xff8003ff != 0x91000000 {
			return 0, false
		}
		imm := uint64((w >> 10) & 0xfff)
		if w&(1<<22) != 0 {
			imm <<= 12
		}
		offset += imm
	}
	return 0, false
}

// This inspects only a version-locked set of live pointers. It cannot create
// actors, read inventory, write memory or enable native multiplayer capabilities.
func inspectNative(ctx context.Context, moduleSHA string, memory memoryReader) *NativeInspection {
	var sites nativeSiteMap
	if err := json.Unmarshal(native190JSON, &sites); err != nil {
		return &NativeInspection{Status: "error", Error: err.Error()}
	}
	return inspectWithMap(ctx, moduleSHA, sites, memory)
}
func inspectWithMap(ctx context.Context, moduleSHA string, sites nativeSiteMap, memory memoryReader) *NativeInspection {
	out := &NativeInspection{Status: "read_only"}
	fail := func(err error) *NativeInspection { out.Error = err.Error(); return out }
	if moduleSHA != sites.ModuleSHA256 {
		out.Status = "skipped"
		out.Error = "módulo diferente do build analisado; nenhuma leitura de ponteiro executada"
		return out
	}
	base, err := addressOf(sites.MainBase)
	if err != nil {
		return fail(err)
	}
	// Disk metadata is insufficient: confirm the code this process actually loaded.
	for _, name := range []string{"actor_create_sync", "actor_request_async", "player_life_get"} {
		f := sites.Functions[name]
		a, err := addressOf(f.Address)
		if err != nil || f.Size < 4 || f.Size > 512 || a < base || a+uint64(f.Size) > base+sites.TextSize {
			return fail(fmt.Errorf("mapa estático inválido: %s", name))
		}
		b, err := memory.Read(ctx, a, f.Size)
		if err != nil {
			return fail(fmt.Errorf("não foi possível verificar %s: %w", name, err))
		}
		h := sha256.Sum256(b)
		if hex.EncodeToString(h[:]) != f.SHA256 {
			return fail(fmt.Errorf("código em execução difere do módulo analisado: %s", name))
		}
	}
	out.FunctionsVerified = true
	infoGlobal, _ := addressOf(sites.Globals["player_info"])
	info, err := readPointer(ctx, memory, infoGlobal)
	if err != nil {
		return fail(err)
	}
	out.PlayerInfo = fmt.Sprintf("0x%x", info)
	actor, err := readPointer(ctx, memory, info+sites.Offsets["player_info_actor"])
	if err != nil {
		return fail(err)
	}
	out.PlayerActor = fmt.Sprintf("0x%x", actor)
	vtable, err := readPointer(ctx, memory, actor)
	if err != nil {
		return fail(err)
	}
	out.PlayerVTable = fmt.Sprintf("0x%x", vtable)
	getter, err := readPointer(ctx, memory, vtable+sites.Offsets["actor_life_vtable_slot"])
	if err != nil {
		return fail(err)
	}
	if getter < base || getter+12 > base+sites.TextSize {
		return fail(fmt.Errorf("getter de vida fora do módulo confirmado"))
	}
	out.LifeGetter = fmt.Sprintf("0x%x", getter)
	code, err := memory.Read(ctx, getter, 12)
	if err != nil {
		return fail(err)
	}
	if offset, ok := decodeLifeOffset(code); ok {
		b, err := memory.Read(ctx, actor+offset, 4)
		if err != nil {
			return fail(err)
		}
		if len(b) != 4 {
			return fail(fmt.Errorf("vida incompleta"))
		}
		life := int32(binary.LittleEndian.Uint32(b))
		if life < 0 || life > 1000 {
			return fail(fmt.Errorf("vida fora do limite de diagnóstico"))
		}
		out.LifeQuarterHearts = &life
	}
	creatorGlobal, _ := addressOf(sites.Globals["actor_creator"])
	creator, err := readPointer(ctx, memory, creatorGlobal)
	if err != nil {
		return fail(err)
	}
	out.ActorCreator = fmt.Sprintf("0x%x", creator)
	start := sites.Offsets["creator_list_count"]
	b, err := memory.Read(ctx, creator+start, 20)
	if err != nil {
		return fail(err)
	}
	if len(b) != 20 {
		return fail(fmt.Errorf("estrutura de atores incompleta"))
	}
	count := binary.LittleEndian.Uint32(b)
	if count > 20000 {
		return fail(fmt.Errorf("contagem de atores inválida"))
	}
	out.RegisteredActors = &count
	out.CreatorListNodeOffset = binary.LittleEndian.Uint32(b[4:])
	flagOffset := sites.Offsets["creator_block_spawns"]
	if flagOffset < start || flagOffset-start >= uint64(len(b)) {
		return fail(fmt.Errorf("offset de controle inválido"))
	}
	blocked := b[flagOffset-start] != 0
	out.SpawnsBlocked = &blocked
	return out
}
