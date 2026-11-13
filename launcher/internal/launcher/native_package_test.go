package launcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackagedNativeModeRejectsOldSelectedRuntime(t *testing.T) {
	t.Setenv(NativePreviewEnv, "0")
	root := t.TempDir()
	launcher := filepath.Join(root, "HyruleLauncher.exe")
	if err := EnablePackagedNativePreview(launcher); err != nil || os.Getenv(NativePreviewEnv) != "0" {
		t.Fatal("regular launcher mode changed", err)
	}
	current := "synthetic current runtime"
	hash := sha256.Sum256([]byte(current))
	manifest, _ := json.Marshal(NativePackage{1, Version, NativePreviewABI, hex.EncodeToString(hash[:])})
	file(t, filepath.Join(root, NativePackageManifest), string(manifest))
	if err := EnablePackagedNativePreview(launcher); err != nil || os.Getenv(NativePreviewEnv) != "1" {
		t.Fatal("direct executable launch did not enable native mode", err)
	}
	c := Config{Executable: filepath.Join(t.TempDir(), "savagewind.exe")}
	file(t, c.Executable, "synthetic old ABI 1 runtime")
	file(t, filepath.Join(filepath.Dir(c.Executable), "hyrulem-build.json"), `{"upstream":"old"}`)
	if err := VerifyNativeRuntime(c, root); err == nil || !strings.Contains(err.Error(), "Iniciar-Nativo.cmd") {
		t.Fatal("old runtime accepted merely because a stamp exists", err)
	}
	file(t, c.Executable, current)
	if err := VerifyNativeRuntime(c, root); err == nil {
		t.Fatal("old adapter metadata accepted")
	}
	file(t, filepath.Join(filepath.Dir(c.Executable), "hyrulem-build.json"), `{"preview_abi":2}`)
	if err := VerifyNativeRuntime(c, root); err != nil {
		t.Fatal("matching package rejected", err)
	}
	file(t, filepath.Join(root, NativePackageManifest), `{"schema":1,"version":"old"}`)
	if err := EnablePackagedNativePreview(launcher); err == nil {
		t.Fatal("mixed package silently fell back to visual mode")
	}
}

func TestNativePreparationRejectsOldAdapterBeforeChangingSandbox(t *testing.T) {
	c := readyFilterFixture(t)
	file(t, filepath.Join(filepath.Dir(c.Executable), "hyrulem-build.json"), `{"upstream":"old"}`)
	p := NewMultiplayerPreparer()
	p.Running = func(Config) bool { return false }
	if _, err := p.Prepare(context.Background(), c, 0, 31201, []string{NativePreviewEnv + "=1"}, nil); err == nil || !strings.Contains(err.Error(), "ABI 2") {
		t.Fatal("old adapter reached expensive preparation", err)
	}
	if IsDir(FilterDir(c)) || IsDir(SandboxSaveDir(c)) {
		t.Fatal("native preflight failure changed game state")
	}
}
