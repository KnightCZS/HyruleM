package launcher

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	f "hyrulelauncher/internal/gamefmt"
)

func bymlFixture(t *testing.T, root *f.Node) []byte {
	t.Helper()
	b, e := f.WriteBYML(f.Document{Root: root, Little: true, Version: 2})
	if e != nil {
		t.Fatal(e)
	}
	return f.PackYaz0(b)
}
func packFixture(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	b, e := f.WriteSARC(files)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func filterFixture(t *testing.T) (Config, map[string][]byte) {
	t.Helper()
	c, _, _ := sandboxFixture(t)
	c.KeepCampaign = false
	quest := func(name, kind string) *f.Node {
		return f.Dict(map[string]*f.Node{"Name": f.Text(name), "Type": f.Text(kind), "Steps": f.Arr(f.Dict(map[string]*f.Node{"Name": f.Text("Ready"), "NextFlag": f.Text(name + "_Activated")}))})
	}
	quests := bymlFixture(t, f.Arr(quest("SyntheticMain", "Main"), quest("SyntheticSide", "Mini"), quest("SyntheticShrine", "Sub")))
	eventFields := map[string]*f.Node{}
	for _, flow := range []string{"SyntheticMain", "SyntheticSide", "SyntheticShrine", "Demo103_0", "Demo126_0", "Demo001_0", "Demo005_1", "Demo707_0", "Demo719_0", "Demo300_0", "Demo650_0"} {
		eventFields[flow+"<"+flow+">"] = f.Dict(map[string]*f.Node{"is_timeline": f.Boolean(true), "mode": f.Text("FullPackage"), "movie": f.Text(flow + ".mp4")})
	}
	eventFields["SyntheticMain<Talk>"] = f.Dict(map[string]*f.Node{"mode": f.Text("Seamless")})
	info := bymlFixture(t, f.Dict(eventFields))
	actor := func(id uint64, unit, flow string) *f.Node {
		return f.Dict(map[string]*f.Node{"HashId": {Kind: f.Uint, Bits: id}, "UnitConfigName": f.Text(unit), "!Parameters": f.Dict(map[string]*f.Node{"EventFlowName": f.Text(flow), "EventFlowEntryName": f.Text(flow)})})
	}
	side := actor(2, "EventTag", "SyntheticSide")
	side.Fields["LinksToObj"] = f.Arr(f.Dict(map[string]*f.Node{"DestUnitHashId": {Kind: f.Uint, Bits: 1}, "DefinitionName": f.Text("Delete")}), f.Dict(map[string]*f.Node{"DestUnitHashId": {Kind: f.Uint, Bits: 3}, "DefinitionName": f.Text("Keep")}))
	world := bymlFixture(t, f.Dict(map[string]*f.Node{"Objs": f.Arr(actor(1, "EventTag", "Demo103_0"), side, actor(3, "Npc_Synthetic", "Demo103_0")), "Rails": f.Arr()}))
	registry := (&f.RSTB{Hashes: map[uint32]uint32{1234: 777}, Names: map[string]uint32{}}).Bytes()
	mainFlow, e := f.NoopFlow("SyntheticMain", []string{"SyntheticMain", "Talk", "InternalHelper"})
	if e != nil {
		t.Fatal(e)
	}
	sharedFlow, e := f.NoopFlow("SyntheticMain", []string{"SyntheticMain", "Talk", "InternalHelper", "SharedHelper"})
	if e != nil {
		t.Fatal(e)
	}
	assets := map[string][]byte{
		"Pack/Bootup.pack":                     packFixture(t, map[string][]byte{"Event/EventInfo.product.sbyml": info, "GameData/Unrelated": []byte("keep gamedata")}),
		"Pack/TitleBG.pack":                    packFixture(t, map[string][]byte{"Quest/QuestProduct.sbquestpack": quests, "EventFlow/SyntheticMain.bfevfl": mainFlow, "EventFlow/Demo001_0.bfevfl": []byte("cooking unchanged"), "EventFlow/SyntheticSide.bfevfl": []byte("side unchanged")}),
		"Event/SyntheticMain.sbeventpack":      f.PackYaz0(packFixture(t, map[string][]byte{"EventFlow/SyntheticMain.bfevfl": mainFlow, "Other/SharedSecondary": []byte("preserve shared asset")})),
		"Map/MainField/I-7/I-7_Dynamic.smubin": world,
		rstbPath:                               f.PackYaz0(registry),
	}
	assets["Event/SyntheticSide.sbeventpack"] = f.PackYaz0(packFixture(t, map[string][]byte{"EventFlow/SyntheticSide.bfevfl": []byte("side unchanged"), "EventFlow/SyntheticMain.bfevfl": sharedFlow}))
	for rel, b := range assets {
		file(t, filepath.Join(c.DataDir, "extracted", "romfs", filepath.FromSlash(rel)), string(b))
	}
	return c, assets
}
func readOverlayDoc(t *testing.T, c Config, packName, entryName string) *f.Node {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(FilterOverlay(c), filepath.FromSlash(packName)))
	if e != nil {
		t.Fatal(e)
	}
	if entryName != "" {
		raw, _, e := f.Unpack(b)
		if e != nil {
			t.Fatal(e)
		}
		pack, e := f.ReadSARC(raw)
		if e != nil {
			t.Fatal(e)
		}
		var ok bool
		b, ok = pack.Get(entryName)
		if !ok {
			t.Fatal("missing", entryName)
		}
	}
	d, _, e := readDoc(b)
	if e != nil {
		t.Fatal(e)
	}
	return d.Root
}
func TestCampaignFilterPreservesSideQuestsAndOriginalAssets(t *testing.T) {
	c, assets := filterFixture(t)
	r, e := PrepareCampaignFilter(context.Background(), c, false, nil)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(r.RemovedMain, []string{"SyntheticMain"}) || !reflect.DeepEqual(r.KeptSide, []string{"SyntheticSide"}) || !reflect.DeepEqual(r.KeptShrine, []string{"SyntheticShrine"}) {
		t.Fatal("wrong quest categories", r)
	}
	if r.RemovedMapTriggers != 1 || r.VerifiedInGame {
		t.Fatal("wrong coverage or verification claim")
	}
	for rel, want := range assets {
		got, e := os.ReadFile(filepath.Join(c.DataDir, "extracted", "romfs", filepath.FromSlash(rel)))
		if e != nil || !bytes.Equal(got, want) {
			t.Fatal("modified original asset", rel, e)
		}
	}
	table := readOverlayDoc(t, c, "Pack/TitleBG.pack", "Quest/QuestProduct.sbquestpack")
	if len(table.List) != 2 || table.List[0].Get("Type").Str() != "Mini" || table.List[1].Get("Type").Str() != "Sub" {
		t.Fatal("main filter corrupted side quests")
	}
	originalPack, _ := f.ReadSARC(assets["Pack/TitleBG.pack"])
	original, _ := originalPack.Get("Quest/QuestProduct.sbquestpack")
	before, _, _ := readDoc(original)
	if !reflect.DeepEqual(before.Root.List[1:], table.List) {
		t.Fatal("secondary quest data changed")
	}
	info := readOverlayDoc(t, c, "Pack/Bootup.pack", "Event/EventInfo.product.sbyml")
	for _, name := range []string{"Demo103_0", "Demo126_0", "Demo650_0", "SyntheticMain"} {
		n := info.Get(name + "<" + name + ">")
		if n.Get("movie") != nil || n.Get("is_timeline").Bits != 0 || n.Get("mode").Str() != "FullPackage" {
			t.Fatal("cinematic not replaced", name)
		}
	}
	for _, name := range []string{"Demo001_0", "Demo005_1", "Demo707_0", "Demo719_0", "Demo300_0", "SyntheticSide"} {
		if info.Get(name+"<"+name+">").Get("movie") == nil {
			t.Fatal("protected event changed", name)
		}
	}
	world := readOverlayDoc(t, c, "Map/MainField/I-7/I-7_Dynamic.smubin", "")
	actors := world.Get("Objs").List
	if len(actors) != 2 || actors[1].Get("UnitConfigName").Str() != "Npc_Synthetic" {
		t.Fatal("NPC was removed instead of trigger")
	}
	if len(actors[0].Get("LinksToObj").List) != 1 {
		t.Fatal("dangling trigger link")
	}
	b, e := os.ReadFile(filepath.Join(FilterOverlay(c), "Event", "Demo126_0.sbeventpack"))
	if e != nil {
		t.Fatal("movie-only event has no replacement pack", e)
	}
	raw, _, e := f.Unpack(b)
	if e != nil {
		t.Fatal(e)
	}
	p, e := f.ReadSARC(raw)
	if e != nil {
		t.Fatal(e)
	}
	if flow, ok := p.Get("EventFlow/Demo126_0.bfevfl"); !ok || !bytes.HasPrefix(flow, []byte("BFEVFL")) {
		t.Fatal("movie-only flow absent")
	}
	if demo, ok := p.Get("Demo/Demo126_0.bdemo"); !ok || !bytes.HasPrefix(demo, []byte("AAMP")) {
		t.Fatal("safe descriptor absent")
	}
}
func TestCampaignFilterCacheAndForceRebuild(t *testing.T) {
	c, _ := filterFixture(t)
	first, e := PrepareCampaignFilter(context.Background(), c, false, nil)
	if e != nil {
		t.Fatal(e)
	}
	next, e := PrepareCampaignFilter(context.Background(), c, false, nil)
	if e != nil || first.Created != next.Created {
		t.Fatal("cache not reused", e)
	}
	forced, e := PrepareCampaignFilter(context.Background(), c, true, nil)
	if e != nil || forced.Created == first.Created {
		t.Fatal("force rebuild failed", e)
	}
	if !IsFile(filepath.Join(FilterDir(c)+"-previous", "filter.json")) {
		t.Fatal("previous filter not retained")
	}
	// Invalid source after a successful build must not replace the last good layer.
	prior, _ := os.ReadFile(filepath.Join(FilterDir(c), "filter.json"))
	file(t, filepath.Join(c.DataDir, "extracted", "romfs", "Pack", "Bootup.pack"), "corrupt")
	if _, e = PrepareCampaignFilter(context.Background(), c, false, nil); e == nil {
		t.Fatal("accepted corrupt source")
	}
	now, _ := os.ReadFile(filepath.Join(FilterDir(c), "filter.json"))
	if !bytes.Equal(prior, now) {
		t.Fatal("last good filter lost")
	}
}
func TestCampaignFilterPolicyCanKeepIndividualScene(t *testing.T) {
	c, _ := filterFixture(t)
	p := DefaultCampaignPolicy()
	p.KeepShrineQuests = false
	p.AllowFlows = []string{"Demo126_0"}
	b, _ := json.Marshal(p)
	file(t, FilterPolicyPath(c), string(b))
	r, e := PrepareCampaignFilter(context.Background(), c, false, nil)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.RemovedShrine) != 1 || len(r.KeptShrine) != 0 {
		t.Fatal("shrine policy ignored")
	}
	for _, s := range r.BlockedFlows {
		if s == "Demo126_0" {
			t.Fatal("allow exception ignored")
		}
	}
	table := readOverlayDoc(t, c, "Pack/TitleBG.pack", "Quest/QuestProduct.sbquestpack")
	if len(table.List) != 1 || table.List[0].Get("Type").Str() != "Mini" {
		t.Fatal("expected only side quest")
	}
}
func TestCampaignFilterCancelledOrInvalidPolicyDoesNotPublish(t *testing.T) {
	c, _ := filterFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := PrepareCampaignFilter(ctx, c, false, nil); e == nil {
		t.Fatal("ignored cancellation")
	}
	if IsDir(FilterDir(c)) {
		t.Fatal("published cancelled layer")
	}
	file(t, FilterPolicyPath(c), `{"Schema":1,"UnknownProperty":true}`)
	if _, e := PrepareCampaignFilter(context.Background(), c, false, nil); e == nil {
		t.Fatal("silently ignored invalid policy")
	}
}
func TestCampaignFilterLaunchWiresOnlySandbox(t *testing.T) {
	c, _ := filterFixture(t)
	if _, e := PrepareSandbox(c, false); e != nil {
		t.Fatal(e)
	}
	if _, e := BuildSandboxLaunch(c, 28888, nil); e == nil {
		t.Fatal("launched without requested filter")
	}
	if _, e := PrepareCampaignFilter(context.Background(), c, false, nil); e != nil {
		t.Fatal(e)
	}
	s, e := BuildSandboxLaunch(c, 28888, nil)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(strings.Join(s.Env, "\n"), "SAVAGEWIND_ROMFS_OVERLAY="+FilterOverlay(c)) {
		t.Fatal("filter was generated but not applied")
	}
	normal, e := BuildLaunch(c, false, s.Env)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(strings.Join(normal.Env, "\n"), "SAVAGEWIND_ROMFS_OVERLAY=") {
		t.Fatal("filter leaked into campaign")
	}
	c.KeepCampaign = true
	legacy, e := BuildSandboxLaunch(c, 28888, s.Env)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(strings.Join(legacy.Env, "\n"), "SAVAGEWIND_ROMFS_OVERLAY=") {
		t.Fatal("compatibility mode kept filter")
	}
}
func TestCampaignFilterOutputTamperingInvalidatesCache(t *testing.T) {
	c, _ := filterFixture(t)
	r, e := PrepareCampaignFilter(context.Background(), c, false, nil)
	if e != nil {
		t.Fatal(e)
	}
	out := r.Outputs[0]
	full := filepath.Join(FilterOverlay(c), filepath.FromSlash(out.Path))
	old, _ := os.ReadFile(full)
	sum := sha256.Sum256(old)
	os.WriteFile(full, []byte("broken"), 0600)
	next, e := PrepareCampaignFilter(context.Background(), c, false, nil)
	if e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(full)
	if sha256.Sum256(after) != sum || !next.Created.After(r.Created) {
		t.Fatal("did not regenerate changed layer")
	}
}
func TestKnownSystemAndSideEventsRemainAllowed(t *testing.T) {
	cat := newCatalog()
	p := DefaultCampaignPolicy()
	for _, s := range []string{"Demo000_0", "Demo001_0", "Demo005_1", "Demo008_0", "Demo011_0", "Demo033_0", "Demo707_0", "Demo719_0", "Demo300_0", "Demo599_0", "Npc_Shop"} {
		if cat.blocked(s, p) {
			t.Fatal("blocked protected workflow", s)
		}
	}
	for _, s := range []string{"Demo103_0", "Demo126_0", "Demo650_0", "Demo710_0", "Demo050_0"} {
		if !cat.blocked(s, p) {
			t.Fatal("did not classify story scene", s)
		}
	}
}
func TestCampaignFilterChangingAssetsInvalidatesSignature(t *testing.T) {
	c, _ := filterFixture(t)
	first, e := PrepareCampaignFilter(context.Background(), c, false, nil)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(c.DataDir, "extracted", "romfs", "Pack", "TitleBG.pack")
	now := time.Now().Add(2 * time.Second)
	os.Chtimes(path, now, now)
	next, e := PrepareCampaignFilter(context.Background(), c, false, nil)
	if e != nil || next.Signature == first.Signature {
		t.Fatal("asset change was not detected", e)
	}
}

func TestSandboxRejectsRuntimeFlagOverridesWithoutChangingOptions(t *testing.T) {
	c, _, _ := sandboxFixture(t)
	p := filepath.Join(c.DataDir, "launch-flags.txt")
	benign := "SAVAGEWIND_FSR3=1\n# SAVAGEWIND_SAVE_DIR=comment\n"
	file(t, p, benign)
	if e := ValidateSandboxFlags(c); e != nil {
		t.Fatal("rejected graphics settings", e)
	}
	bad := benign + "SAVAGEWIND_ROMFS_OVERLAY=other-mod\n"
	file(t, p, bad)
	if e := ValidateSandboxFlags(c); e == nil {
		t.Fatal("runtime could override launcher overlay")
	}
	got, _ := os.ReadFile(p)
	if string(got) != bad {
		t.Fatal("rewrote saved user options")
	}
}

func TestReplacementIncludesInternalSubflowEntry(t *testing.T) {
	c, _ := filterFixture(t)
	if _, e := PrepareCampaignFilter(context.Background(), c, false, nil); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(FilterOverlay(c), "Event", "SyntheticMain.sbeventpack"))
	if e != nil {
		t.Fatal(e)
	}
	raw, _, e := f.Unpack(b)
	if e != nil {
		t.Fatal(e)
	}
	pack, e := f.ReadSARC(raw)
	if e != nil {
		t.Fatal(e)
	}
	flow, ok := pack.Get("EventFlow/SyntheticMain.bfevfl")
	if !ok {
		t.Fatal("missing flow")
	}
	_, entries, e := f.FlowEntries(flow)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, name := range entries {
		if name == "SharedHelper" {
			found = true
		}
	}
	if !found {
		t.Fatal("lost helper used by another flow")
	}
}
