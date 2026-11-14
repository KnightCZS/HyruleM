package launcher

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const NativePreviewABI = 3
const NativePackageManifest = "hyrulem-preview.json"

// A native package selects its own mode even when the executable is opened
// directly. The regular launcher, without this manifest, keeps its usual mode.
type NativePackage struct {
	Schema        int    `json:"schema"`
	Version       string `json:"version"`
	PreviewABI    int    `json:"preview_abi"`
	RuntimeSHA256 string `json:"runtime_sha256"`
}

func LoadNativePackage(root string) (*NativePackage, error) {
	data, err := os.ReadFile(filepath.Join(root, NativePackageManifest))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p NativePackage
	if err = json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("pacote nativo inválido: extraia o ZIP completo novamente")
	}
	hash, err := hex.DecodeString(p.RuntimeSHA256)
	if err != nil || len(hash) != sha256.Size || p.Schema != 1 || p.Version != Version || p.PreviewABI != NativePreviewABI {
		return nil, fmt.Errorf("launcher e pacote nativo incompatíveis; extraia todos os arquivos da mesma versão")
	}
	return &p, nil
}

func EnablePackagedNativePreview(executable string) error {
	p, err := LoadNativePackage(filepath.Dir(executable))
	if err != nil {
		return err
	}
	if p != nil {
		return os.Setenv(NativePreviewEnv, "1")
	}
	return nil
}

func VerifyNativeRuntime(c Config, packageRoot string) error {
	p, err := LoadNativePackage(packageRoot)
	if err != nil {
		return err
	}
	if p != nil {
		f, err := os.Open(c.Executable)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return err
		}
		if hex.EncodeToString(h.Sum(nil)) != p.RuntimeSHA256 {
			return fmt.Errorf("o runtime selecionado não é o deste pacote: %s. Feche os jogos e abra Iniciar-Nativo.cmd para atualizar essa instalação", c.Executable)
		}
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(c.Executable), "hyrulem-build.json"))
	var build struct {
		PreviewABI int `json:"preview_abi"`
	}
	if err != nil || json.Unmarshal(data, &build) != nil || build.PreviewABI != NativePreviewABI {
		return fmt.Errorf("runtime nativo antigo ou não instalado (exige ABI %d). Feche os jogos e execute Iniciar-Nativo.cmd do pacote novo", NativePreviewABI)
	}
	return nil
}
