package launcher

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hyrulelauncher/internal/nativeassets"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const NativePreviewEnv = "HYRULEM_NATIVE_PREVIEW"

func nativePreviewRequested(env []string) bool {
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, NativePreviewEnv) {
			return value == "1"
		}
	}
	return false
}
func NativePreviewOverlay(c Config) string {
	return filepath.Join(c.DataDir, "hyrulem", "native-preview-overlay")
}

// The native layer includes the campaign layer so one RomFS overlay cannot
// silently disable the other. Never replace a layer while either game uses it.
func prepareNativePreview(c Config, running bool) error {
	out := NativePreviewOverlay(c)
	manifestPath := filepath.Join(out, "hyrulem-native-overlay.json")
	if running {
		if !IsFile(manifestPath) || !IsFile(filepath.Join(out, filepath.FromSlash(nativeassets.PackPath))) {
			return fmt.Errorf("feche os jogos para preparar o overlay nativo inicial")
		}
		return nil
	}
	romfs := filepath.Join(c.DataDir, "extracted", "romfs")
	inputs := map[string]string{}
	read := func(path string) ([]byte, error) {
		full := filepath.Join(romfs, filepath.FromSlash(path))
		if !c.KeepCampaign {
			filtered := filepath.Join(FilterOverlay(c), filepath.FromSlash(path))
			if IsFile(filtered) {
				full = filtered
			}
		}
		data, err := readRegular(full, 512<<20)
		if err == nil {
			sum := sha256.Sum256(data)
			inputs[path] = hex.EncodeToString(sum[:])
		}
		return data, err
	}
	title, err := read("Pack/TitleBG.pack")
	if err != nil {
		return err
	}
	info, err := read(nativeassets.InfoPath)
	if err != nil {
		return err
	}
	table, err := read(nativeassets.RSTBPath)
	if err != nil {
		return err
	}
	inputs["policy"] = "remote-demo-npc-v3-generic-as-two-tracks"
	if !c.KeepCampaign {
		err = filepath.WalkDir(FilterOverlay(c), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, e := filepath.Rel(FilterOverlay(c), path)
			if e != nil {
				return e
			}
			data, e := readRegular(path, 512<<20)
			if e != nil {
				return e
			}
			sum := sha256.Sum256(data)
			inputs["filter/"+filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
			return nil
		})
		if err != nil {
			return err
		}
	}
	manifest, _ := json.Marshal(inputs)
	if existing, e := os.ReadFile(manifestPath); e == nil && string(existing) == string(manifest) && IsFile(filepath.Join(out, filepath.FromSlash(nativeassets.PackPath))) {
		return nil
	}
	generated, err := nativeassets.BuildRemote(title, info, table)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(out), 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(out), ".native-overlay-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	write := func(name string, data []byte) error {
		path := filepath.Join(stage, filepath.FromSlash(name))
		if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			return e
		}
		return os.WriteFile(path, data, 0600)
	}
	if !c.KeepCampaign {
		err = filepath.WalkDir(FilterOverlay(c), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, e := filepath.Rel(FilterOverlay(c), path)
			if e != nil {
				return e
			}
			data, e := readRegular(path, 512<<20)
			if e != nil {
				return e
			}
			return write(rel, data)
		})
		if err != nil {
			return err
		}
	}
	for path, data := range generated {
		if err = write(path, data); err != nil {
			return err
		}
	}
	if err = write("hyrulem-native-overlay.json", manifest); err != nil {
		return err
	}
	backup := out + ".previous"
	if _, e := os.Lstat(backup); e == nil {
		return fmt.Errorf("overlay anterior preservado em %s; restaure ou mova essa pasta antes de atualizar", backup)
	}
	old := false
	if _, e := os.Lstat(out); e == nil {
		if err = os.Rename(out, backup); err != nil {
			return err
		}
		old = true
	}
	if err = os.Rename(stage, out); err != nil {
		if old {
			_ = os.Rename(backup, out)
		}
		return err
	}
	if old {
		_ = os.RemoveAll(backup)
	}
	return nil
}
