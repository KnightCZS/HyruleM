//go:build windows

package main

import (
	"encoding/binary"
	"hash/crc32"
	core "hyrulelauncher/internal/launcher"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func fixtureSave(flags map[string][]uint32) []byte {
	keys := make([]uint32, 0, len(flags))
	values := map[uint32][]uint32{}
	for k, v := range flags {
		hash := crc32.ChecksumIEEE([]byte(k))
		keys = append(keys, hash)
		values[hash] = v
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	out := make([]byte, 12)
	binary.LittleEndian.PutUint32(out, 0x15)
	binary.LittleEndian.PutUint32(out[4:], 0xffffffff)
	binary.LittleEndian.PutUint32(out[8:], 1)
	for _, k := range keys {
		for _, v := range values[k] {
			b := make([]byte, 8)
			binary.LittleEndian.PutUint32(b, k)
			binary.LittleEndian.PutUint32(b[4:], v)
			out = append(out, b...)
		}
	}
	return append(out, 0xff, 0xff, 0xff, 0xff)
}
func TestNativeWindowsRegression(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	root := t.TempDir()
	data := filepath.Join(root, "Dados do teste")
	write := func(path string, b []byte) {
		t.Helper()
		if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	exe := filepath.Join(root, "invalid-runtime.exe")
	write(exe, []byte("not a Windows executable"))
	for _, p := range []string{"extracted/exefs/main", "extracted/swgame.dll"} {
		write(filepath.Join(data, filepath.FromSlash(p)), []byte("synthetic fixture"))
	}
	os.MkdirAll(filepath.Join(data, "extracted", "romfs"), 0700)
	slot := filepath.Join(data, "save", "save_0", "0")
	write(filepath.Join(slot, "game_data.sav"), fixtureSave(map[string][]uint32{"PlayerSavePos": {0, 0, 0}, "PlayerSavePosMapType": make([]uint32, 8), "PlayerSavePosMapName": make([]uint32, 8), "IsGet_PlayerStole2": {0}, "FirstTouchdown": {0}}))
	write(filepath.Join(slot, "caption.sav"), fixtureSave(map[string][]uint32{"CaptionValue": {1}}))
	instance, _, _ := getModule.Call(0)
	a := &app{instance: instance, root: root, configPath: filepath.Join(root, "config.json"), config: core.Config{Backend: core.Savage, Executable: exe, DataDir: data, SandboxPlace: "hateno", KeepCampaign: true}, controls: map[int]uintptr{}, session: core.NewSession(nil), fonts: map[string]uintptr{}, dpi: 1, initial: true}
	previous := current
	current = a
	defer func() { current = previous }()
	a.bgBrush, _, _ = createBrush.Call(colorBG)
	a.editBrush, _, _ = createBrush.Call(colorEdit)
	a.makeFonts()
	name := wide("HyruleMRegressionWindow")
	wc := wndClass{Size: uint32(unsafe.Sizeof(wndClass{})), Proc: syscall.NewCallback(windowProc), Instance: instance, Background: a.bgBrush, ClassName: name}
	if atom, _, e := registerClass.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		t.Fatal(e)
	}
	a.window, _, _ = createWindow.Call(0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(wide("HyruleM UI regression - synthetic data"))), 0x02ef0000, 10, 10, 1000, 760, 0, 0, instance, 0)
	if a.window == 0 {
		t.Fatal("window did not initialize")
	}
	defer func() {
		if a.multiplayer != nil {
			a.multiplayer.shutdown()
		}
		destroyWindow.Call(a.window)
	}()
	a.buildControls()
	a.initial = false
	a.fillControls()
	a.refresh()
	showWindow.Call(a.window, 5)
	peek := user32.NewProc("PeekMessageW")
	pump := func() {
		for n := 0; n < 200; n++ {
			var m msg
			ok, _, _ := peek.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, 1)
			if ok == 0 {
				break
			}
			if m.Message != 0x12 {
				translateMessage.Call(uintptr(unsafe.Pointer(&m)))
				dispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
			}
		}
	}
	until := func(f func() bool) {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			pump()
			if f() {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatal("UI stopped responding or task did not complete", a.status)
	}
	// Thousands of worker updates must yield a bounded queue and one final status.
	for n := 0; n < 20000; n++ {
		a.post(uiEvent{kind: "progress", progress: core.Progress{Stage: "Synthetic preparation", Done: int64(n), Total: 20000}})
	}
	a.queueMu.Lock()
	queued := len(a.queue)
	a.queueMu.Unlock()
	if queued != 1 {
		t.Fatalf("progress queue grew to %d", queued)
	}
	pump()
	if !strings.Contains(a.status, "99%") {
		t.Fatal("last progress missing", a.status)
	}
	resources := user32.NewProc("GetGuiResources")
	process, _, _ := kernel32.NewProc("GetCurrentProcess").Call()
	before, _, _ := resources.Call(process, 0)
	for n := 0; n < 120; n++ {
		a.scroll = int32((n * 31) % 500)
		a.layout()
		updateWindow.Call(a.window)
		pump()
	}
	after, _, _ := resources.Call(process, 0)
	if after > before+20 {
		t.Fatalf("GDI resource leak: before=%d after=%d", before, after)
	}
	// Exercise the actual sandbox button and asynchronous start failure. No game
	// assets are used, and the synthetic executable must never start successfully.
	a.startSandbox(false)
	until(func() bool { return !a.preparing && !a.session.Busy() })
	if _, e := core.LoadSandbox(a.config); e != nil {
		t.Fatal("sandbox preparation broke", e)
	}
	if !a.statusError || !strings.Contains(a.status, "iniciar") {
		t.Fatal("original launch error lost", a.status)
	}
	// Bad private extracted link must be diagnosed before a connection is made.
	a.openMultiplayer()
	u := a.multiplayer
	if u == nil {
		t.Fatal("multiplayer menu missing")
	}
	if port, e := core.LocalControlPort(); e == nil {
		u.set(mpPort, strconv.Itoa(port))
		u.set(mpHostPort, strconv.Itoa(port))
	} else {
		t.Fatal(e)
	}
	u.host()
	if u.server == nil {
		t.Fatal("host could not start")
	}
	os.MkdirAll(filepath.Join(core.InstanceDataDir(a.config, 0), "extracted"), 0700)
	u.join(0)
	until(func() bool { return !u.slots[0].pending })
	status := textOf(u.controls[mpStatus1])
	if !strings.Contains(status, "Falha na preparação:") || strings.Contains(status, "Desconectado") {
		t.Fatal("original preparation error was hidden", status)
	}
	if u.server.Count() != 0 {
		t.Fatal("connected before validating local game")
	}
	if dir := os.Getenv("HYRULEM_UI_REVIEW_DIR"); dir != "" {
		for _, v := range []struct {
			page int
			name string
		}{{idHome, "home"}, {idSettingsPage, "settings"}, {idSandboxPage, "sandbox"}} {
			a.changePage(v.page)
			updateWindow.Call(a.window)
			pump()
			captureReview(t, a.window, filepath.Join(dir, v.name+".png"))
		}
		t.Setenv(core.NativePreviewEnv, "1")
		u.localTest = false
		u.layout()
		updateWindow.Call(u.window)
		pump()
		captureReview(t, u.window, filepath.Join(dir, "multiplayer.png"))
		a.changePage(idHome)
		suggested := rect{10, 10, 1510, 1150}
		windowProc(a.window, 0x2e0, 144|144<<16, uintptr(unsafe.Pointer(&suggested)))
		pump()
		captureReview(t, a.window, filepath.Join(dir, "home-150.png"))
		suggested = rect{10, 10, 1010, 770}
		windowProc(a.window, 0x2e0, 96|96<<16, uintptr(unsafe.Pointer(&suggested)))
		pump()
		u.localTest = true
		u.layout()
		u.scroll = u.px(180)
		u.layout()
		updateWindow.Call(u.window)
		pump()
		captureReview(t, u.window, filepath.Join(dir, "local-two.png"))
	}
	t.Log("progress coalescing, scroll/GDI stability, sandbox start failure and multiplayer preparation error passed")
}
