package launcher

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMultiplayerInstancesIsolateSavesAndSettings(t *testing.T) {
	c, seed, original := sandboxFixture(t)
	file(t, filepath.Join(c.DataDir, "extracted", "exefs", "main"), "test")
	os.MkdirAll(filepath.Join(c.DataDir, "extracted", "romfs"), 0700)
	file(t, filepath.Join(c.DataDir, "extracted", "swgame.dll"), "test")
	file(t, filepath.Join(c.DataDir, "savagewind.ini"), "field_of_view = 80\n")
	file(t, filepath.Join(c.DataDir, "launch-flags.txt"), "SAVAGEWIND_SAVE_DIR=bad\n")
	if _, e := PrepareSandbox(c, false); e != nil {
		t.Fatal(e)
	}
	a, e := PrepareMultiplayerInstance(c, 0, 32101, []string{"SAVAGEWIND_CONTROL=99", "SAVAGEWIND_TEST_WARP=bad"})
	if e != nil {
		t.Fatal(e)
	}
	b, e := PrepareMultiplayerInstance(c, 1, 32102, nil)
	if e != nil {
		t.Fatal(e)
	}
	if a.Config.DataDir == b.Config.DataDir || SandboxSaveDir(a.Config) == SandboxSaveDir(b.Config) {
		t.Fatal("shared writable state")
	}
	for _, inst := range []MultiplayerInstance{a, b} {
		env := strings.Join(inst.Spec.Env, "\n")
		if !strings.Contains(env, "SAVAGEWIND_SETTINGS="+filepath.Join(inst.Config.DataDir, "savagewind.ini")) || !strings.Contains(env, "SAVAGEWIND_SAVE_DIR="+SandboxSaveDir(inst.Config)) || strings.Contains(env, "=bad") {
			t.Fatal("unsafe env", env)
		}
		if _, e := os.Lstat(filepath.Join(inst.Config.DataDir, "launch-flags.txt")); !os.IsNotExist(e) {
			t.Fatal("copied launch flags")
		}
		real, e := filepath.EvalSymlinks(filepath.Join(inst.Config.DataDir, "extracted"))
		if e != nil || !SamePath(real, filepath.Join(c.DataDir, "extracted")) {
			t.Fatal("game assets not linked", real, e)
		}
	}
	path := filepath.Join(sandboxSlot(a.Config), "game_data.sav")
	os.WriteFile(path, []byte("private change"), 0600)
	if _, e = PrepareMultiplayerInstance(c, 0, 32101, nil); e != nil {
		t.Fatal(e)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "private change" {
		t.Fatal("rejoin reset save")
	}
	got, _ = os.ReadFile(seed)
	if !bytes.Equal(got, original) {
		t.Fatal("campaign overwritten")
	}
	got, _ = os.ReadFile(filepath.Join(sandboxSlot(b.Config), "game_data.sav"))
	if string(got) == "private change" {
		t.Fatal("client 2 affected")
	}
}

type treeFinder []ProcessInfo

func (f treeFinder) Find(string, time.Time) []ProcessInfo { return []ProcessInfo(f) }
func TestInstanceFinderNeverAdoptsOtherClient(t *testing.T) {
	f := &InstanceFinder{source: treeFinder{{PID: 10}, {PID: 11, ParentPID: 10}, {PID: 20}, {PID: 21, ParentPID: 20}}}
	if len(f.Find("x", time.Time{})) != 0 {
		t.Fatal("unstarted finder sees processes")
	}
	f.TrackPID(10)
	found := f.Find("x", time.Time{})
	if len(found) != 2 || found[0].PID != 10 || found[1].PID != 11 {
		t.Fatal("crossed process trees", found)
	}
	f.source = treeFinder{{PID: 11, ParentPID: 10}, {PID: 21, ParentPID: 20}}
	found = f.Find("x", time.Time{})
	if len(found) != 1 || found[0].PID != 11 {
		t.Fatal("lost restart")
	}
}
