package launcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"hyrulelauncher/internal/gamefmt"
)

const filterSchema = 1
const rstbPath = "System/Resource/ResourceSizeTable.product.srsizetable"

type CampaignPolicy struct {
	Schema               int
	KeepShrineQuests     bool
	BlockStoryCinematics bool
	AllowFlows           []string
	BlockFlows           []string
}

func DefaultCampaignPolicy() CampaignPolicy {
	return CampaignPolicy{Schema: 1, KeepShrineQuests: true, BlockStoryCinematics: true, AllowFlows: []string{}, BlockFlows: []string{}}
}
func FilterDir(c Config) string        { return filepath.Join(c.DataDir, "hyrule-sandbox-filter") }
func FilterOverlay(c Config) string    { return filepath.Join(FilterDir(c), "overlay") }
func FilterPolicyPath(c Config) string { return filepath.Join(c.DataDir, "hyrule-sandbox-policy.json") }
func validFlow(s string) bool {
	return s != "" && len(s) <= 255 && s != "." && s != ".." && !strings.ContainsAny(s, "/\\<>\x00\r\n")
}
func ReadCampaignPolicy(c Config) (CampaignPolicy, []byte, error) {
	p := DefaultCampaignPolicy()
	b, e := readRegular(FilterPolicyPath(c), 128<<10)
	if os.IsNotExist(e) {
		b, _ = json.MarshalIndent(p, "", "  ")
		b = append(b, '\n')
		if e = os.WriteFile(FilterPolicyPath(c), b, 0600); e != nil {
			return p, nil, e
		}
	} else if e != nil {
		return p, nil, e
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&p); e != nil {
		return p, nil, fmt.Errorf("política do sandbox inválida: %w", e)
	}
	var trailing any
	if dec.Decode(&trailing) != io.EOF {
		return p, nil, fmt.Errorf("política do sandbox contém dados depois do JSON")
	}
	if p.Schema != 1 {
		return p, nil, fmt.Errorf("versão da política de campanha inválida")
	}
	for _, v := range append(append([]string(nil), p.AllowFlows...), p.BlockFlows...) {
		if !validFlow(v) {
			return p, nil, fmt.Errorf("nome de fluxo inválido na política")
		}
	}
	return p, b, nil
}

type filterSource struct{ Root, Prefix, ID string }
type filterInput struct {
	Source   int
	Relative string
	Size     int64
	Modified int64
	SHA256   string `json:",omitempty"`
}
type FilterOutput struct {
	Path     string
	Size     int64
	SHA256   string
	Modified int64
}
type FilterReport struct {
	Schema             int
	Launcher           string
	Created            time.Time
	Signature          string
	RemovedMain        []string
	RemovedShrine      []string
	KeptSide           []string
	KeptShrine         []string
	BlockedFlows       []string
	EventEntries       int
	RemovedMapTriggers int
	ChangedPackages    int
	Outputs            []FilterOutput
	Sources            []filterInput
	Policy             CampaignPolicy
	VerifiedInGame     bool
}
type filterCatalog struct {
	main, side, shrines map[string]bool
	entries             map[string]map[string]bool
}

func newCatalog() *filterCatalog {
	return &filterCatalog{main: map[string]bool{}, side: map[string]bool{}, shrines: map[string]bool{}, entries: map[string]map[string]bool{}}
}
func eventKey(key string) (string, string, error) {
	i := strings.IndexByte(key, '<')
	if i <= 0 || !strings.HasSuffix(key, ">") {
		return "", "", fmt.Errorf("EventInfo: chave inválida")
	}
	flow, entry := key[:i], key[i+1:len(key)-1]
	if entry == "" {
		entry = flow
	}
	if !validFlow(flow) || !validFlow(entry) {
		return "", "", fmt.Errorf("EventInfo: nome inválido")
	}
	return flow, entry, nil
}
func isQuestFile(name string) bool {
	return name == "Quest/QuestProduct.sbquestpack" || name == "Quest/QuestProduct.bquestpack"
}
func isEventInfo(name string) bool {
	return name == "Event/EventInfo.product.sbyml" || name == "Event/EventInfo.product.byml"
}
func readDoc(blob []byte) (gamefmt.Document, bool, error) {
	raw, z, e := gamefmt.Unpack(blob)
	if e != nil {
		return gamefmt.Document{}, z, e
	}
	d, e := gamefmt.ReadBYML(raw)
	return d, z, e
}
func (cat *filterCatalog) collect(name string, blob []byte) error {
	if !isQuestFile(name) && !isEventInfo(name) {
		return nil
	}
	d, _, e := readDoc(blob)
	if e != nil {
		return e
	}
	if isQuestFile(name) {
		if d.Root.Kind != gamefmt.Array {
			return fmt.Errorf("QuestProduct: raiz inesperada")
		}
		for _, q := range d.Root.List {
			n, t := q.Get("Name").Str(), q.Get("Type").Str()
			if !validFlow(n) {
				return fmt.Errorf("QuestProduct: nome inválido")
			}
			switch t {
			case "Main":
				cat.main[n] = true
			case "Mini":
				cat.side[n] = true
			case "Sub":
				cat.shrines[n] = true
			default:
				return fmt.Errorf("QuestProduct: categoria desconhecida %q", t)
			}
		}
		return nil
	}
	if d.Root.Kind != gamefmt.Map {
		return fmt.Errorf("EventInfo: raiz inesperada")
	}
	for key := range d.Root.Fields {
		flow, entry, e := eventKey(key)
		if e != nil {
			return e
		}
		if cat.entries[flow] == nil {
			cat.entries[flow] = map[string]bool{}
		}
		cat.entries[flow][entry] = true
	}
	return nil
}
func (cat *filterCatalog) blocked(flow string, p CampaignPolicy) bool {
	for _, v := range p.AllowFlows {
		if v == flow {
			return false
		}
	}
	for _, v := range p.BlockFlows {
		if v == flow {
			return true
		}
	}
	if cat.main[flow] || (!p.KeepShrineQuests && cat.shrines[flow]) {
		return true
	}
	if !p.BlockStoryCinematics || len(flow) < 9 || !strings.HasPrefix(flow, "Demo") || flow[7] != '_' {
		return false
	}
	number, e := strconv.Atoi(flow[4:7])
	if e != nil {
		return false
	}
	// Hestu's pouch expansion and the other documented non-story exception are
	// retained. System 000-099 and side-quest 300-599 scenes remain untouched.
	if number == 707 || number == 719 {
		return false
	}
	return number >= 100 && number <= 199 || number >= 600 && number <= 699 || number >= 700 && number <= 799 || number == 27 || number == 50 || number == 51
}
func setNames(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for s := range m {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
func (cat *filterCatalog) patch(name string, blob []byte, p CampaignPolicy, r *FilterReport) ([]byte, int, bool, error) {
	if !isQuestFile(name) && !isEventInfo(name) && !strings.HasSuffix(name, ".smubin") && !strings.HasSuffix(name, ".mubin") {
		return nil, 0, false, nil
	}
	d, z, e := readDoc(blob)
	if e != nil {
		return nil, 0, false, e
	}
	changed := false
	switch {
	case isQuestFile(name):
		kept := make([]*gamefmt.Node, 0, len(d.Root.List))
		for _, q := range d.Root.List {
			t := q.Get("Type").Str()
			if t == "Main" || t == "Sub" && !p.KeepShrineQuests {
				changed = true
				continue
			}
			kept = append(kept, q)
		}
		if changed {
			d.Root = gamefmt.Arr(kept...)
		}
	case isEventInfo(name):
		root := d.Root.CloneMap()
		for key := range root.Fields {
			flow, _, e := eventKey(key)
			if e != nil {
				return nil, 0, false, e
			}
			if !cat.blocked(flow, p) {
				continue
			}
			root.Fields[key] = gamefmt.Dict(map[string]*gamefmt.Node{"is_timeline": gamefmt.Boolean(false), "mode": gamefmt.Text("FullPackage"), "demo_event": gamefmt.Boolean(false), "auto_save": gamefmt.Boolean(false), "is_startable_air": gamefmt.Boolean(true), "is_pause_non_demo_member": gamefmt.Boolean(false), "child": gamefmt.Arr(), "subfile": gamefmt.Arr()})
			changed = true
			r.EventEntries++
		}
		if changed {
			d.Root = root
		}
	default:
		// Removing the actual trigger actor also removes its placement links. Other
		// actors, including combat, shops and secondary quest objects, keep their data.
		objects := d.Root.Get("Objs")
		if objects == nil || objects.Kind != gamefmt.Array {
			return nil, 0, false, nil
		}
		kept := make([]*gamefmt.Node, 0, len(objects.List))
		removed := map[uint64]bool{}
		for _, actor := range objects.List {
			params := actor.Get("!Parameters")
			if params == nil {
				params = actor.Get("Parameters")
			}
			flow := params.Get("EventFlowName").Str()
			unit := actor.Get("UnitConfigName").Str()
			trigger := strings.HasPrefix(unit, "EventTag") || unit == "Tag_EventFlow"
			if trigger && flow != "" && cat.blocked(flow, p) {
				changed = true
				r.RemovedMapTriggers++
				if h := actor.Get("HashId"); h != nil {
					removed[h.Bits] = true
				}
				continue
			}
			kept = append(kept, actor)
		}
		if changed {
			for i, actor := range kept {
				links := actor.Get("LinksToObj")
				if links == nil || links.Kind != gamefmt.Array {
					continue
				}
				var filtered []*gamefmt.Node
				for _, link := range links.List {
					dest := link.Get("DestUnitHashId")
					if dest == nil || !removed[dest.Bits] {
						filtered = append(filtered, link)
					}
				}
				if len(filtered) != len(links.List) {
					copy := actor.CloneMap()
					copy.Fields["LinksToObj"] = gamefmt.Arr(filtered...)
					kept[i] = copy
				}
			}
			root := d.Root.CloneMap()
			root.Fields["Objs"] = gamefmt.Arr(kept...)
			d.Root = root
		}
	}
	if !changed {
		return nil, 0, false, nil
	}
	raw, e := gamefmt.WriteBYML(d)
	if e != nil {
		return nil, 0, false, e
	}
	return gamefmt.Repack(raw, z), len(raw), true, nil
}
func filterCandidates(c Config) ([]filterSource, []filterInput, error) {
	return filterCandidatesContext(context.Background(), c)
}
func filterCandidatesContext(ctx context.Context, c Config) ([]filterSource, []filterInput, error) {
	sources := []filterSource{{Root: filepath.Join(c.DataDir, "extracted", "romfs"), ID: "romfs"}}
	if !IsFile(filepath.Join(sources[0].Root, "Pack", "Bootup.pack")) || !IsFile(filepath.Join(sources[0].Root, "Pack", "TitleBG.pack")) {
		return nil, nil, fmt.Errorf("não encontrei Pack/Bootup.pack e Pack/TitleBG.pack em extracted/romfs; o filtro precisa dos arquivos extraídos usados pelo Savage Wind")
	}
	if !IsDir(filepath.Join(sources[0].Root, "Event")) {
		return nil, nil, fmt.Errorf("a pasta Event do RomFS não foi encontrada; o filtro precisa da instalação extraída completa")
	}
	for i := 1; i <= 8; i++ {
		id := "aoc" + strconv.Itoa(i)
		root := filepath.Join(c.DataDir, "extracted", id)
		if IsDir(root) {
			sources = append(sources, filterSource{Root: root, ID: id, Prefix: "aoc/0010"})
		}
	}
	var inputs []filterInput
	for index, src := range sources {
		e := filepath.WalkDir(src.Root, func(full string, de fs.DirEntry, err error) error {
			if e := ctx.Err(); e != nil {
				return e
			}
			if err != nil {
				return err
			}
			if de.IsDir() {
				return nil
			}
			rel, e := filepath.Rel(src.Root, full)
			if e != nil {
				return e
			}
			rel = filepath.ToSlash(rel)
			if !candidate(rel) {
				return nil
			}
			st, e := de.Info()
			if e != nil {
				return e
			}
			if !st.Mode().IsRegular() || st.Size() > gamefmt.MaxData {
				return fmt.Errorf("arquivo inválido ou muito grande: %s", rel)
			}
			input := filterInput{Source: index, Relative: rel, Size: st.Size(), Modified: st.ModTime().UnixNano()}
			if rel == rstbPath || rel == "Pack/Bootup.pack" || rel == "Pack/TitleBG.pack" || path.Base(rel) == "AocMainField.pack" {
				f, e := os.Open(full)
				if e != nil {
					return e
				}
				h := sha256.New()
				_, e = io.Copy(h, cancelReader{ctx: ctx, reader: f})
				closeErr := f.Close()
				if e != nil {
					return e
				}
				if closeErr != nil {
					return closeErr
				}
				input.SHA256 = hex.EncodeToString(h.Sum(nil))
			}
			inputs = append(inputs, input)
			return nil
		})
		if e != nil {
			return nil, nil, e
		}
	}
	sort.Slice(inputs, func(i, j int) bool {
		if inputs[i].Source == inputs[j].Source {
			return inputs[i].Relative < inputs[j].Relative
		}
		return inputs[i].Source < inputs[j].Source
	})
	return sources, inputs, nil
}

type cancelReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r cancelReader) Read(b []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.reader.Read(b)
}
func isPackage(name string) bool {
	return strings.HasSuffix(name, ".pack") || strings.HasSuffix(name, ".sbeventpack") || strings.HasSuffix(name, ".sbactorpack")
}
func candidate(name string) bool {
	return isPackage(name) || isQuestFile(name) || isEventInfo(name) || strings.HasSuffix(name, ".smubin") || strings.HasSuffix(name, ".mubin") || strings.HasSuffix(name, ".bfevfl") || strings.HasSuffix(name, ".bdemo") || name == rstbPath
}
func signature(sources []filterSource, inputs []filterInput, policy []byte) string {
	b, _ := json.Marshal(struct {
		Version int
		Sources []filterSource
		Inputs  []filterInput
		Policy  string
	}{filterSchema, sources, inputs, string(policy)})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func loadFilter(c Config, sig string) (FilterReport, error) {
	var r FilterReport
	b, e := readRegular(filepath.Join(FilterDir(c), "filter.json"), 8<<20)
	if e != nil {
		return r, e
	}
	if e = json.Unmarshal(b, &r); e != nil {
		return r, e
	}
	if r.Schema != filterSchema || sig != "" && r.Signature != sig {
		return r, fmt.Errorf("filtro precisa ser atualizado")
	}
	if len(r.Outputs) == 0 || len(r.RemovedMain) == 0 {
		return r, fmt.Errorf("filtro incompleto")
	}
	for _, out := range r.Outputs {
		if !safeRelative(out.Path) {
			return r, fmt.Errorf("caminho de filtro inválido")
		}
		st, e := os.Lstat(filepath.Join(FilterOverlay(c), filepath.FromSlash(out.Path)))
		if e != nil || !st.Mode().IsRegular() || st.Size() != out.Size || st.ModTime().UnixNano() != out.Modified {
			return r, fmt.Errorf("arquivo do filtro ausente ou alterado: %s", out.Path)
		}
	}
	return r, nil
}
func CampaignFilterReport(c Config) (FilterReport, error) { return loadFilter(c, "") }
func safeRelative(s string) bool {
	return s != "" && !strings.ContainsAny(s, "\\:\x00") && !strings.HasPrefix(s, "/") && path.Clean(s) == s && s != ".." && !strings.HasPrefix(s, "../")
}
func PrepareCampaignFilter(ctx context.Context, c Config, force bool, notify func(Progress)) (FilterReport, error) {
	var report FilterReport
	if e := ctx.Err(); e != nil {
		return report, e
	}
	if notify != nil {
		notify(Progress{Stage: "Conferindo arquivos e cache do filtro…"})
	}
	if c.Backend != Savage {
		return report, fmt.Errorf("o filtro exige Savage Wind")
	}
	policy, policyBytes, e := ReadCampaignPolicy(c)
	if e != nil {
		return report, e
	}
	sources, inputs, e := filterCandidatesContext(ctx, c)
	if e != nil {
		return report, e
	}
	sig := signature(sources, inputs, policyBytes)
	if !force {
		if old, e := loadFilter(c, sig); e == nil {
			return old, nil
		}
	}
	progress := func(stage string, done int) {
		if notify != nil {
			notify(Progress{Stage: stage, Done: int64(done), Total: int64(len(inputs))})
		}
	}
	catalog := newCatalog()
	for i, in := range inputs {
		if e = ctx.Err(); e != nil {
			return report, e
		}
		if !(strings.HasPrefix(in.Relative, "Pack/") && strings.HasSuffix(in.Relative, ".pack")) && !isQuestFile(in.Relative) && !isEventInfo(in.Relative) {
			continue
		}
		progress("Lendo categorias e eventos", i)
		blob, e := readRegular(filepath.Join(sources[in.Source].Root, filepath.FromSlash(in.Relative)), gamefmt.MaxData)
		if e != nil {
			return report, e
		}
		if isPackage(in.Relative) {
			raw, _, e := gamefmt.Unpack(blob)
			if e != nil {
				return report, e
			}
			pack, e := gamefmt.ReadSARC(raw)
			if e != nil {
				return report, fmt.Errorf("%s: %w", in.Relative, e)
			}
			for _, entry := range pack.Entries {
				if isQuestFile(entry.Name) || isEventInfo(entry.Name) {
					d, _ := pack.Get(entry.Name)
					if e = catalog.collect(entry.Name, d); e != nil {
						return report, fmt.Errorf("%s/%s: %w", in.Relative, entry.Name, e)
					}
				}
			}
		} else if e = catalog.collect(in.Relative, blob); e != nil {
			return report, e
		}
	}
	if len(catalog.main) == 0 || len(catalog.side) == 0 || len(catalog.entries) == 0 {
		return report, fmt.Errorf("catálogo de campanha incompleto; nenhum filtro foi publicado")
	}
	for name := range catalog.main {
		if catalog.side[name] || catalog.shrines[name] {
			return report, fmt.Errorf("categoria de missão conflitante em %s; filtro não publicado", name)
		}
	}
	report = FilterReport{Schema: filterSchema, Launcher: Version, Created: time.Now().UTC(), Signature: sig, RemovedMain: setNames(catalog.main), KeptSide: setNames(catalog.side), Policy: policy, Sources: inputs, VerifiedInGame: false}
	if policy.KeepShrineQuests {
		report.KeptShrine = setNames(catalog.shrines)
	} else {
		report.RemovedShrine = setNames(catalog.shrines)
	}

	// Collect the complete entry table before emitting any replacement. Internal
	// helper entries used by secondary subflows are not all listed in EventInfo.
	collectFlow := func(name string, data []byte) error {
		if !strings.HasPrefix(name, "EventFlow/") || !strings.HasSuffix(name, ".bfevfl") {
			return nil
		}
		id := strings.TrimSuffix(strings.TrimPrefix(name, "EventFlow/"), ".bfevfl")
		if !catalog.blocked(id, policy) {
			return nil
		}
		declared, entries, e := gamefmt.FlowEntries(data)
		if e != nil {
			return fmt.Errorf("%s: %w", name, e)
		}
		if declared != id {
			return fmt.Errorf("nome interno de fluxo diferente do arquivo: %s", name)
		}
		// Empty quest flows are valid. Keep any EventInfo declarations already
		// collected, but do not invent an entry-less replacement for a graph
		// that has no runnable entry in the original either (issue #1).
		if len(entries) == 0 {
			return nil
		}
		if catalog.entries[id] == nil {
			catalog.entries[id] = map[string]bool{}
		}
		for _, entry := range entries {
			if !validFlow(entry) {
				return fmt.Errorf("entrada inválida em %s", name)
			}
			catalog.entries[id][entry] = true
		}
		return nil
	}
	for i, in := range inputs {
		if e = ctx.Err(); e != nil {
			return report, e
		}
		packageNeeded := isPackage(in.Relative)
		if !packageNeeded && !(strings.HasPrefix(in.Relative, "EventFlow/") && strings.HasSuffix(in.Relative, ".bfevfl")) {
			continue
		}
		progress("Conferindo entradas dos eventos", i)
		blob, e := readRegular(filepath.Join(sources[in.Source].Root, filepath.FromSlash(in.Relative)), gamefmt.MaxData)
		if e != nil {
			return report, e
		}
		if packageNeeded {
			raw, _, e := gamefmt.Unpack(blob)
			if e != nil {
				return report, e
			}
			pack, e := gamefmt.ReadSARC(raw)
			if e != nil {
				return report, e
			}
			for _, entry := range pack.Entries {
				b, _ := pack.Get(entry.Name)
				if e = collectFlow(entry.Name, b); e != nil {
					return report, e
				}
			}
		} else if e = collectFlow(in.Relative, blob); e != nil {
			return report, e
		}
	}
	stubs := map[string][]byte{}
	demos := map[string][]byte{}
	for flow, entries := range catalog.entries {
		if !catalog.blocked(flow, policy) {
			continue
		}
		names := setNames(entries)
		stub, e := gamefmt.NoopFlow(flow, names)
		if e != nil {
			return report, e
		}
		stubs[flow] = stub
		report.BlockedFlows = append(report.BlockedFlows, flow)
		for _, entry := range names {
			if strings.HasPrefix(entry, "Demo") {
				demos[entry] = gamefmt.NoopDemo()
			}
		}
	}
	sort.Strings(report.BlockedFlows)
	if len(stubs) == 0 {
		return report, fmt.Errorf("nenhum evento de campanha identificado")
	}
	// One main RSTB is required; DLC images that have their own receive a matching
	// table. An image without one relies on the main game's resource registry.
	registries := map[int]*gamefmt.RSTB{}
	for i, src := range sources {
		b, e := readRegular(filepath.Join(src.Root, filepath.FromSlash(rstbPath)), 16<<20)
		if os.IsNotExist(e) && i > 0 {
			continue
		}
		if e != nil {
			return report, fmt.Errorf("tabela de recursos não encontrada: %w", e)
		}
		raw, _, e := gamefmt.Unpack(b)
		if e != nil {
			return report, e
		}
		registries[i], e = gamefmt.ReadRSTB(raw)
		if e != nil {
			return report, e
		}
	}
	stage, e := os.MkdirTemp(c.DataDir, ".filter-stage-")
	if e != nil {
		return report, e
	}
	defer os.RemoveAll(stage)
	outputRoot := filepath.Join(stage, "overlay")
	if e = os.MkdirAll(outputRoot, 0700); e != nil {
		return report, e
	}
	writes := map[string]FilterOutput{}
	write := func(index int, relative string, blob []byte, rawSize int) error {
		if !safeRelative(relative) {
			return fmt.Errorf("caminho de saída inválido")
		}
		name := path.Join(sources[index].Prefix, relative)
		h := sha256.Sum256(blob)
		hash := hex.EncodeToString(h[:])
		registry := registries[index]
		if registry == nil {
			registry = registries[0]
		}
		if e := registry.Ensure(relative, rawSize); e != nil {
			return e
		}
		if old, ok := writes[name]; ok {
			if old.SHA256 != hash {
				return fmt.Errorf("DLCs possuem versões conflitantes de %s; o filtro anterior foi preservado", name)
			}
			return nil
		}
		full := filepath.Join(outputRoot, filepath.FromSlash(name))
		if e := os.MkdirAll(filepath.Dir(full), 0700); e != nil {
			return e
		}
		if e := os.WriteFile(full, blob, 0600); e != nil {
			return e
		}
		st, e := os.Stat(full)
		if e != nil {
			return e
		}
		writes[name] = FilterOutput{Path: name, Size: st.Size(), Modified: st.ModTime().UnixNano(), SHA256: hash}
		return nil
	}
	// Replacements exist in complete Event packs. That also covers movie-only
	// events and avoids depending on adding new RomFS directory names.
	generatedPacks := map[string]bool{}
	for i, in := range inputs {
		if e = ctx.Err(); e != nil {
			return report, e
		}
		if in.Relative == rstbPath {
			continue
		}
		progress("Gerando filtro de campanha", i)
		blob, e := readRegular(filepath.Join(sources[in.Source].Root, filepath.FromSlash(in.Relative)), gamefmt.MaxData)
		if e != nil {
			return report, e
		}
		if isPackage(in.Relative) {
			raw, z, e := gamefmt.Unpack(blob)
			if e != nil {
				return report, e
			}
			pack, e := gamefmt.ReadSARC(raw)
			if e != nil {
				return report, fmt.Errorf("%s: %w", in.Relative, e)
			}
			changed := false
			for _, entry := range append([]gamefmt.Entry(nil), pack.Entries...) {
				data, _ := pack.Get(entry.Name)
				replacement, rawSize, yes, e := catalog.patch(entry.Name, data, policy, &report)
				if e != nil {
					return report, fmt.Errorf("%s/%s: %w", in.Relative, entry.Name, e)
				}
				if flow, ok := strings.CutPrefix(entry.Name, "EventFlow/"); ok && strings.HasSuffix(flow, ".bfevfl") {
					if stub := stubs[strings.TrimSuffix(flow, ".bfevfl")]; stub != nil {
						replacement, rawSize, yes = stub, len(stub), true
					}
				}
				if demo, ok := strings.CutPrefix(entry.Name, "Demo/"); ok && strings.HasSuffix(demo, ".bdemo") {
					if stub := demos[strings.TrimSuffix(demo, ".bdemo")]; stub != nil {
						replacement, rawSize, yes = stub, len(stub), true
					}
				}
				if !yes {
					continue
				}
				changed = true
				if e = pack.Replace(entry.Name, replacement); e != nil {
					return report, e
				}
				registry := registries[in.Source]
				if registry == nil {
					registry = registries[0]
				}
				if e = registry.Ensure(entry.Name, rawSize); e != nil {
					return report, e
				}
			}
			// A requested story pack can have been only a timeline/movie pack. Rebuild
			// it with an added flow and descriptor while preserving its other files.
			flowName := strings.TrimSuffix(strings.TrimPrefix(in.Relative, "Event/"), ".sbeventpack")
			if strings.HasPrefix(in.Relative, "Event/") && strings.HasSuffix(in.Relative, ".sbeventpack") && stubs[flowName] != nil {
				files := map[string][]byte{}
				for _, entry := range pack.Entries {
					if entry.Name == "" {
						return report, fmt.Errorf("event pack possui arquivo sem nome")
					}
					d, _ := pack.Get(entry.Name)
					files[entry.Name] = append([]byte(nil), d...)
				}
				files["EventFlow/"+flowName+".bfevfl"] = stubs[flowName]
				for entry := range catalog.entries[flowName] {
					if demos[entry] != nil {
						files["Demo/"+entry+".bdemo"] = demos[entry]
					}
				}
				rebuilt, e := gamefmt.WriteSARC(files)
				if e != nil {
					return report, e
				}
				pack.Data = rebuilt
				changed = true
				generatedPacks[strconv.Itoa(in.Source)+":"+flowName] = true
			}
			if changed {
				report.ChangedPackages++
				if e = write(in.Source, in.Relative, gamefmt.Repack(pack.Data, z), len(pack.Data)); e != nil {
					return report, e
				}
			}
		} else {
			replacement, rawSize, changed, e := catalog.patch(in.Relative, blob, policy, &report)
			if e != nil {
				return report, fmt.Errorf("%s: %w", in.Relative, e)
			}
			if flow, ok := strings.CutPrefix(in.Relative, "EventFlow/"); ok && strings.HasSuffix(flow, ".bfevfl") {
				if stub := stubs[strings.TrimSuffix(flow, ".bfevfl")]; stub != nil {
					replacement, rawSize, changed = stub, len(stub), true
				}
			}
			if demo, ok := strings.CutPrefix(in.Relative, "Demo/"); ok && strings.HasSuffix(demo, ".bdemo") {
				if stub := demos[strings.TrimSuffix(demo, ".bdemo")]; stub != nil {
					replacement, rawSize, changed = stub, len(stub), true
				}
			}
			if changed {
				if e = write(in.Source, in.Relative, replacement, rawSize); e != nil {
					return report, e
				}
			}
		}
	}
	for index, src := range sources {
		if index > 0 && !IsDir(filepath.Join(src.Root, "Event")) {
			continue
		}
		for flow, stub := range stubs {
			for _, reg := range registries {
				if e = reg.Ensure("EventFlow/"+flow+".bfevfl", len(stub)); e != nil {
					return report, e
				}
			}
			if generatedPacks[strconv.Itoa(index)+":"+flow] {
				continue
			}
			files := map[string][]byte{"EventFlow/" + flow + ".bfevfl": stub}
			for entry := range catalog.entries[flow] {
				if d := demos[entry]; d != nil {
					files["Demo/"+entry+".bdemo"] = d
					for _, reg := range registries {
						if e = reg.Ensure("Demo/"+entry+".bdemo", len(d)); e != nil {
							return report, e
						}
					}
				}
			}
			pack, e := gamefmt.WriteSARC(files)
			if e != nil {
				return report, e
			}
			if e = write(index, "Event/"+flow+".sbeventpack", gamefmt.PackYaz0(pack), len(pack)); e != nil {
				return report, e
			}
		}
	}
	for index, registry := range registries {
		// Include descriptors present in already rebuilt packs, too.
		for entry, d := range demos {
			if e = registry.Ensure("Demo/"+entry+".bdemo", len(d)); e != nil {
				return report, e
			}
		}
		raw := registry.Bytes()
		if e = registry.Ensure(rstbPath, len(raw)+8); e != nil {
			return report, e
		}
		raw = registry.Bytes()
		if e = write(index, rstbPath, gamefmt.PackYaz0(raw), len(raw)); e != nil {
			return report, e
		}
	}
	if report.EventEntries == 0 {
		return report, fmt.Errorf("nenhum EventInfo foi alterado; filtro não publicado")
	}
	// Source folders must stay stable throughout preparation.
	againSources, again, e := filterCandidatesContext(ctx, c)
	if e != nil {
		return report, e
	}
	if signature(againSources, again, policyBytes) != sig {
		return report, fmt.Errorf("os arquivos do jogo mudaram durante a preparação; feche a recomp e tente novamente")
	}
	for _, out := range writes {
		report.Outputs = append(report.Outputs, out)
	}
	sort.Slice(report.Outputs, func(i, j int) bool { return report.Outputs[i].Path < report.Outputs[j].Path })
	data, e := json.MarshalIndent(report, "", "  ")
	if e != nil {
		return report, e
	}
	if e = os.WriteFile(filepath.Join(stage, "filter.json"), append(data, '\n'), 0600); e != nil {
		return report, e
	}
	target := FilterDir(c)
	backup := target + "-previous"
	if _, e = os.Lstat(target); e == nil {
		if st, e := os.Lstat(target); e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return report, fmt.Errorf("pasta de filtro inválida; conteúdo preservado")
		}
		if e = os.RemoveAll(backup); e != nil {
			return report, e
		}
		if e = os.Rename(target, backup); e != nil {
			return report, e
		}
	} else if !os.IsNotExist(e) {
		return report, e
	}
	if e = os.Rename(stage, target); e != nil {
		if IsDir(backup) {
			if restore := os.Rename(backup, target); restore != nil {
				return report, fmt.Errorf("falha ao publicar; filtro anterior em %s", backup)
			}
		}
		return report, e
	}
	progress("Filtro de campanha pronto", len(inputs))
	return report, nil
}

// The upstream runtime reapplies saved launch flags after process start. A
// conflicting option would silently defeat save isolation or this overlay.
// Preserve the player's graphics/debug options and explain only real conflicts.
func ValidateSandboxFlags(c Config) error {
	b, e := readRegular(filepath.Join(c.DataDir, "launch-flags.txt"), 1<<20)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	reserved := map[string]bool{"SAVAGEWIND_SAVE_DIR": true, "SAVAGEWIND_ROMFS_OVERLAY": true, "SAVAGEWIND_CONTROL": true, "SAVAGEWIND_TEST_FLAGS": true, "SAVAGEWIND_TEST_WARP": true, "SAVAGEWIND_TEST_TIME": true, "SAVAGEWIND_TEST_WEATHER": true}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, _, _ := strings.Cut(line, "=")
		name = strings.ToUpper(strings.TrimSpace(name))
		if reserved[name] {
			return fmt.Errorf("a opção %s em launch-flags.txt conflita com o sandbox; remova essa opção nas opções de lançamento do Savage Wind e tente novamente", name)
		}
	}
	return nil
}
