package launcher

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash/crc32"
	"math"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func syntheticSave(flags map[string][]uint32) []byte {
	hashes := make([]uint32, 0, len(flags))
	values := map[uint32][]uint32{}
	for k, v := range flags {
		h := crc32.ChecksumIEEE([]byte(k))
		hashes = append(hashes, h)
		values[h] = v
	}
	sort.Slice(hashes, func(i, j int) bool { return hashes[i] < hashes[j] })
	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, []uint32{0x15, 0xffffffff, 1})
	for _, h := range hashes {
		for _, v := range values[h] {
			binary.Write(&out, binary.LittleEndian, []uint32{h, v})
		}
	}
	binary.Write(&out, binary.LittleEndian, uint32(0xffffffff))
	return out.Bytes()
}
func fixtureFlags() map[string][]uint32 {
	return map[string][]uint32{
		"PlayerSavePos":        {math.Float32bits(1), math.Float32bits(2), math.Float32bits(3)},
		"PlayerSavePosMapType": make([]uint32, 8), "PlayerSavePosMapName": make([]uint32, 8),
		"IsGet_PlayerStole2": {0}, "FirstTouchdown": {0}, "IsGet_Obj_Magnetglove": {0},
		"StoryQuest_NotOurs": {0}, "TestInventoryArray": {11, 22, 33},
	}
}
func sandboxFixture(t *testing.T) (Config, string, []byte) {
	t.Helper()
	root := t.TempDir()
	c := Config{Backend: Savage, Executable: file(t, filepath.Join(root, "runtime", "savagewind.exe"), "fixture"), DataDir: filepath.Join(root, "Dados de João"), SandboxPlace: "hateno", KeepCampaign: true}
	gd := syntheticSave(fixtureFlags())
	seed := filepath.Join(c.DataDir, "save", "save_0", "0", "game_data.sav")
	file(t, seed, string(gd))
	file(t, filepath.Join(filepath.Dir(seed), "caption.sav"), string(syntheticSave(map[string][]uint32{"CaptionValue": {8}})))
	file(t, filepath.Join(filepath.Dir(filepath.Dir(seed)), "option.sav"), string(syntheticSave(map[string][]uint32{"OptionValue": {7}})))
	return c, seed, gd
}
func TestSandboxCopiesPatchesAndPreservesCampaign(t *testing.T) {
	c, seed, original := sandboxFixture(t)
	m, err := PrepareSandbox(c, false)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(seed)
	if !bytes.Equal(got, original) {
		t.Fatal("campaign changed")
	}
	h := sha256.Sum256(original)
	if m.SourceSHA256 != hex.EncodeToString(h[:]) {
		t.Fatal("wrong source evidence")
	}
	if m.CampaignRemoved {
		t.Fatal("must not claim campaign removal")
	}
	b, err := os.ReadFile(filepath.Join(sandboxSlot(c), "game_data.sav"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := parseFlagSave(b)
	if err != nil {
		t.Fatal(err)
	}
	get := func(name string) []uint32 {
		var v []uint32
		for _, o := range s.offsets[crc32.ChecksumIEEE([]byte(name))] {
			v = append(v, binary.LittleEndian.Uint32(b[o:]))
		}
		return v
	}
	if !reflect.DeepEqual(get("PlayerSavePos"), []uint32{math.Float32bits(3437.5), math.Float32bits(235.9), math.Float32bits(2101.5)}) {
		t.Fatal("spawn incorrect")
	}
	if !reflect.DeepEqual(get("IsGet_PlayerStole2"), []uint32{1}) || !reflect.DeepEqual(get("FirstTouchdown"), []uint32{1}) {
		t.Fatal("free movement flags missing")
	}
	if !reflect.DeepEqual(get("TestInventoryArray"), []uint32{11, 22, 33}) || !reflect.DeepEqual(get("StoryQuest_NotOurs"), []uint32{0}) {
		t.Fatal("changed unrelated state")
	}
	if !reflect.DeepEqual(get("IsGet_Obj_Magnetglove"), []uint32{0}) {
		t.Fatal("sandbox granted a rune implicitly")
	}
	opt, _ := os.ReadFile(filepath.Join(SandboxSaveDir(c), "save_0", "option.sav"))
	want, _ := os.ReadFile(filepath.Join(c.DataDir, "save", "save_0", "option.sav"))
	if !bytes.Equal(opt, want) {
		t.Fatal("options lost")
	}
}
func TestSandboxResumeDoesNotResetProgress(t *testing.T) {
	c, _, _ := sandboxFixture(t)
	if _, e := PrepareSandbox(c, false); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(sandboxSlot(c), "game_data.sav")
	before, _ := os.ReadFile(path)
	// A later in-game save is retained verbatim on normal launch.
	after := append([]byte(nil), before...)
	after[16] ^= 1
	os.WriteFile(path, after, 0600)
	c.SandboxPlace = "rito"
	if _, e := PrepareSandbox(c, false); e != nil {
		t.Fatal(e)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, after) {
		t.Fatal("resume reset existing sandbox")
	}
}
func TestSandboxRebuildBacksUpOnlySandbox(t *testing.T) {
	c, seed, original := sandboxFixture(t)
	if _, e := PrepareSandbox(c, false); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(sandboxSlot(c), "game_data.sav")
	old, _ := os.ReadFile(path)
	c.SandboxPlace = "kakariko"
	m, e := PrepareSandbox(c, true)
	if e != nil {
		t.Fatal(e)
	}
	backup, _ := os.ReadFile(filepath.Join(m.Backup, "save", "save_0", "0", "game_data.sav"))
	if !bytes.Equal(old, backup) {
		t.Fatal("backup was not retained")
	}
	now, _ := os.ReadFile(path)
	if bytes.Equal(old, now) {
		t.Fatal("new location not applied")
	}
	got, _ := os.ReadFile(seed)
	if !bytes.Equal(got, original) {
		t.Fatal("source changed")
	}
}
func TestFailedRebuildLeavesExistingSandbox(t *testing.T) {
	c, seed, _ := sandboxFixture(t)
	if _, e := PrepareSandbox(c, false); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(sandboxSlot(c), "game_data.sav")
	old, _ := os.ReadFile(path)
	os.WriteFile(seed, []byte("incomplete source"), 0600)
	if _, e := PrepareSandbox(c, true); e == nil {
		t.Fatal("accepted corrupt source")
	}
	now, _ := os.ReadFile(path)
	if !bytes.Equal(old, now) {
		t.Fatal("existing sandbox lost")
	}
	stages, _ := filepath.Glob(filepath.Join(c.DataDir, ".sandbox-stage-*"))
	if len(stages) > 0 {
		t.Fatal("left staged files")
	}
}
func TestSandboxRequiredShapeAndEndian(t *testing.T) {
	cases := []string{"missing-paraglider", "wrong-vector-size", "wrong-string-size", "big-endian", "unsorted", "bad-footer", "truncated"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			c, seed, _ := sandboxFixture(t)
			flags := fixtureFlags()
			switch name {
			case "missing-paraglider":
				delete(flags, "IsGet_PlayerStole2")
			case "wrong-vector-size":
				flags["PlayerSavePos"] = []uint32{1}
			case "wrong-string-size":
				flags["PlayerSavePosMapType"] = []uint32{1}
			}
			b := syntheticSave(flags)
			switch name {
			case "big-endian":
				for o := 0; o < len(b); o += 4 {
					b[o], b[o+3] = b[o+3], b[o]
					b[o+1], b[o+2] = b[o+2], b[o+1]
				}
			case "unsorted":
				copy(b[12:16], []byte{255, 255, 255, 255})
			case "bad-footer":
				b[len(b)-1] = 0
			case "truncated":
				b = b[:len(b)-1]
			}
			os.WriteFile(seed, b, 0600)
			if _, e := PrepareSandbox(c, false); e == nil {
				t.Fatal("accepted incompatible seed")
			}
			if IsDir(SandboxDir(c)) {
				t.Fatal("published partial sandbox")
			}
		})
	}
}
func TestActiveProfileAndLatestSlot(t *testing.T) {
	c, _, gd := sandboxFixture(t)
	file(t, filepath.Join(c.DataDir, "save_profile.txt"), "Teste com espaços\n")
	base := filepath.Join(c.DataDir, "save-profiles", "Teste com espaços", "save", "save_0")
	first := file(t, filepath.Join(base, "0", "game_data.sav"), string(gd))
	second := file(t, filepath.Join(base, "3", "game_data.sav"), string(gd))
	old := time.Now().Add(-time.Hour)
	os.Chtimes(first, old, old)
	got, e := FindSandboxSeed(c)
	if e != nil || got != second {
		t.Fatalf("profile/slot not selected: %s %v", got, e)
	}
	file(t, filepath.Join(c.DataDir, "save_profile.txt"), "../escape")
	if _, e = FindSandboxSeed(c); e == nil {
		t.Fatal("accepted traversal")
	}
}
func TestSandboxCannotUseItselfAsSeed(t *testing.T) {
	c, _, _ := sandboxFixture(t)
	if _, e := PrepareSandbox(c, false); e != nil {
		t.Fatal(e)
	}
	c.SandboxSeed = filepath.Join(sandboxSlot(c), "game_data.sav")
	if _, e := PrepareSandbox(c, true); e == nil {
		t.Fatal("accepted its own seed")
	}
}
func TestCampaignAndSandboxEnvironmentAreSeparate(t *testing.T) {
	c, _, _ := sandboxFixture(t)
	if _, e := PrepareSandbox(c, false); e != nil {
		t.Fatal(e)
	}
	base := []string{"PATH=keep", "savagewind_test_flags=FirstTouchdown=0", "SAVAGEWIND_TEST_WARP=castle", "SAVAGEWIND_TEST_TIME=22", "SAVAGEWIND_TEST_WEATHER=3", "SAVAGEWIND_ROMFS_OVERLAY=foreign", "SAVAGEWIND_SAVE_DIR=foreign", "SAVAGEWIND_CONTROL=11111"}
	campaign, e := BuildLaunch(c, false, base)
	if e != nil {
		t.Fatal(e)
	}
	for _, env := range campaign.Env {
		if strings.HasPrefix(strings.ToUpper(env), "SAVAGEWIND_TEST_") || strings.HasPrefix(env, "SAVAGEWIND_ROMFS_OVERLAY=") || strings.HasPrefix(env, "SAVAGEWIND_SAVE_DIR=") || strings.HasPrefix(env, "SAVAGEWIND_CONTROL=") {
			t.Fatal("sandbox state leaked to campaign", env)
		}
	}
	sandbox, e := BuildSandboxLaunch(c, 27777, base)
	if e != nil {
		t.Fatal(e)
	}
	joined := strings.Join(sandbox.Env, "\n")
	if !strings.Contains(joined, "SAVAGEWIND_SAVE_DIR="+SandboxSaveDir(c)) || !strings.Contains(joined, "SAVAGEWIND_CONTROL=27777") {
		t.Fatal("sandbox not isolated")
	}
	c.Backend = Native
	if _, e = BuildSandboxLaunch(c, 27777, base); e == nil {
		t.Fatal("unsupported backend accepted")
	}
}

type mockControl struct {
	mu       sync.Mutex
	commands []string
	polls    int
}

func startControl(t *testing.T, handle func(string) string) int {
	t.Helper()
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			c, e := l.Accept()
			if e != nil {
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer c.Close()
				c.SetDeadline(time.Now().Add(3 * time.Second))
				s := bufio.NewScanner(c)
				if s.Scan() {
					out := handle(s.Text())
					if out != "" {
						c.Write([]byte(out + "\n"))
					}
				}
			}()
		}
	}()
	t.Cleanup(func() { l.Close(); <-done; wg.Wait() })
	return l.Addr().(*net.TCPAddr).Port
}
func TestControlReadinessAndRejectedCommands(t *testing.T) {
	var m mockControl
	port := startControl(t, func(cmd string) string {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.commands = append(m.commands, cmd)
		return `{"ok":false,"error":"No player loaded"}`
	})
	c := ControlClient{Port: port, Timeout: time.Second}
	if _, e := c.Apply(context.Background(), 8); e == nil {
		t.Fatal("accepted title-screen action")
	}
	m.mu.Lock()
	if !reflect.DeepEqual(m.commands, []string{"pos"}) {
		t.Fatal(m.commands)
	}
	m.mu.Unlock()
	if _, e := c.Command(context.Background(), "pos\nrupees 9999"); e == nil {
		t.Fatal("accepted multiple lines")
	}
}
func TestControlErrorAndJSONValidation(t *testing.T) {
	for _, response := range []string{`not json`, `{"ok":true}`, `{"ok":false,"error":"unsupported"}`} {
		t.Run(response, func(t *testing.T) {
			port := startControl(t, func(string) string { return response })
			if _, e := (ControlClient{Port: port}).Command(context.Background(), "pos"); e == nil {
				t.Fatal("accepted invalid/error reply")
			}
		})
	}
}
func TestControlKitAndTeleportCompletion(t *testing.T) {
	var m mockControl
	port := startControl(t, func(cmd string) string {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.commands = append(m.commands, cmd)
		if cmd == "teleport status" {
			m.polls++
			if m.polls < 2 {
				return `{"ok":true,"data":{"state":"settling"}}`
			}
			return `{"ok":true,"data":{"state":"done"}}`
		}
		return `{"ok":true,"data":{}}`
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c := ControlClient{Port: port}
	if _, e := c.Apply(ctx, len(SandboxActions)-1); e != nil {
		t.Fatal(e)
	}
	if _, e := c.Teleport(ctx, "hateno", nil); e != nil {
		t.Fatal(e)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.polls != 2 {
		t.Fatal("did not await map loading")
	}
	want := []string{"pos", "give Weapon_Sword_001 1", "give Weapon_Shield_001 1", "give Item_Fruit_A 10", "pos", "goto 3437.5000 235.9000 2101.5000", "teleport status", "teleport status"}
	if !reflect.DeepEqual(m.commands, want) {
		t.Fatalf("commands: %v", m.commands)
	}
}
func TestControlCancellation(t *testing.T) {
	port := startControl(t, func(string) string { time.Sleep(200 * time.Millisecond); return "" })
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)
	start := time.Now()
	if _, e := (ControlClient{Port: port}).Command(ctx, "pos"); e == nil {
		t.Fatal("ignored cancellation")
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancellation did not unblock read")
	}
}
func TestSandboxManifestIsReadableJSON(t *testing.T) {
	c, _, _ := sandboxFixture(t)
	if _, e := PrepareSandbox(c, false); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(SandboxDir(c), "sandbox.json"))
	if e != nil {
		t.Fatal(e)
	}
	var m SandboxManifest
	if e = json.Unmarshal(b, &m); e != nil || m.SourceSHA256 == "" || m.Schema != 1 {
		t.Fatal("report missing", e)
	}
}

func TestTeleportAlreadyThereDoesNotPollStaleState(t *testing.T) {
	var m mockControl
	port := startControl(t, func(cmd string) string {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.commands = append(m.commands, cmd)
		if strings.HasPrefix(cmd, "goto ") {
			return `{"ok":true,"data":{"state":"already_there"}}`
		}
		return `{"ok":true,"data":{}}`
	})
	if _, e := (ControlClient{Port: port}).Teleport(context.Background(), "hateno", nil); e != nil {
		t.Fatal(e)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.commands) != 2 {
		t.Fatal("polled stale teleport state", m.commands)
	}
}
