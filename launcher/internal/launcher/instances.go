package launcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// InstanceFinder only adopts this launch's process tree, never another client.
type InstanceFinder struct {
	mu     sync.Mutex
	pids   map[int]bool
	source Finder
}

func NewInstanceFinder() *InstanceFinder { return &InstanceFinder{source: SystemFinder{}} }
func (f *InstanceFinder) TrackPID(pid int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pids = map[int]bool{pid: true}
}
func (f *InstanceFinder) Find(exe string, since time.Time) []ProcessInfo {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.pids) == 0 {
		return nil
	}
	all := f.source.Find(exe, since)
	for n := 0; n < len(all); n++ {
		for _, p := range all {
			if f.pids[p.ParentPID] {
				f.pids[p.PID] = true
			}
		}
	}
	var out []ProcessInfo
	for _, p := range all {
		if f.pids[p.PID] {
			out = append(out, p)
		}
	}
	return out
}

type MultiplayerInstance struct {
	Config Config
	Spec   LaunchSpec
	Port   int
	Slot   int
}

func InstanceDataDir(c Config, slot int) string {
	return filepath.Join(c.DataDir, "hyrulem", "client-"+strconv.Itoa(slot+1))
}

// PrepareMultiplayerInstance creates private writable state and links only the
// extracted assets. It never downloads, includes, or modifies game resources.
func PrepareMultiplayerInstance(c Config, slot, port int, baseEnv []string) (MultiplayerInstance, error) {
	var result MultiplayerInstance
	if slot < 0 || slot > 1 {
		return result, fmt.Errorf("instância inválida")
	}
	if c.Backend != Savage || PreparationNeeded(c) {
		return result, fmt.Errorf("prepare o Savage Wind e o jogo antes de entrar")
	}
	if _, err := LoadSandbox(c); err != nil {
		return result, fmt.Errorf("crie e teste o sandbox antes do multiplayer: %w", err)
	}
	private := c
	private.DataDir = InstanceDataDir(c, slot)
	private.SandboxPlace = "hateno"
	if err := os.MkdirAll(private.DataDir, 0700); err != nil {
		return result, err
	}
	target, err := filepath.EvalSymlinks(filepath.Join(c.DataDir, "extracted"))
	if err != nil {
		return result, err
	}
	link := filepath.Join(private.DataDir, "extracted")
	if _, err = os.Lstat(link); os.IsNotExist(err) {
		err = directoryLink(link, target)
	}
	if err != nil {
		return result, fmt.Errorf("não foi possível vincular extracted: %w", err)
	}
	linkedInfo, linkErr := os.Stat(link)
	targetInfo, targetErr := os.Stat(target)
	if linkErr != nil || targetErr != nil || !os.SameFile(linkedInfo, targetInfo) {
		return result, fmt.Errorf("o vínculo extracted desta instância aponta para outra instalação; pasta preservada: %s", link)
	}
	// Only ordinary bounded settings files are copied once. Launch flags and the
	// runtime's internal re-exec marker must never enter a private data root.
	for _, name := range []string{"savagewind.ini"} {
		dst := filepath.Join(private.DataDir, name)
		if _, err := os.Lstat(dst); os.IsNotExist(err) {
			b, e := readRegular(filepath.Join(c.DataDir, name), 1024*1024)
			if e == nil {
				e = os.WriteFile(dst, b, 0600)
			}
			if e != nil && !os.IsNotExist(e) {
				return result, e
			}
		}
	}
	seedConfig := c
	seedConfig.DataDir = c.DataDir
	root := SandboxSaveDir(c)
	seedConfig.DataDir = filepath.Dir(root) // FindSandboxSeed's automatic search is root/save.
	seedConfig.SandboxSeed = ""
	source, err := FindSandboxSeed(seedConfig)
	if err != nil {
		return result, err
	}
	private.SandboxSeed = source
	if _, err = PrepareSandbox(private, false); err != nil {
		return result, err
	}
	// Build against the original filter; it is read-only and potentially expensive.
	spec, err := BuildLaunch(private, false, baseEnv)
	if err != nil {
		return result, err
	}
	if port < 1024 || port > 65535 {
		return result, fmt.Errorf("porta de controle inválida")
	}
	vars := map[string]string{"SAVAGEWIND_SETTINGS": filepath.Join(private.DataDir, "savagewind.ini"), "SAVAGEWIND_SAVE_DIR": SandboxSaveDir(private), "SAVAGEWIND_CONTROL": strconv.Itoa(port), "SAVAGEWIND_NO_UPDATE_CHECK": "1"}
	for _, p := range []string{filepath.Join(c.DataDir, "extracted", "swgame.dll"), filepath.Join(filepath.Dir(c.Executable), "swgame.dll")} {
		if IsFile(p) {
			vars["SAVAGEWIND_GAME_LIB"] = p
			break
		}
	}
	if !c.KeepCampaign {
		if _, err := loadFilter(c, ""); err != nil {
			return result, fmt.Errorf("atualize o filtro de campanha antes de entrar: %w", err)
		}
		vars["SAVAGEWIND_ROMFS_OVERLAY"] = FilterOverlay(c)
	}
	if nativePreviewRequested(baseEnv) {
		if !IsFile(filepath.Join(NativePreviewOverlay(c), "hyrulem-native-overlay.json")) {
			return result, fmt.Errorf("overlay nativo ainda não preparado")
		}
		vars["SAVAGEWIND_ROMFS_OVERLAY"] = NativePreviewOverlay(c)
		vars[NativePreviewEnv] = "1"
	}
	// Inherited test/debug switches could change spawn or auto-close the game.
	var env []string
	for _, entry := range spec.Env {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		if strings.HasPrefix(key, "SAVAGEWIND_TEST_") || key == "SAVAGEWIND_AUTO_EXIT" {
			continue
		}
		env = append(env, entry)
	}
	spec.Env = MergeEnv(env, vars, nil)
	result = MultiplayerInstance{private, spec, port, slot}
	return result, nil
}
