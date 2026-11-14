package launcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const Version = "0.5.0-preview.8"

type Backend string

const (
	Savage Backend = "savagewind"
	Native Backend = "botw-native"
)

type Config struct {
	Schema       int
	Backend      Backend
	Executable   string
	GameDir      string
	DataDir      string
	SandboxSeed  string `json:",omitempty"`
	SandboxPlace string `json:",omitempty"`
	KeepCampaign bool   `json:",omitempty"`
}

func DefaultConfig() Config { return Config{Schema: 1, Backend: Savage} }
func AppDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(base, "HyruleLauncher"), nil
}
func LoadConfig(path string) (Config, error) {
	c := DefaultConfig()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("configuração inválida; o arquivo foi preservado: %w", err)
	}
	if c.Schema > 1 {
		return c, fmt.Errorf("a configuração pertence a uma versão mais nova")
	}
	c.Normalize()
	if c.Backend != Savage && c.Backend != Native {
		return c, fmt.Errorf("recomp desconhecida: %s", c.Backend)
	}
	return c, nil
}
func (c *Config) Normalize() {
	c.Schema = 1
	if c.Backend == "" {
		c.Backend = Savage
	}
	c.Executable = cleanInput(c.Executable)
	c.GameDir = cleanInput(c.GameDir)
	c.DataDir = cleanInput(c.DataDir)
	c.SandboxSeed = cleanInput(c.SandboxSeed)
	if c.SandboxPlace == "" {
		c.SandboxPlace = "hateno"
	}
}
func cleanInput(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "" {
		return ""
	}
	return filepath.Clean(s)
}
func SaveConfig(path string, c Config) error {
	c.Normalize()
	if c.Backend != Savage && c.Backend != Native {
		return fmt.Errorf("recomp inválida")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if old, readErr := os.ReadFile(path); readErr == nil {
		if err = os.WriteFile(path+".bak", old, 0600); err != nil {
			return fmt.Errorf("não foi possível preservar a configuração anterior: %w", err)
		}
	}
	return os.Rename(name, path)
}
func LogPath(root string) string { return filepath.Join(root, "logs", "launcher.log") }
func RuntimeLog(c Config) string {
	if c.Backend == Native {
		return filepath.Join(c.DataDir, "runtime.log")
	}
	return filepath.Join(c.DataDir, "savagewind.log")
}
func SaveDir(c Config) string { return filepath.Join(c.DataDir, "save") }
func AppendLog(path, message string) {
	_ = os.MkdirAll(filepath.Dir(path), 0700)
	if st, err := os.Stat(path); err == nil && st.Size() > 2*1024*1024 {
		_ = os.Remove(path + ".1")
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s  %s\n", time.Now().Format(time.RFC3339), message)
}
func ReadLogTail(path string, limit int64) string {
	f, err := os.Open(path)
	if err != nil {
		return "Ainda não há registro disponível nesta pasta."
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err.Error()
	}
	start := st.Size() - limit
	if start < 0 {
		start = 0
	}
	_, _ = f.Seek(start, 0)
	b := make([]byte, st.Size()-start)
	n, _ := f.Read(b)
	return string(b[:n])
}
