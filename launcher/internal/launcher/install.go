package launcher

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const SavageTag = "v0.5.3-alpha.5"
const SavageURL = "https://github.com/Revivedx/SavageWindRecomp/releases/download/v0.5.3-alpha.5/SavageWind-0.5.3-alpha.5-windows.zip"
const SavageSHA256 = "667c4909a8ed72c002e5aa40a00d38720627eebf48b860beb5cd8805421f1285"
const SavageDownloadBytes int64 = 93924792
const maxArchiveBytes int64 = 256 * 1024 * 1024
const maxExtractBytes uint64 = 2 * 1024 * 1024 * 1024

type Progress struct {
	Stage string
	Done  int64
	Total int64
}
type InstallResult struct {
	Executable string
	Directory  string
	Reused     bool
}
type downloadProgress struct {
	writer      io.Writer
	fn          func(Progress)
	done, total int64
	last        time.Time
}

func (p *downloadProgress) Write(b []byte) (int, error) {
	n, err := p.writer.Write(b)
	p.done += int64(n)
	if time.Since(p.last) > 250*time.Millisecond {
		p.fn(Progress{Stage: "Baixando a recomp", Done: p.done, Total: p.total})
		p.last = time.Now()
	}
	return n, err
}
func InstallSavage(ctx context.Context, root string, notify func(Progress)) (InstallResult, error) {
	return InstallSavageFrom(ctx, root, "", notify)
}
func InstallSavageFrom(ctx context.Context, root, localArchive string, notify func(Progress)) (InstallResult, error) {
	if notify == nil {
		notify = func(Progress) {}
	}
	destination := filepath.Join(root, "runtimes", "savagewind-"+SavageTag)
	manifest := filepath.Join(destination, "hyrule-install.json")
	if b, err := os.ReadFile(manifest); err == nil {
		var m struct {
			SHA256     string
			Executable string
		}
		if json.Unmarshal(b, &m) == nil && m.SHA256 == SavageSHA256 {
			exe := filepath.Join(destination, m.Executable)
			if rel, err := filepath.Rel(destination, exe); err == nil && !strings.HasPrefix(rel, "..") && IsFile(exe) {
				return InstallResult{exe, destination, true}, nil
			}
		}
	}
	if _, err := os.Stat(destination); err == nil {
		return InstallResult{}, fmt.Errorf("a pasta de destino já existe; escolha a instalação existente pelo botão Procurar")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return InstallResult{}, err
	}
	staging, err := os.MkdirTemp(filepath.Dir(destination), ".savage-download-")
	if err != nil {
		return InstallResult{}, err
	}
	defer os.RemoveAll(staging)
	archive := filepath.Join(staging, "runtime.zip")
	var source io.ReadCloser
	stage := "Baixando a recomp"
	if localArchive != "" {
		source, err = os.Open(localArchive)
		if err != nil {
			return InstallResult{}, fmt.Errorf("pacote local não encontrado: %w", err)
		}
		stage = "Verificando pacote incluído"
	} else {
		client := &http.Client{Timeout: 20 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 10 {
				return fmt.Errorf("redirecionamentos demais")
			}
			return officialURL(req.URL)
		}}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, SavageURL, nil)
		if err != nil {
			return InstallResult{}, err
		}
		req.Header.Set("User-Agent", "HyruleLauncher/"+Version)
		if err = officialURL(req.URL); err != nil {
			return InstallResult{}, err
		}
		response, err := client.Do(req)
		if err != nil {
			return InstallResult{}, fmt.Errorf("falha no download; sua instalação atual não foi alterada: %w", err)
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return InstallResult{}, fmt.Errorf("o servidor respondeu HTTP %d", response.StatusCode)
		}
		if response.ContentLength > maxArchiveBytes {
			response.Body.Close()
			return InstallResult{}, fmt.Errorf("arquivo maior que o limite permitido")
		}
		source = response.Body
	}
	defer source.Close()
	f, err := os.OpenFile(archive, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return InstallResult{}, err
	}
	h := sha256.New()
	writer := &downloadProgress{writer: io.MultiWriter(f, h), fn: func(p Progress) { p.Stage = stage; notify(p) }, total: SavageDownloadBytes}
	n, copyErr := io.Copy(writer, io.LimitReader(source, maxArchiveBytes+1))
	closeErr := f.Close()
	if copyErr != nil {
		return InstallResult{}, copyErr
	}
	if closeErr != nil {
		return InstallResult{}, closeErr
	}
	if n != SavageDownloadBytes {
		return InstallResult{}, fmt.Errorf("download incompleto ou diferente da versão esperada (%d bytes)", n)
	}
	notify(Progress{Stage: "Verificando integridade", Done: n, Total: n})
	if hex.EncodeToString(h.Sum(nil)) != SavageSHA256 {
		return InstallResult{}, fmt.Errorf("a verificação SHA-256 falhou; o pacote não foi instalado")
	}
	extracted := filepath.Join(staging, "runtime")
	notify(Progress{Stage: "Extraindo a recomp"})
	if err = ExtractZIP(ctx, archive, extracted); err != nil {
		return InstallResult{}, err
	}
	exe, err := FindSavageExecutable(extracted)
	if err != nil {
		return InstallResult{}, err
	}
	relative, err := filepath.Rel(extracted, exe)
	if err != nil {
		return InstallResult{}, err
	}
	m, _ := json.MarshalIndent(struct{ Version, SHA256, Executable string }{SavageTag, SavageSHA256, relative}, "", "  ")
	if err = os.WriteFile(filepath.Join(extracted, "hyrule-install.json"), m, 0600); err != nil {
		return InstallResult{}, err
	}
	if err = ctx.Err(); err != nil {
		return InstallResult{}, err
	}
	if _, err = os.Stat(destination); err == nil {
		return InstallResult{}, fmt.Errorf("outra instalação ocupou a pasta de destino")
	}
	if err = os.Rename(extracted, destination); err != nil {
		return InstallResult{}, err
	}
	return InstallResult{filepath.Join(destination, relative), destination, false}, nil
}
func officialURL(u *url.URL) error {
	if u.Scheme != "https" || u.User != nil {
		return fmt.Errorf("endereço de download inválido")
	}
	switch strings.ToLower(u.Hostname()) {
	case "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com", "github-releases.githubusercontent.com":
		return nil
	}
	return fmt.Errorf("origem de download inesperada: %s", u.Hostname())
}
func safeZIPPath(name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	clean := path.Clean(name)
	if clean == "." || strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, ":") {
		return "", fmt.Errorf("caminho inválido no pacote")
	}
	for _, component := range strings.Split(clean, "/") {
		if strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") || strings.ContainsAny(component, "\x00<>\"|?*") {
			return "", fmt.Errorf("nome inválido no pacote")
		}
		base := strings.ToUpper(strings.SplitN(component, ".", 2)[0])
		switch base {
		case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
			return "", fmt.Errorf("nome reservado no pacote")
		}
	}
	return filepath.FromSlash(clean), nil
}
func ExtractZIP(ctx context.Context, archive, destination string) error {
	z, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer z.Close()
	if len(z.File) > 50000 {
		return fmt.Errorf("o pacote tem arquivos demais")
	}
	var total uint64
	seen := map[string]bool{}
	// Validate the complete index before writing any entry.
	for _, f := range z.File {
		name, err := safeZIPPath(f.Name)
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		if f.Mode()&os.ModeSymlink != 0 || (!f.FileInfo().IsDir() && !f.Mode().IsRegular()) {
			return fmt.Errorf("o pacote contém um tipo de arquivo não permitido")
		}
		key := strings.ToLower(name)
		if seen[key] {
			return fmt.Errorf("entrada duplicada no pacote: %s", f.Name)
		}
		seen[key] = true
		if f.UncompressedSize64 > 512*1024*1024 || total > maxExtractBytes-f.UncompressedSize64 {
			return fmt.Errorf("pacote expandido maior que o limite")
		}
		total += f.UncompressedSize64
	}
	if err = os.MkdirAll(destination, 0700); err != nil {
		return err
	}
	for _, f := range z.File {
		if err = ctx.Err(); err != nil {
			return err
		}
		relative, _ := safeZIPPath(f.Name)
		target := filepath.Join(destination, relative)
		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(target, 0700); err != nil {
				return err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		input, err := f.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			input.Close()
			return err
		}
		n, copyErr := io.Copy(output, io.LimitReader(input, int64(f.UncompressedSize64)+1))
		inErr := input.Close()
		outErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inErr != nil {
			return inErr
		}
		if outErr != nil {
			return outErr
		}
		if uint64(n) != f.UncompressedSize64 {
			return fmt.Errorf("tamanho extraído incorreto")
		}
	}
	return nil
}
func FindSavageExecutable(root string) (string, error) {
	var found []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(d.Name(), "savagewind.exe") {
			found = append(found, p)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(found) != 1 {
		return "", fmt.Errorf("o pacote precisa conter exatamente um savagewind.exe; encontrados: %d", len(found))
	}
	return found[0], nil
}
