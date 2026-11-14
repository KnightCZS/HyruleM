package nativeassets

import (
	"fmt"
	"hash/crc32"
	f "hyrulelauncher/internal/gamefmt"
	"strings"
)

const EquipmentPrefix = "HyruleM_Equip_"

func visualEquipmentName(s string) bool {
	if len(s) == 0 || len(s) > 63-len(EquipmentPrefix) {
		return false
	}
	for _, c := range s {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return strings.HasPrefix(s, "Armor_") || strings.HasPrefix(s, "Weapon_Sword_") || strings.HasPrefix(s, "Weapon_Lsword_") || strings.HasPrefix(s, "Weapon_Spear_") || strings.HasPrefix(s, "Weapon_Bow_") || strings.HasPrefix(s, "Weapon_Shield_")
}

// Metadata supplies each model's archive/unit (which is not always its actor
// name). Derived packs contain visual controllers only and are generated on the
// player's PC; no game resources are shipped with HyruleM.
func equipmentResources(player *f.SARC, simpleModel []byte, mainFiles map[string][]byte, link []byte, actors []*f.Node, remote *f.Node) (map[string][]byte, []*f.Node, error) {
	model, ok := player.Get("Actor/ModelList/Player_Link.bmodellist")
	if !ok {
		return nil, nil, nil
	}
	shared := map[string][]byte{}
	for name, data := range mainFiles {
		if strings.HasPrefix(name, "Actor/AS/") || strings.HasPrefix(name, "Actor/ASList/") {
			shared[name] = data
		}
	}
	out := map[string][]byte{}
	var rows []*f.Node
	for _, source := range actors {
		name, folder, unit := source.Get("name").Str(), source.Get("bfres").Str(), source.Get("mainModel").Str()
		if !visualEquipmentName(name) || folder == "" || unit == "" {
			continue
		}
		if len(rows) >= 2048 {
			return nil, nil, fmt.Errorf("equipamentos excedem o limite")
		}
		actor := EquipmentPrefix + name
		weapon := strings.HasPrefix(name, "Weapon_")
		template := model
		if weapon {
			if len(simpleModel) == 0 {
				continue
			}
			template = simpleModel
		}
		modelData, err := f.PatchModelResource(template, folder, unit)
		if err != nil {
			return nil, nil, err
		}
		targets := map[string]string{"ModelUser": actor, "GParamUser": "Dummy", "AnimationInfo": "Dummy"}
		if weapon {
			targets["ASUser"] = "Dummy"
		}
		actorLink, err := f.PatchAAMPStrings(link, "LinkTarget", targets)
		if err != nil {
			return nil, nil, err
		}
		packFiles := map[string][]byte{"Actor/ActorLink/" + actor + ".bxml": actorLink, "Actor/ModelList/" + actor + ".bmodellist": modelData}
		// The shared paths are byte-identical across the body and all pieces.
		// Every pack is self-contained even when streaming loads a piece first.
		for path, data := range shared {
			if weapon {
				continue
			}
			packFiles[path] = data
		}
		pack, err := f.WriteSARC(packFiles)
		if err != nil {
			return nil, nil, err
		}
		out["Actor/Pack/"+actor+".sbactorpack"] = f.PackYaz0(pack)
		row := remote.CloneMap()
		row.Fields["name"] = f.Text(actor)
		row.Fields["bfres"] = f.Text(folder)
		row.Fields["mainModel"] = f.Text(unit)
		rows = append(rows, row)
	}
	return out, rows, nil
}

func reserveEquipment(table *f.RSTB, packs map[string][]byte, sourceModelSize int) error {
	seen := map[uint32]string{}
	for path, packed := range packs {
		plain, _, err := f.Unpack(packed)
		if err != nil {
			return err
		}
		s, err := f.ReadSARC(plain)
		if err != nil {
			return err
		}
		if err = table.Ensure(path, len(plain)); err != nil {
			return err
		}
		for _, e := range s.Entries {
			if !strings.HasPrefix(e.Name, "Actor/ModelList/HyruleM_Equip_") && !strings.HasPrefix(e.Name, "Actor/ActorLink/HyruleM_Equip_") {
				continue
			}
			h := crc32.ChecksumIEEE([]byte(f.Canonical(e.Name)))
			if old, ok := seen[h]; ok && old != e.Name {
				return fmt.Errorf("colisão de equipamento no RSTB")
			}
			seen[h] = e.Name
			if err = reserveProxyResource(table, e.Name, "Actor/ModelList/Player_Link.bmodellist", int(e.End-e.Start), sourceModelSize, false); err != nil {
				return err
			}
		}
	}
	return nil
}
