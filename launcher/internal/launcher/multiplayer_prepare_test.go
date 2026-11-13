package launcher

import (
	"bytes"
	"context"
	f "hyrulelauncher/internal/gamefmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func readyFilterFixture(t *testing.T) Config {
	t.Helper()
	c, _ := filterFixture(t)
	file(t, filepath.Join(c.DataDir, "extracted", "exefs", "main"), "synthetic NSO")
	file(t, filepath.Join(c.DataDir, "extracted", "swgame.dll"), "synthetic game library")
	return c
}
func TestMultiplayerAutomaticallyPreparesMissingFilter(t *testing.T) {
	c := readyFilterFixture(t)
	p := NewMultiplayerPreparer()
	p.Running = func(Config) bool { return false }
	seed, e := FindSandboxSeed(c)
	if e != nil {
		t.Fatal(e)
	}
	before, e := os.ReadFile(seed)
	if e != nil {
		t.Fatal(e)
	}
	if IsFile(filepath.Join(FilterDir(c), "filter.json")) {
		t.Fatal("fixture should start without filter.json (issue #2)")
	}
	inst, e := p.Prepare(context.Background(), c, 0, 31201, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	report, e := CampaignFilterReport(c)
	if e != nil {
		t.Fatal("filter was not published automatically", e)
	}
	if _, e := LoadSandbox(c); e != nil {
		t.Fatal("sandbox was not prepared", e)
	}
	if !strings.Contains(strings.Join(inst.Spec.Env, "\n"), "SAVAGEWIND_ROMFS_OVERLAY="+FilterOverlay(c)) {
		t.Fatal("generated filter was not attached")
	}
	after, e := os.ReadFile(seed)
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("campaign changed", e)
	}
	second, e := p.Prepare(context.Background(), c, 1, 31202, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	cached, e := CampaignFilterReport(c)
	if e != nil || !cached.Created.Equal(report.Created) {
		t.Fatal("valid filter was unnecessarily rebuilt", e)
	}
	if inst.Config.DataDir == second.Config.DataDir {
		t.Fatal("clients share writable data")
	}
}
func TestMultiplayerPreparationSerializesClientsAndCancelsWait(t *testing.T) {
	c := readyFilterFixture(t)
	p := NewMultiplayerPreparer()
	p.Running = func(Config) bool { return false }
	p.gate <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := p.Prepare(ctx, c, 0, 31201, nil, nil); e != context.Canceled {
		t.Fatal("cancelled waiter did not return", e)
	}
	<-p.gate
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for slot := 0; slot < 2; slot++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			_, e := p.Prepare(context.Background(), c, slot, 31201+slot, nil, nil)
			errs <- e
		}(slot)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if IsDir(FilterDir(c) + "-previous") {
		t.Fatal("concurrent clients rebuilt the same filter twice")
	}
}
func TestMultiplayerNeverRebuildsFilterUnderRunningGame(t *testing.T) {
	c := readyFilterFixture(t)
	if _, e := PrepareSandbox(c, false); e != nil {
		t.Fatal(e)
	}
	p := NewMultiplayerPreparer()
	p.Running = func(Config) bool { return true }
	if _, e := p.Prepare(context.Background(), c, 0, 31201, nil, nil); e == nil || !strings.Contains(e.Error(), "feche os jogos") {
		t.Fatal("missing filter rebuilt during gameplay", e)
	}
	if IsDir(FilterDir(c)) {
		t.Fatal("published a filter while game was running")
	}
	p.Running = func(Config) bool { return false }
	if _, e := p.Prepare(context.Background(), c, 0, 31201, nil, nil); e != nil {
		t.Fatal(e)
	}
	p.Running = func(Config) bool { return true }
	if _, e := p.Prepare(context.Background(), c, 1, 31202, nil, nil); e != nil {
		t.Fatal("running client prevented reuse of valid filter", e)
	}
}
func TestCampaignFilterAcceptsIndependentEmptyQuest(t *testing.T) {
	c := readyFilterFixture(t)
	root := filepath.Join(c.DataDir, "extracted", "romfs")
	titlePath := filepath.Join(root, "Pack", "TitleBG.pack")
	b, e := os.ReadFile(titlePath)
	if e != nil {
		t.Fatal(e)
	}
	pack, e := f.ReadSARC(b)
	if e != nil {
		t.Fatal(e)
	}
	quests, _ := pack.Get("Quest/QuestProduct.sbquestpack")
	doc, _, e := readDoc(quests)
	if e != nil {
		t.Fatal(e)
	}
	doc.Root.List = append(doc.Root.List, f.Dict(map[string]*f.Node{"Name": f.Text("SyntheticEmptyQuest"), "Type": f.Text("Main")}))
	questBytes, e := f.WriteBYML(doc)
	if e != nil {
		t.Fatal(e)
	}
	empty, e := os.ReadFile(filepath.Join("..", "gamefmt", "testdata", "synthetic-empty-quest.bfevfl"))
	if e != nil {
		t.Fatal(e)
	}
	files := map[string][]byte{}
	for _, entry := range pack.Entries {
		data, _ := pack.Get(entry.Name)
		files[entry.Name] = data
	}
	files["Quest/QuestProduct.sbquestpack"] = f.PackYaz0(questBytes)
	files["EventFlow/SyntheticEmptyQuest.bfevfl"] = empty
	source, e := f.WriteSARC(files)
	if e != nil {
		t.Fatal(e)
	}
	file(t, titlePath, string(source))
	report, e := PrepareCampaignFilter(context.Background(), c, false, nil)
	if e != nil {
		t.Fatal("empty quest blocks complete filter (issue #1)", e)
	}
	if len(report.RemovedMain) != 2 {
		t.Fatal("empty main quest was not removed from quest log")
	}
	out, e := os.ReadFile(filepath.Join(FilterOverlay(c), "Pack", "TitleBG.pack"))
	if e != nil {
		t.Fatal(e)
	}
	patched, e := f.ReadSARC(out)
	if e != nil {
		t.Fatal(e)
	}
	preserved, ok := patched.Get("EventFlow/SyntheticEmptyQuest.bfevfl")
	if !ok || !bytes.Equal(preserved, empty) {
		t.Fatal("invented replacement for an already empty flow")
	}
}
