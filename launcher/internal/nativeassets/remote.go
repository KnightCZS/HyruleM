// Package nativeassets builds private overlays from the player's own resources.
// No original resource is embedded in the executable or modified in place.
package nativeassets

import (
	"fmt"
	"hash/crc32"
	"hyrulelauncher/internal/gamefmt"
	"path/filepath"
	"sort"
	"strings"
)

const Actor = "HyruleM_RemoteLink"
const FallbackActor = "HyruleM_RemoteLink_Fallback"
const FallbackPackPath = "Actor/Pack/" + FallbackActor + ".sbactorpack"
const PackPath = "Actor/Pack/" + Actor + ".sbactorpack"
const InfoPath = "Actor/ActorInfo.product.sbyml"
const RSTBPath = "System/Resource/ResourceSizeTable.product.srsizetable"

// BuildRemote is experimental: resource validation does not prove that the
// engine can initialize a DemoNPC with Link's model and AS list.
func BuildRemote(title, actorInfo, resourceTable []byte) (map[string][]byte, error) {
	bg, err := gamefmt.ReadSARC(title)
	if err != nil {
		return nil, err
	}
	packed, ok := bg.Get("Actor/Pack/GameROMPlayer.sbactorpack")
	if !ok {
		return nil, fmt.Errorf("TitleBG sem GameROMPlayer")
	}
	plain, _, err := gamefmt.Unpack(packed)
	if err != nil {
		return nil, err
	}
	player, err := gamefmt.ReadSARC(plain)
	if err != nil {
		return nil, err
	}
	link, ok := player.Get("Actor/ActorLink/GameROMPlayer.bxml")
	if !ok {
		return nil, fmt.Errorf("ActorLink do jogador ausente")
	}
	sourceLink := link
	targets := map[string]string{
		"ASUser":      Actor,
		"ProfileUser": "DemoNPC", "AIProgramUser": "Dummy", "AIScheduleUser": "Dummy",
		"AttentionUser": "Dummy", "AwarenessUser": "Dummy", "BoneControlUser": "Dummy",
		"ChemicalUser": "Dummy", "ElinkUser": "Dummy", "SlinkUser": "Dummy", "XlinkUser": "Dummy",
		// A Player controller/damage handler may assume the concrete local Player
		// class. The presence test deliberately has no collision/damage component.
		"PhysicsUser": "Dummy", "DamageParamUser": "Dummy", "RgBlendWeightUser": "Dummy", "RgConfigListUser": "Dummy",
	}
	link, err = gamefmt.PatchAAMPStrings(sourceLink, "LinkTarget", targets)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	budgets := map[string]string{}
	var fallback []byte
	if wait, ok := player.Get("Actor/AS/Player_Wait.bas"); ok {
		fallback, err = gamefmt.ProxyAS(wait, "Wait")
		if err != nil {
			return nil, err
		}
	}
	for _, e := range player.Entries {
		if e.Name != "Actor/ActorLink/GameROMPlayer.bxml" {
			name := e.Name
			if name == "Actor/ASList/Player.baslist" {
				x, e2 := gamefmt.NamespaceASList(player.Data[e.Start:e.End], "HM_")
				if e2 != nil {
					return nil, e2
				}
				name = "Actor/ASList/" + Actor + ".baslist"
				files[name] = x
				budgets[name] = e.Name
				continue
			}
			files[name] = player.Data[e.Start:e.End]
			if strings.HasPrefix(e.Name, "Actor/AS/") && strings.HasSuffix(e.Name, ".bas") {
				action := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(e.Name), "Player_"), ".bas")
				proxy, e2 := gamefmt.ProxyAS(files[e.Name], action)
				if e2 != nil {
					if fallback == nil {
						return nil, e2
					}
					proxy = fallback
				}
				delete(files, e.Name)
				name = "Actor/AS/HM_" + filepath.Base(e.Name)
				files[name] = proxy
				budgets[name] = e.Name
				if e2 != nil {
					budgets[name] = "Actor/AS/Player_Wait.bas"
				}
			}
		}
	}
	linkPath := "Actor/ActorLink/" + Actor + ".bxml"
	files[linkPath] = link
	pack, err := gamefmt.WriteSARC(files)
	if err != nil {
		return nil, err
	}
	legacy := map[string][]byte{}
	for _, entry := range player.Entries {
		if entry.Name != "Actor/ActorLink/GameROMPlayer.bxml" {
			legacy[entry.Name] = player.Data[entry.Start:entry.End]
		}
	}
	targets["ASUser"] = "Player"
	legacyLink, err := gamefmt.PatchAAMPStrings(sourceLink, "LinkTarget", targets)
	if err != nil {
		return nil, err
	}
	legacyLinkPath := "Actor/ActorLink/" + FallbackActor + ".bxml"
	legacy[legacyLinkPath] = legacyLink
	legacyPack, err := gamefmt.WriteSARC(legacy)
	if err != nil {
		return nil, err
	}
	info, _, err := gamefmt.Unpack(actorInfo)
	if err != nil {
		return nil, err
	}
	originalInfoSize := len(info)
	doc, err := gamefmt.ReadBYML(info)
	if err != nil {
		return nil, err
	}
	actors, hashes := doc.Root.Get("Actors"), doc.Root.Get("Hashes")
	if actors == nil || hashes == nil || actors.Kind != gamefmt.Array || hashes.Kind != gamefmt.Array || len(actors.List) != len(hashes.List) {
		return nil, fmt.Errorf("ActorInfo: índice inválido")
	}
	var original *gamefmt.Node
	newHash := crc32.ChecksumIEEE([]byte(Actor))
	for i, a := range actors.List {
		if a.Get("name").Str() == "GameROMPlayer" {
			original = a
		}
		if a.Get("name").Str() == Actor || a.Get("name").Str() == FallbackActor || uint32(hashes.List[i].Bits) == newHash || uint32(hashes.List[i].Bits) == crc32.ChecksumIEEE([]byte(FallbackActor)) {
			return nil, fmt.Errorf("ActorInfo já contém nome/hash do ator remoto")
		}
	}
	if original == nil || original.Get("profile").Str() != "Player" || original.Get("bfres").Str() != "Link" {
		return nil, fmt.Errorf("ActorInfo do Link incompatível")
	}
	remote := original.CloneMap()
	remote.Fields["name"] = gamefmt.Text(Actor)
	remote.Fields["profile"] = gamefmt.Text("DemoNPC")
	// Player-specific sound/effect controllers must not attach to the local player.
	for _, key := range []string{"elink", "slink", "xlink"} {
		if remote.Get(key) != nil {
			remote.Fields[key] = gamefmt.Text("Dummy")
		}
	}
	var simpleModel []byte
	if data, ok := bg.Get("Actor/Pack/Armor_Default_Extra_00.sbactorpack"); ok {
		plain, _, e := gamefmt.Unpack(data)
		if e != nil {
			return nil, e
		}
		s, e := gamefmt.ReadSARC(plain)
		if e != nil {
			return nil, e
		}
		simpleModel, _ = s.Get("Actor/ModelList/Armor_Default_Extra_00.bmodellist")
	}
	equipmentPacks, equipmentRows, err := equipmentResources(player, simpleModel, files, link, actors.List, remote)
	if err != nil {
		return nil, err
	}
	type entry struct {
		hash  uint32
		actor *gamefmt.Node
		kind  byte
	}
	entries := make([]entry, 0, len(actors.List)+1)
	for i, a := range actors.List {
		if hashes.List[i].Kind != gamefmt.Uint && hashes.List[i].Kind != gamefmt.Int {
			return nil, fmt.Errorf("ActorInfo: hash não inteiro")
		}
		entries = append(entries, entry{uint32(hashes.List[i].Bits), a, hashes.List[i].Kind})
	}
	entries = append(entries, entry{newHash, remote, gamefmt.Uint})
	legacyInfo := remote.CloneMap()
	legacyInfo.Fields["name"] = gamefmt.Text(FallbackActor)
	entries = append(entries, entry{crc32.ChecksumIEEE([]byte(FallbackActor)), legacyInfo, gamefmt.Uint})
	knownHashes := map[uint32]bool{}
	for _, e := range entries {
		knownHashes[e.hash] = true
	}
	for _, row := range equipmentRows {
		h := crc32.ChecksumIEEE([]byte(row.Get("name").Str()))
		if knownHashes[h] {
			return nil, fmt.Errorf("colisão de ator de equipamento")
		}
		knownHashes[h] = true
		entries = append(entries, entry{h, row, gamefmt.Uint})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].hash < entries[j].hash })
	actors.List, hashes.List = nil, nil
	for _, e := range entries {
		actors.List = append(actors.List, e.actor)
		hashes.List = append(hashes.List, &gamefmt.Node{Kind: e.kind, Bits: uint64(e.hash)})
	}
	info, err = gamefmt.WriteBYML(doc)
	if err != nil {
		return nil, err
	}
	rawTable, _, err := gamefmt.Unpack(resourceTable)
	if err != nil {
		return nil, err
	}
	table, err := gamefmt.ReadRSTB(rawTable)
	if err != nil {
		return nil, err
	}
	for path, source := range budgets {
		old, _ := player.Get(source)
		if err = reserveProxyResource(table, path, source, len(files[path]), len(old), strings.HasPrefix(path, "Actor/AS/")); err != nil {
			return nil, err
		}
	}
	if err = reserveProxyResource(table, InfoPath, InfoPath, len(info), originalInfoSize, false); err != nil {
		return nil, err
	}
	for path, size := range map[string]int{PackPath: len(pack), FallbackPackPath: len(legacyPack), InfoPath: len(info), linkPath: len(link), legacyLinkPath: len(legacyLink)} {
		if err = table.Ensure(path, size); err != nil {
			return nil, err
		}
	}
	model, _ := player.Get("Actor/ModelList/Player_Link.bmodellist")
	if err = reserveEquipment(table, equipmentPacks, len(model)); err != nil {
		return nil, err
	}
	out := map[string][]byte{PackPath: gamefmt.PackYaz0(pack), FallbackPackPath: gamefmt.PackYaz0(legacyPack), InfoPath: gamefmt.PackYaz0(info), RSTBPath: gamefmt.PackYaz0(table.Bytes())}
	for path, data := range equipmentPacks {
		out[path] = data
	}
	return out, nil
}
