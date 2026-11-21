package launcher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// A child copy of the test binary acts as the runtime CLI on both platforms.
func init() {
	if os.Getenv("HYRULEM_TEST_NATIVE_BUILDER") != "1" {
		return
	}
	for _, arg := range os.Args[1:] {
		if arg == "--hyrulem-gamelib-id" {
			fmt.Println("1234567890abcdef")
			os.Exit(0)
		}
		if arg == "--hyrulem-build-game" {
			root := os.Getenv("SAVAGEWIND_DATA_ROOT")
			dir := filepath.Join(root, "extracted")
			if os.Getenv("SAVAGEWIND_GAME_LIB") != "" {
				os.Exit(5)
			}
			os.MkdirAll(dir, 0700)
			for name, value := range map[string]string{"swgame.dll": "rebuilt", "swgame.dll.id": "1234567890abcdef\n", "build.count": "1"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0600); err != nil {
					os.Exit(6)
				}
			}
			os.Exit(0)
		}
	}
	os.Exit(7)
}
func TestNativeLibraryBuildAndReuse(t *testing.T) {
	t.Setenv("HYRULEM_TEST_NATIVE_BUILDER", "1")
	t.Setenv("SAVAGEWIND_GAME_LIB", "obsolete borrowed DLL")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{Backend: Savage, Executable: exe, DataDir: t.TempDir()}
	if err = ensureNativeLibrary(context.Background(), cfg, true, nil); err == nil {
		t.Fatal("rebuilt a running installation")
	}
	if err = ensureNativeLibrary(context.Background(), cfg, false, nil); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(cfg.DataDir, "extracted", "build.count")
	os.WriteFile(marker, []byte("kept"), 0600)
	if err = ensureNativeLibrary(context.Background(), cfg, false, nil); err != nil {
		t.Fatal(err)
	}
	value, _ := os.ReadFile(marker)
	if string(value) != "kept" {
		t.Fatal("rebuilt a compatible shared library")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = ensureNativeLibrary(ctx, cfg, false, nil); !errors.Is(err, context.Canceled) {
		t.Fatal("ignored cancellation", err)
	}
}
