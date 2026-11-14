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
	"strings"
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
	Schema          int                `json:"schema"`
	ProbeVersion    string             `json:"probe_version"`
	Created         string             `json:"created_utc"`
	Slot            int                `json:"slot"`
	Port            int                `json:"control_port"`
	ControlSource   string             `json:"control_source,omitempty"`
	NativeReady     bool               `json:"native_ready"`
	NativeError     string             `json:"native_error,omitempty"`
	Preview         PreviewDiagnostic  `json:"preview"`
	Capabilities    *gen1.Capabilities `json:"capabilities,omitempty"`
	Files           []FileMetadata     `json:"files"`
	Queries         []ProbeCommand     `json:"queries"`
	Inspection      *NativeInspection  `json:"native_inspection,omitempty"`
	OverlayPolicy   string             `json:"overlay_policy,omitempty"`
	RuntimeMessages []string           `json:"runtime_messages,omitempty"`
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

func discoverPort(cfg core.Config, appRoot string, slot int) (int, string, error) {
	private := cfg
	name := "sandbox-processo.log"
	if slot > 0 {
		private.DataDir = core.InstanceDataDir(cfg, slot-1)
		name = fmt.Sprintf("multiplayer-processo-%d.log", slot)
	}
	// Session.Start captures stdout/stderr in the launcher's logs directory.
	// The runtime's optional own log is only a fallback, not the primary source.
	paths := []string{}
	if appRoot != "" {
		paths = append(paths, filepath.Join(appRoot, "logs", name))
	}
	paths = append(paths, core.RuntimeLog(private))
	for _, path := range paths {
		if port, err := ControlPort(core.ReadLogTail(path, 256*1024)); err == nil {
			return port, filepath.Base(path), nil
		}
	}
	return 0, "", fmt.Errorf("porta do cliente %d não encontrada; abra essa instância pelo launcher e aguarde carregar, ou informe --port", slot)
}

// Probe only calls this fixed read-only command list. It never starts the game,
// prepares a sandbox, writes memory, grants items or edits a save.
func Probe(ctx context.Context, cfg core.Config, slot, port int) (Report, error) {
	root, _ := core.AppDir()
	return ProbeAt(ctx, cfg, root, slot, port)
}

// ProbeAt uses the folder of the selected config.json for launcher capture logs,
// including when --config points outside the default AppDir.
func ProbeAt(ctx context.Context, cfg core.Config, appRoot string, slot, port int) (Report, error) {
	r := Report{Schema: 2, ProbeVersion: "native-preview-diag.9", Created: time.Now().UTC().Format(time.RFC3339), Slot: slot}
	if cfg.DataDir == "" || slot < 0 || slot > 2 {
		return r, fmt.Errorf("configuração/pasta de dados ou instância inválida")
	}
	data := cfg.DataDir
	if slot > 0 {
		data = core.InstanceDataDir(cfg, slot-1)
	}
	r.Files = append(r.Files, FileInfo("runtime", cfg.Executable), FileInfo("extracted/exefs/main", filepath.Join(cfg.DataDir, "extracted", "exefs", "main")))
	var policy map[string]string
	if b, e := os.ReadFile(filepath.Join(core.NativePreviewOverlay(cfg), "hyrulem-native-overlay.json")); e == nil && json.Unmarshal(b, &policy) == nil {
		r.OverlayPolicy = policy["policy"]
	}
	for _, actor := range []string{"HyruleM_RemoteLink", "HyruleM_RemoteLink_Fallback"} {
		path := filepath.Join(core.NativePreviewOverlay(cfg), "Actor", "Pack", actor+".sbactorpack")
		if core.IsFile(path) {
			r.Files = append(r.Files, FileInfo("overlay/"+actor, path))
		}
	}
	logName := "sandbox-processo.log"
	if slot > 0 {
		logName = fmt.Sprintf("multiplayer-processo-%d.log", slot)
	}
	for _, line := range strings.Split(core.ReadLogTail(filepath.Join(appRoot, "logs", logName), 128*1024), "\n") {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "[hyrulem]") && !(strings.Contains(lower, "error") && (strings.Contains(lower, "resource") || strings.Contains(lower, "actor"))) {
			continue
		}
		for _, private := range []string{cfg.DataDir, filepath.Dir(cfg.Executable), appRoot} {
			if private != "" && private != "." {
				line = strings.ReplaceAll(line, private, "<local>")
			}
		}
		r.RuntimeMessages = append(r.RuntimeMessages, line)
		if len(r.RuntimeMessages) > 32 {
			r.RuntimeMessages = r.RuntimeMessages[1:]
		}
	}
	for _, name := range []string{"swgame.dll", "swgame.so"} {
		path := filepath.Join(data, name)
		if _, err := os.Stat(path); err == nil {
			r.Files = append(r.Files, FileInfo(name, path))
		}
	}
	if port == 0 {
		var err error
		port, r.ControlSource, err = discoverPort(cfg, appRoot, slot)
		if err != nil {
			r.NativeError = err.Error()
			r.Preview = DiagnosePreview(r.Queries)
			return r, nil
		}
	} else {
		r.ControlSource = "--port"
	}
	if port < 1024 || port > 65535 {
		return r, fmt.Errorf("porta deve ficar entre 1024 e 65535")
	}
	r.Port = port
	c := core.ControlClient{Port: port, Timeout: 2 * time.Second}
	playerLoaded := false
	for _, command := range []string{"version", "help", "sites", "mem regions", "pos", "camera", "hm_caps", "hm_native_status", "hm_world_status", "hm_local_state", "time", "weather", "hm_items_status"} {
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
		if command == "pos" && err == nil {
			playerLoaded = true
		}
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
	r.Preview = DiagnosePreview(r.Queries)
	if playerLoaded && ctx.Err() == nil {
		r.Inspection = inspectNative(ctx, r.Files[1].SHA256, controlMemory{client: c})
	}
	return r, nil
}
