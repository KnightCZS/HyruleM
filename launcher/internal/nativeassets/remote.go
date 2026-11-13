// Package nativeassets builds private overlays from the player's own resources.
// No original resource is embedded in the executable or modified in place.
package nativeassets

import (
	"fmt"
	"hash/crc32"
	"hyrulelauncher/internal/gamefmt"
	"sort"
)

const Actor = "HyruleM_RemoteLink"
const PackPath = "Actor/Pack/" + Actor + ".sbactorpack"
const InfoPath = "Actor/ActorInfo.product.sbyml"
const RSTBPath = "System/Resource/ResourceSizeTable.product.srsizetable"

// BuildRemote is experimental: resource validation does not prove that the
// engine can initialize a DemoNPC with Link's model, AS list and physics.
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
	link, err = gamefmt.PatchAAMPStrings(link, "LinkTarget", map[string]string{
		"ProfileUser": "DemoNPC", "AIProgramUser": "Dummy", "AIScheduleUser": "Dummy",
		"AttentionUser": "Dummy", "AwarenessUser": "Dummy", "BoneControlUser": "Dummy",
		"ChemicalUser": "Dummy", "ElinkUser": "Dummy", "SlinkUser": "Dummy", "XlinkUser": "Dummy",
	})
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	for _, e := range player.Entries {
		if e.Name != "Actor/ActorLink/GameROMPlayer.bxml" {
			files[e.Name] = player.Data[e.Start:e.End]
		}
	}
	linkPath := "Actor/ActorLink/" + Actor + ".bxml"
	files[linkPath] = link
	pack, err := gamefmt.WriteSARC(files)
	if err != nil {
		return nil, err
	}
	info, _, err := gamefmt.Unpack(actorInfo)
	if err != nil {
		return nil, err
	}
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
		if a.Get("name").Str() == Actor || uint32(hashes.List[i].Bits) == newHash {
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
	for path, size := range map[string]int{PackPath: len(pack), InfoPath: len(info), linkPath: len(link)} {
		if err = table.Ensure(path, size); err != nil {
			return nil, err
		}
	}
	return map[string][]byte{PackPath: gamefmt.PackYaz0(pack), InfoPath: gamefmt.PackYaz0(info), RSTBPath: gamefmt.PackYaz0(table.Bytes())}, nil
}
