package nativebridge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hyrulelauncher/internal/gen1"
	core "hyrulelauncher/internal/launcher"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

type FileMetadata struct {
	Name    string `json:"name"`
	Size    int64  `json:"size,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
	BuildID string `json:"nso_build_id,omitempty"`
	Error   string `json:"error,omitempty"`
}
type ProbeCommand struct {
	Command string          `json:"command"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
}
type Report struct {
	Schema       int                `json:"schema"`
	Created      string             `json:"created_utc"`
	Slot         int                `json:"slot"`
	Port         int                `json:"control_port"`
	NativeReady  bool               `json:"native_ready"`
	NativeError  string             `json:"native_error,omitempty"`
	Capabilities *gen1.Capabilities `json:"capabilities,omitempty"`
	Files        []FileMetadata     `json:"files"`
	Queries      []ProbeCommand     `json:"queries"`
}

// FileInfo exports metadata only, never the game module or save bytes.
func FileInfo(name, path string) FileMetadata {
	m := FileMetadata{Name: name}
	f, err := os.Open(path)
	if err != nil {
		m.Error = "arquivo não encontrado ou inacessível"
		return m
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() > 2*1024*1024*1024 {
		m.Error = "arquivo inválido ou maior que 2 GiB"
		return m
	}
	m.Size = st.Size()
	header := make([]byte, 256)
	n, _ := f.ReadAt(header, 0)
	if n == 256 && string(header[:4]) == "NSO0" {
		m.BuildID = hex.EncodeToString(header[0x40:0x60])
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		m.Error = "falha ao calcular hash"
		return m
	}
	m.SHA256 = hex.EncodeToString(h.Sum(nil))
	return m
}

var endpoint = regexp.MustCompile(`\[control\] listening on 127\.0\.0\.1:([0-9]+)`)

func ControlPort(log string) (int, error) {
	matches := endpoint.FindAllStringSubmatch(log, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("porta não encontrada no log; abra o sandbox ou informe --port")
	}
	p, err := strconv.Atoi(matches[len(matches)-1][1])
	if err != nil || p < 1024 || p > 65535 {
		return 0, fmt.Errorf("porta de controle inválida no log")
	}
	return p, nil
}

// Probe only calls this fixed read-only command list. It never starts the game,
// prepares a sandbox, writes memory, grants items or edits a save.
func Probe(ctx context.Context, cfg core.Config, slot, port int) (Report, error) {
	r := Report{Schema: 1, Created: time.Now().UTC().Format(time.RFC3339), Slot: slot}
	if cfg.DataDir == "" || slot < 0 || slot > 2 {
		return r, fmt.Errorf("configuração/pasta de dados ou instância inválida")
	}
	data := cfg.DataDir
	if slot > 0 {
		data = core.InstanceDataDir(cfg, slot-1)
	}
	r.Files = append(r.Files, FileInfo("runtime", cfg.Executable), FileInfo("extracted/exefs/main", filepath.Join(cfg.DataDir, "extracted", "exefs", "main")))
	for _, name := range []string{"swgame.dll", "swgame.so"} {
		path := filepath.Join(data, name)
		if _, err := os.Stat(path); err == nil {
			r.Files = append(r.Files, FileInfo(name, path))
		}
	}
	if port == 0 {
		var err error
		port, err = ControlPort(core.ReadLogTail(filepath.Join(data, "savagewind.log"), 256*1024))
		if err != nil {
			r.NativeError = err.Error()
			return r, nil
		}
	}
	if port < 1024 || port > 65535 {
		return r, fmt.Errorf("porta deve ficar entre 1024 e 65535")
	}
	r.Port = port
	c := core.ControlClient{Port: port, Timeout: 2 * time.Second}
	for _, command := range []string{"version", "help", "sites", "mem regions", "pos", "camera", "hm_caps"} {
		if ctx.Err() != nil {
			r.NativeError = ctx.Err().Error()
			break
		}
		reply, err := c.Command(ctx, command)
		q := ProbeCommand{Command: command, Data: reply.Data}
		if err != nil {
			q.Error = err.Error()
		}
		r.Queries = append(r.Queries, q)
		if command == "version" && err != nil {
			r.NativeError = "Controle indisponível; mantenha a janela do jogo renderizando. " + err.Error()
			break
		}
		if command == "hm_caps" {
			var caps gen1.Capabilities
			if err == nil {
				err = json.Unmarshal(reply.Data, &caps)
			}
			if err == nil {
				r.Capabilities = &caps
				err = caps.Require(false)
			}
			if err != nil {
				r.NativeError = "Backend nativo indisponível: " + err.Error()
			} else {
				r.NativeReady = true
			}
		}
	}
	return r, nil
}
