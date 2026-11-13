package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func file(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestSavageLaunchUsesFlagNotGamePath(t *testing.T) {
	dir := t.TempDir()
	exe := file(t, filepath.Join(dir, "Instalação com espaços", "savagewind.exe"), "fixture")
	data := filepath.Join(dir, "Dados de João")
	c := Config{Backend: Savage, Executable: exe, DataDir: data}
	original := []string{"PATH=test", "SAVAGEWIND_CONTROL=9999", "SAVAGEWIND_DEV=1", "SAVAGEWIND_LAUNCH_FLAGS_APPLIED=old:options", "SAVAGEWIND_DATA_ROOT=old", "BOTW_USER_ROOT=foreign"}
	before := append([]string(nil), original...)
	spec, err := BuildLaunch(c, false, original)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(spec.Args, []string{"--game"}) {
		t.Fatalf("wrong argument contract: %q", spec.Args)
	}
	if spec.Dir != filepath.Dir(exe) {
		t.Fatal("incorrect working directory")
	}
	joined := strings.Join(spec.Env, "\n")
	if !strings.Contains(joined, "SAVAGEWIND_DATA_ROOT="+data) || !strings.Contains(joined, "SAVAGEWIND_NO_UPDATE_CHECK=1") {
		t.Fatal("missing runtime isolation")
	}
	if strings.Contains(joined, "CONTROL=") || strings.Contains(joined, "BOTW_USER_ROOT=") || strings.Contains(joined, "DEV=") || strings.Contains(joined, "LAUNCH_FLAGS_APPLIED=") {
		t.Fatal("unexpected foreign session state")
	}
	if !reflect.DeepEqual(original, before) {
		t.Fatal("changed parent environment")
	}
	setup, err := BuildLaunch(c, true, original)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(setup.Args, []string{"--config"}) {
		t.Fatal("wrong setup arguments")
	}
}
func TestNativePathIsSingleArgument(t *testing.T) {
	root := t.TempDir()
	exe := file(t, filepath.Join(root, "Runtime", "BotWRecomp.exe"), "fixture")
	game := filepath.Join(root, "Jogo & caracteres $(nao-executar)")
	file(t, filepath.Join(game, "code", "U-King.rpx"), "fixture")
	os.MkdirAll(filepath.Join(game, "content"), 0700)
	spec, err := BuildLaunch(Config{Backend: Native, Executable: exe, GameDir: game, DataDir: filepath.Join(root, "dados")}, false, []string{"PATH=test"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(spec.Args, []string{"--game", game, "--renderer", "vulkan"}) {
		t.Fatal(spec.Args)
	}
}
func TestNativeSetupBeforeGameWasCompiled(t *testing.T) {
	root := t.TempDir()
	setup := file(t, filepath.Join(root, "BotW-Native-Setup.exe"), "fixture")
	spec, err := BuildLaunch(Config{Backend: Native, Executable: filepath.Join(root, "BotWRecomp.exe"), DataDir: filepath.Join(root, "dados")}, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Executable != setup {
		t.Fatal("setup was not discovered")
	}
}
func TestValidationRejectsWrongGameFolder(t *testing.T) {
	root := t.TempDir()
	exe := file(t, filepath.Join(root, "BotWRecomp.exe"), "fixture")
	_, err := BuildLaunch(Config{Backend: Native, Executable: exe, GameDir: root, DataDir: root}, false, nil)
	if err == nil {
		t.Fatal("accepted a directory without game code")
	}
}
func TestConfigurationPreservesUnicodeAndPriorVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configuração", "config.json")
	cfg := Config{Backend: Savage, Executable: "João & Ana", DataDir: "aventura"}
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	actual, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if actual.Executable != cfg.Executable {
		t.Fatal("Unicode path was lost")
	}
	cfg.Executable = "outra instalação"
	if err = SaveConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	backup, err := LoadConfig(path + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if backup.Executable != actual.Executable {
		t.Fatal("previous configuration was not preserved")
	}
}
func TestBrokenConfigIsNotOverwrittenOnLoad(t *testing.T) {
	p := file(t, filepath.Join(t.TempDir(), "config.json"), "{broken")
	if _, err := LoadConfig(p); err == nil {
		t.Fatal("accepted invalid config")
	}
	data, _ := os.ReadFile(p)
	if string(data) != "{broken" {
		t.Fatal("changed broken file")
	}
}
func makeZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, body := range entries {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "package.zip")
	if err := os.WriteFile(p, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestZIPRejectsTraversalBeforeWritingAnything(t *testing.T) {
	archive := makeZip(t, map[string]string{"good.txt": "ok", "../outside.txt": "bad"})
	out := filepath.Join(t.TempDir(), "install")
	if err := ExtractZIP(context.Background(), archive, out); err == nil {
		t.Fatal("accepted directory traversal")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("wrote files before validating the archive")
	}
}
func TestZIPRejectsWindowsPathVariants(t *testing.T) {
	for _, name := range []string{"..\\outside", "C:/Windows/file", "/absolute", "CON.txt", "folder/file.", "folder/aux", "folder/a:b"} {
		t.Run(name, func(t *testing.T) {
			if _, err := safeZIPPath(name); err == nil {
				t.Fatal("accepted invalid Windows path")
			}
		})
	}
}
func TestZIPRejectsCaseCollision(t *testing.T) {
	archive := makeZip(t, map[string]string{"Run.exe": "one", "run.exe": "two"})
	if err := ExtractZIP(context.Background(), archive, filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatal("accepted case-insensitive collision")
	}
}
func TestZIPInstallAndExecutableDiscovery(t *testing.T) {
	archive := makeZip(t, map[string]string{"Savage/savagewind.exe": "fixture", "Savage/zig/tool": "compiler"})
	out := filepath.Join(t.TempDir(), "out")
	if err := ExtractZIP(context.Background(), archive, out); err != nil {
		t.Fatal(err)
	}
	got, err := FindSavageExecutable(out)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(out, "Savage", "savagewind.exe") {
		t.Fatal(got)
	}
}
func TestCancelledExtraction(t *testing.T) {
	archive := makeZip(t, map[string]string{"savagewind.exe": "fixture"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ExtractZIP(ctx, archive, filepath.Join(t.TempDir(), "out")); err != context.Canceled {
		t.Fatalf("wanted cancellation, got %v", err)
	}
}
func TestSessionStartsRealChildWithCorrectEnvironment(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "Árvore com espaços")
	os.MkdirAll(root, 0700)
	result := filepath.Join(root, "result.json")
	spec := LaunchSpec{Executable: exe, Dir: root, Args: []string{"-test.run=TestHelperProcess", "--", result, "argumento & literal"}, Env: append(os.Environ(), "HLR_HELPER=1", "HLR_VALUE=valor com espaços")}
	session := NewSession(nil)
	var mu sync.Mutex
	var events []Event
	if err = session.Start(spec, filepath.Join(root, "process.log"), func(e Event) { mu.Lock(); events = append(events, e); mu.Unlock() }); err != nil {
		t.Fatal(err)
	}
	if err = session.Start(spec, filepath.Join(root, "process2.log"), func(Event) {}); err == nil {
		t.Fatal("allowed duplicate launch")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = session.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	var data map[string]string
	b, err := os.ReadFile(result)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &data); err != nil {
		t.Fatal(err)
	}
	if data["cwd"] != root || data["value"] != "valor com espaços" || data["arg"] != "argumento & literal" {
		t.Fatal(data)
	}
}
func TestSessionReportsNonzeroExit(t *testing.T) {
	exe, _ := os.Executable()
	session := NewSession(nil)
	events := make(chan Event, 5)
	root := t.TempDir()
	err := session.Start(LaunchSpec{Executable: exe, Dir: root, Args: []string{"-test.run=TestHelperProcess"}, Env: append(os.Environ(), "HLR_HELPER=fail")}, filepath.Join(root, "log"), func(e Event) { events <- e })
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = session.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-events:
		if e.Kind != "started" {
			t.Fatal(e)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case e := <-events:
		if e.Kind != "failed" || e.Code != 7 {
			t.Fatal(e)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
func TestHelperProcess(t *testing.T) {
	if os.Getenv("HLR_HELPER") == "" {
		return
	}
	if os.Getenv("HLR_HELPER") == "fail" {
		os.Exit(7)
	}
	time.Sleep(120 * time.Millisecond)
	cwd, _ := os.Getwd()
	i := 0
	for index, arg := range os.Args {
		if arg == "--" {
			i = index + 1
			break
		}
	}
	if i == 0 || i+1 >= len(os.Args) {
		os.Exit(8)
	}
	data, _ := json.Marshal(map[string]string{"cwd": cwd, "value": os.Getenv("HLR_VALUE"), "arg": os.Args[i+1]})
	if err := os.WriteFile(os.Args[i], data, 0600); err != nil {
		os.Exit(9)
	}
	os.Exit(0)
}
func TestMissingProgramDoesNotLockSession(t *testing.T) {
	s := NewSession(nil)
	err := s.Start(LaunchSpec{Executable: filepath.Join(t.TempDir(), "missing"), Dir: t.TempDir()}, filepath.Join(t.TempDir(), "log"), func(Event) {})
	if err == nil || s.Busy() {
		t.Fatal("failed start left session locked")
	}
}

func TestCorruptLocalPackageNeverPublishesInstall(t *testing.T) {
	root := t.TempDir()
	archive := file(t, filepath.Join(t.TempDir(), "runtime.zip"), "not the official package")
	if _, err := InstallSavageFrom(context.Background(), root, archive, nil); err == nil {
		t.Fatal("accepted a corrupt package")
	}
	destination := filepath.Join(root, "runtimes", "savagewind-"+SavageTag)
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("published a partial installation")
	}
}
func TestBundledRuntimeInstallsWithoutNetwork(t *testing.T) {
	archive := os.Getenv("HYRULE_RUNTIME_ARCHIVE")
	if archive == "" {
		t.Skip("set HYRULE_RUNTIME_ARCHIVE to verify the real upstream archive")
	}
	root := t.TempDir()
	installed, err := InstallSavageFrom(context.Background(), root, archive, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !IsFile(installed.Executable) {
		t.Fatal("runtime executable was not installed")
	}
	again, err := InstallSavageFrom(context.Background(), root, "missing-archive", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Reused || again.Executable != installed.Executable {
		t.Fatal("verified installation was not reused")
	}
}
