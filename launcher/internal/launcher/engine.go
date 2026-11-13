package launcher

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type LaunchSpec struct {
	Executable string
	Args       []string
	Dir        string
	Env        []string
}

func IsFile(path string) bool { st, err := os.Stat(path); return err == nil && st.Mode().IsRegular() }
func IsDir(path string) bool  { st, err := os.Stat(path); return err == nil && st.IsDir() }
func SuggestDataDir(exe string, backend Backend) string {
	if backend == Savage {
		return filepath.Dir(exe)
	}
	if root := os.Getenv("APPDATA"); root != "" {
		return filepath.Join(root, "BotWRecomp")
	}
	root, _ := os.UserConfigDir()
	return filepath.Join(root, "BotWRecomp")
}
func Identify(exe string) Backend {
	name := strings.ToLower(filepath.Base(exe))
	if strings.Contains(name, "botwrecomp") || strings.Contains(name, "botw-native") || strings.Contains(name, "botwlauncher") {
		return Native
	}
	return Savage
}
func Validate(c Config) error {
	if c.Backend != Savage && c.Backend != Native {
		return fmt.Errorf("selecione uma recomp válida")
	}
	if c.Executable == "" {
		return fmt.Errorf("selecione o programa da recomp ou baixe o Savage Wind")
	}
	if !filepath.IsAbs(c.Executable) {
		return fmt.Errorf("o caminho do programa precisa ser absoluto")
	}
	if !IsFile(c.Executable) {
		return fmt.Errorf("programa não encontrado: selecione novamente a recomp")
	}
	if c.DataDir == "" || !filepath.IsAbs(c.DataDir) {
		return fmt.Errorf("selecione uma pasta de dados com caminho absoluto")
	}
	if c.Backend == Native {
		if c.GameDir == "" || !filepath.IsAbs(c.GameDir) {
			return fmt.Errorf("selecione a pasta de BOTW Wii U")
		}
		if !IsFile(filepath.Join(c.GameDir, "code", "U-King.rpx")) {
			return fmt.Errorf("a pasta Wii U precisa conter code/U-King.rpx")
		}
		if !IsDir(filepath.Join(c.GameDir, "content")) {
			return fmt.Errorf("a pasta Wii U precisa conter content")
		}
	}
	return nil
}
func PreparationNeeded(c Config) bool {
	if !IsFile(c.Executable) {
		return true
	}
	if c.Backend == Native {
		return !IsFile(filepath.Join(c.GameDir, "code", "U-King.rpx"))
	}
	root := filepath.Join(c.DataDir, "extracted")
	if !IsFile(filepath.Join(root, "exefs", "main")) || !IsDir(filepath.Join(root, "romfs")) {
		return true
	}
	for _, p := range []string{filepath.Join(root, "swgame.dll"), filepath.Join(filepath.Dir(c.Executable), "swgame.dll")} {
		if IsFile(p) {
			return false
		}
	}
	return true
}
func BuildLaunch(c Config, setup bool, baseEnv []string) (LaunchSpec, error) {
	c.Normalize()
	if c.Executable == "" || (!IsFile(c.Executable) && !(setup && c.Backend == Native && SetupProgram(c) != "")) {
		return LaunchSpec{}, fmt.Errorf("selecione uma recomp instalada")
	}
	if !filepath.IsAbs(c.Executable) {
		return LaunchSpec{}, fmt.Errorf("o caminho da recomp precisa ser absoluto")
	}
	if c.DataDir == "" || !filepath.IsAbs(c.DataDir) {
		return LaunchSpec{}, fmt.Errorf("selecione uma pasta de dados válida")
	}
	spec := LaunchSpec{Executable: c.Executable, Dir: filepath.Dir(c.Executable)}
	vars := map[string]string{}
	remove := []string{"BOTW_GAME", "BOTW_USER_ROOT", "SAVAGEWIND_DATA_ROOT", "SAVAGEWIND_SAVE_DIR", "SAVAGEWIND_CONTROL", "SAVAGEWIND_DEV", "SAVAGEWIND_INPUT", "SAVAGEWIND_REEXEC", "SAVAGEWIND_NO_LAUNCHER", "SAVAGEWIND_GAME_LIB", "SAVAGEWIND_ROMFS_OVERLAY", "SAVAGEWIND_TEST_WARP", "SAVAGEWIND_TEST_FLAGS", "SAVAGEWIND_TEST_TIME", "SAVAGEWIND_TEST_WEATHER", "SAVAGEWIND_LAUNCH_FLAGS_APPLIED"}
	switch c.Backend {
	case Savage:
		vars["SAVAGEWIND_DATA_ROOT"] = c.DataDir
		vars["SAVAGEWIND_NO_UPDATE_CHECK"] = "1"
		if setup {
			spec.Args = []string{"--config"}
		} else {
			spec.Args = []string{"--game"}
		}
	case Native:
		if setup {
			found := SetupProgram(c)
			if found == "" {
				return spec, fmt.Errorf("setup do BotW-Native não encontrado ao lado do jogo; abra o launcher oficial da instalação")
			}
			spec.Executable = found
		} else {
			if err := Validate(c); err != nil {
				return spec, err
			}
			spec.Args = []string{"--game", c.GameDir, "--renderer", "vulkan"}
		}
		vars["BOTW_USER_ROOT"] = c.DataDir
	default:
		return spec, fmt.Errorf("recomp desconhecida")
	}
	spec.Env = MergeEnv(baseEnv, vars, remove)
	return spec, nil
}
func SetupProgram(c Config) string {
	if c.Backend == Savage {
		if IsFile(c.Executable) {
			return c.Executable
		}
		return ""
	}
	for _, name := range []string{"BotW-Native-Setup.exe", "BotWLauncher.exe", "BotWLauncherClassic.exe"} {
		p := filepath.Join(filepath.Dir(c.Executable), name)
		if IsFile(p) {
			return p
		}
	}
	return ""
}
func MergeEnv(base []string, values map[string]string, remove []string) []string {
	blocked := map[string]bool{}
	for _, k := range remove {
		blocked[strings.ToUpper(k)] = true
	}
	for k := range values {
		blocked[strings.ToUpper(k)] = true
	}
	out := make([]string, 0, len(base)+len(values))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok && !blocked[strings.ToUpper(key)] {
			out = append(out, entry)
		}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out = append(out, key+"="+values[key])
	}
	return out
}
func SamePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
