package launcher

// Sandbox saves are derived locally from a player's existing Switch save. The
// original is never written. No flag tables, saves or game assets are shipped.
import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Place struct {
	ID, Name string
	X, Y, Z  float32
}

var SandboxPlaces = []Place{
	{"hateno", "Vila Hateno", 3437.5, 235.9, 2101.5},
	{"kakariko", "Vila Kakariko", 1811.7, 218.5, 982.6},
	{"stable", "Estábulo da margem", 384.8, 121.7, 1035.6},
	{"plateau", "Grande Planalto", -1041.6, 250.2, 1812.1},
	{"rito", "Vila Rito", -3607.2, 286.3, -1805.5},
	{"desert", "Bazar Kara Kara", -3196, 129.4, 2549.3},
}

func FindPlace(id string) (Place, error) {
	for _, p := range SandboxPlaces {
		if p.ID == id {
			return p, nil
		}
	}
	return Place{}, fmt.Errorf("região do sandbox inválida")
}
func SandboxDir(c Config) string     { return filepath.Join(c.DataDir, "hyrule-sandbox") }
func SandboxSaveDir(c Config) string { return filepath.Join(SandboxDir(c), "save") }
func sandboxSlot(c Config) string    { return filepath.Join(SandboxSaveDir(c), "save_0", "0") }

type SandboxManifest struct {
	Schema          int
	Launcher        string
	Created         time.Time
	Source          string
	SourceSHA256    string
	Revision        uint32
	Place           string
	Patched         []string
	MissingOptional []string
	Backup          string `json:",omitempty"`
	CampaignRemoved bool   `json:",omitempty"`
}

type flagSave struct {
	data     []byte
	offsets  map[uint32][]int
	revision uint32
}

func parseFlagSave(data []byte) (*flagSave, error) {
	if len(data) < 24 || len(data) > 16*1024*1024 || (len(data)-16)%8 != 0 {
		return nil, fmt.Errorf("tamanho de save inválido")
	}
	le := binary.LittleEndian
	rev := le.Uint32(data[:4])
	if rev == 0 || rev >= 0x10000 {
		return nil, fmt.Errorf("o sandbox exige um save Switch; importe saves Wii U/Cemu no Savage Wind primeiro")
	}
	if le.Uint32(data[4:8]) != 0xffffffff || le.Uint32(data[8:12]) != 1 || le.Uint32(data[len(data)-4:]) != 0xffffffff {
		return nil, fmt.Errorf("cabeçalho ou rodapé de save inválido")
	}
	s := &flagSave{data: append([]byte(nil), data...), offsets: map[uint32][]int{}, revision: rev}
	var last uint32
	for off := 12; off < len(data)-4; off += 8 {
		h := le.Uint32(data[off:])
		if off > 12 && h < last {
			return nil, fmt.Errorf("tabela de save fora de ordem")
		}
		last = h
		s.offsets[h] = append(s.offsets[h], off+4)
	}
	return s, nil
}
func (s *flagSave) setWords(name string, words []uint32, required bool) (bool, error) {
	offsets := s.offsets[crc32.ChecksumIEEE([]byte(name))]
	if len(offsets) == 0 && !required {
		return false, nil
	}
	if len(offsets) != len(words) {
		return false, fmt.Errorf("save incompatível: %s tem %d palavras; esperado %d", name, len(offsets), len(words))
	}
	for i, o := range offsets {
		binary.LittleEndian.PutUint32(s.data[o:], words[i])
	}
	return true, nil
}
func (s *flagSave) setString(name, value string, required bool) (bool, error) {
	offsets := s.offsets[crc32.ChecksumIEEE([]byte(name))]
	if len(offsets) == 0 && !required {
		return false, nil
	}
	if len(offsets) != 8 && len(offsets) != 16 {
		return false, fmt.Errorf("save incompatível: tamanho de %s", name)
	}
	if len(value) >= 4*len(offsets) {
		return false, fmt.Errorf("texto excede %s", name)
	}
	raw := make([]byte, 4*len(offsets))
	copy(raw, []byte(value))
	ws := make([]uint32, len(offsets))
	for i := range ws {
		ws[i] = binary.LittleEndian.Uint32(raw[i*4:])
	}
	return s.setWords(name, ws, true)
}
func patchSandbox(s *flagSave, p Place, required bool) (patched, missing []string, err error) {
	put := func(name string, w []uint32, must bool) {
		if err != nil {
			return
		}
		var ok bool
		ok, err = s.setWords(name, w, must)
		if ok {
			patched = append(patched, name)
		} else if err == nil {
			missing = append(missing, name)
		}
	}
	put("PlayerSavePos", []uint32{math.Float32bits(p.X), math.Float32bits(p.Y), math.Float32bits(p.Z)}, required)
	for _, v := range [][2]string{{"PlayerSavePosMapType", "MainField"}, {"PlayerSavePosMapName", fmt.Sprintf("%c-%d", 'A'+int((p.X+5000)/1000), int((p.Z+4000)/1000)+1)}} {
		if err != nil {
			return
		}
		var ok bool
		ok, err = s.setString(v[0], v[1], required)
		if ok {
			patched = append(patched, v[0])
		} else if err == nil {
			missing = append(missing, v[0])
		}
	}
	put("IsGet_PlayerStole2", []uint32{1}, required)
	// Tools and tower flags are deliberately inherited from the seed. A future
	// server chooses progression; this launcher does not grant runes implicitly.
	put("FirstTouchdown", []uint32{1}, false)
	return
}
func readRegular(path string, limit int64) ([]byte, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > limit {
		return nil, fmt.Errorf("arquivo inválido ou muito grande: %s", filepath.Base(path))
	}
	return os.ReadFile(path)
}

// CampaignRoot follows the runtime's selected profile, without modifying it.
func CampaignRoot(c Config) (string, error) {
	b, err := readRegular(filepath.Join(c.DataDir, "save_profile.txt"), 4096)
	if os.IsNotExist(err) {
		return SaveDir(c), nil
	}
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(string(b))
	if name == "" || strings.EqualFold(name, "Default") {
		return SaveDir(c), nil
	}
	if name == "." || name == ".." || strings.ContainsAny(name, "/\\:\x00") || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return "", fmt.Errorf("nome de perfil de saves inválido")
	}
	profile := filepath.Join(c.DataDir, "save-profiles", name, "save")
	if !IsDir(profile) {
		return SaveDir(c), nil
	}
	return profile, nil
}
func FindSandboxSeed(c Config) (string, error) {
	if c.SandboxSeed != "" {
		if !filepath.IsAbs(c.SandboxSeed) || !strings.EqualFold(filepath.Base(c.SandboxSeed), "game_data.sav") {
			return "", fmt.Errorf("selecione um game_data.sav de Switch com caminho absoluto")
		}
		return c.SandboxSeed, nil
	}
	root, err := CampaignRoot(c)
	if err != nil {
		return "", err
	}
	type candidate struct {
		path string
		at   time.Time
	}
	var candidates []candidate
	for n := 0; n < 6; n++ {
		p := filepath.Join(root, "save_0", strconv.Itoa(n), "game_data.sav")
		if st, e := os.Lstat(p); e == nil && st.Mode().IsRegular() {
			candidates = append(candidates, candidate{p, st.ModTime()})
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("nenhum save Switch encontrado: jogue a campanha até criar um save, feche o jogo e tente novamente; ou escolha game_data.sav em Save base")
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].at.After(candidates[j].at) })
	return candidates[0].path, nil
}
func LoadSandbox(c Config) (SandboxManifest, error) {
	var m SandboxManifest
	b, err := readRegular(filepath.Join(SandboxDir(c), "sandbox.json"), 128*1024)
	if err != nil {
		return m, err
	}
	if err = json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	if m.Schema != 1 {
		return m, fmt.Errorf("versão de sandbox incompatível")
	}
	// Any one normal slot is enough after the player resumes/saves the game.
	for i := 0; i < 6; i++ {
		if IsFile(filepath.Join(SandboxSaveDir(c), "save_0", strconv.Itoa(i), "game_data.sav")) {
			return m, nil
		}
	}
	return m, fmt.Errorf("o sandbox não possui saves; use Novo sandbox para recriá-lo")
}

// PrepareSandbox stages a complete new save before publishing it. Rebuild keeps
// the old sandbox under a unique backup name; campaign files are read only.
func PrepareSandbox(c Config, rebuild bool) (SandboxManifest, error) {
	c.Normalize()
	var m SandboxManifest
	if c.Backend != Savage {
		return m, fmt.Errorf("o modo sandbox desta versão exige Savage Wind para Switch 1.9.0")
	}
	if c.DataDir == "" || !filepath.IsAbs(c.DataDir) {
		return m, fmt.Errorf("selecione a pasta de dados")
	}
	p, err := FindPlace(c.SandboxPlace)
	if err != nil {
		return m, err
	}
	target := SandboxDir(c)
	if st, e := os.Lstat(target); e == nil && (!st.IsDir() || st.Mode()&os.ModeSymlink != 0) {
		return m, fmt.Errorf("a pasta de sandbox precisa ser uma pasta real; o conteúdo existente foi preservado")
	}
	if _, err = os.Lstat(target); err == nil && !rebuild {
		m, err = LoadSandbox(c)
		if err != nil {
			return m, fmt.Errorf("sandbox existente preservado: %w", err)
		}
		return m, nil
	} else if err != nil && !os.IsNotExist(err) {
		return m, err
	}
	source, err := FindSandboxSeed(c)
	if err != nil {
		return m, err
	}
	sourceAbs, err := filepath.EvalSymlinks(source)
	if err != nil {
		return m, err
	}
	realParent, err := filepath.EvalSymlinks(c.DataDir)
	if err != nil {
		return m, err
	}
	targetAbs, err := filepath.Abs(filepath.Join(realParent, "hyrule-sandbox"))
	if err != nil {
		return m, err
	}
	// A sandbox is never its own seed, including an explicitly chosen slot.
	if rel, e := filepath.Rel(targetAbs, sourceAbs); e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return m, fmt.Errorf("use um save de campanha como base; o sandbox atual será preservado")
	}
	b, err := readRegular(source, 16*1024*1024)
	if err != nil {
		return m, err
	}
	save, err := parseFlagSave(b)
	if err != nil {
		return m, err
	}
	patched, missing, err := patchSandbox(save, p, true)
	if err != nil {
		return m, err
	}
	if err = os.MkdirAll(c.DataDir, 0700); err != nil {
		return m, err
	}
	stage, err := os.MkdirTemp(c.DataDir, ".sandbox-stage-")
	if err != nil {
		return m, err
	}
	defer os.RemoveAll(stage)
	slot := filepath.Join(stage, "save", "save_0", "0")
	if err = os.MkdirAll(slot, 0700); err != nil {
		return m, err
	}
	if err = os.WriteFile(filepath.Join(slot, "game_data.sav"), save.data, 0600); err != nil {
		return m, err
	}
	// Keep the matching caption, image and options; no tracker paths from another
	// region are imported. All copies are bounded, ordinary files only.
	for _, name := range []string{"caption.sav", "caption.jpg"} {
		data, e := readRegular(filepath.Join(filepath.Dir(source), name), 16*1024*1024)
		if os.IsNotExist(e) {
			if name == "caption.sav" {
				return m, fmt.Errorf("o save base precisa de caption.sav na mesma pasta; use um slot salvo pela recomp")
			}
			continue
		}
		if e != nil {
			return m, e
		}
		if name == "caption.sav" {
			cap, e := parseFlagSave(data)
			if e != nil {
				return m, fmt.Errorf("caption.sav inválido: %w", e)
			}
			if cap.revision != save.revision {
				return m, fmt.Errorf("revisões diferentes em caption e game_data")
			}
			if _, _, e = patchSandbox(cap, p, false); e != nil {
				return m, e
			}
			data = cap.data
		}
		if e = os.WriteFile(filepath.Join(slot, name), data, 0600); e != nil {
			return m, e
		}
	}
	// option.sav lives in save_0 next to the numbered slots.
	option := filepath.Join(filepath.Dir(filepath.Dir(source)), "option.sav")
	if data, e := readRegular(option, 16*1024*1024); e == nil {
		opt, e := parseFlagSave(data)
		if e != nil {
			return m, e
		}
		if opt.revision != save.revision {
			return m, fmt.Errorf("revisão de opções incompatível")
		}
		if e = os.WriteFile(filepath.Join(stage, "save", "save_0", "option.sav"), data, 0600); e != nil {
			return m, e
		}
	} else if !os.IsNotExist(e) {
		return m, e
	}
	// Detect a concurrently rewritten source rather than publishing a mixed copy.
	again, err := readRegular(source, 16*1024*1024)
	if err != nil {
		return m, err
	}
	if !bytes.Equal(b, again) {
		return m, fmt.Errorf("o save base mudou durante a cópia; feche o jogo e tente novamente")
	}
	digest := sha256.Sum256(b)
	m = SandboxManifest{Schema: 1, Launcher: Version, Created: time.Now().UTC(), Source: source, SourceSHA256: hex.EncodeToString(digest[:]), Revision: save.revision, Place: p.ID, Patched: patched, MissingOptional: missing, CampaignRemoved: false}
	if _, e := os.Lstat(target); e == nil {
		m.Backup = target + "-backup-" + time.Now().UTC().Format("20060102T150405.000000000")
	}
	report, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return m, err
	}
	if err = os.WriteFile(filepath.Join(stage, "sandbox.json"), append(report, '\n'), 0600); err != nil {
		return m, err
	}
	if m.Backup != "" {
		if err = os.Rename(target, m.Backup); err != nil {
			return m, err
		}
	}
	if err = os.Rename(stage, target); err != nil {
		if m.Backup != "" {
			if restore := os.Rename(m.Backup, target); restore != nil {
				return m, fmt.Errorf("falha ao publicar; seu sandbox anterior está em %s: %w", m.Backup, restore)
			}
		}
		return m, err
	}
	return m, nil
}
func BuildSandboxLaunch(c Config, port int, baseEnv []string) (LaunchSpec, error) {
	c.Normalize()
	if err := ValidateSandboxFlags(c); err != nil {
		return LaunchSpec{}, err
	}
	if c.Backend != Savage {
		return LaunchSpec{}, fmt.Errorf("sandbox disponível apenas no Savage Wind")
	}
	if port < 1024 || port > 65535 {
		return LaunchSpec{}, fmt.Errorf("porta local inválida")
	}
	if _, err := LoadSandbox(c); err != nil {
		return LaunchSpec{}, fmt.Errorf("prepare o sandbox antes de iniciar: %w", err)
	}
	spec, err := BuildLaunch(c, false, baseEnv)
	if err != nil {
		return spec, err
	}
	spec.Env = MergeEnv(spec.Env, map[string]string{"SAVAGEWIND_SAVE_DIR": SandboxSaveDir(c), "SAVAGEWIND_CONTROL": strconv.Itoa(port)}, nil)
	if !c.KeepCampaign {
		if _, err := loadFilter(c, ""); err != nil {
			return LaunchSpec{}, fmt.Errorf("prepare o filtro de campanha: %w", err)
		}
		spec.Env = MergeEnv(spec.Env, map[string]string{"SAVAGEWIND_ROMFS_OVERLAY": FilterOverlay(c)}, nil)
	}
	return spec, nil
}
