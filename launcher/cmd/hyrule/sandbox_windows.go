//go:build windows

package main

import (
	"context"
	"fmt"
	core "hyrulelauncher/internal/launcher"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

func (a *app) buildSandboxControls() {
	for _, v := range []struct {
		id    int
		label string
	}{{idSandbox, "Jogar sandbox"}, {idNewSandbox, "Novo + backup"}, {idSeed, "Escolher save base"}, {idAutoSeed, "Usar campanha atual"}, {idApply, "Aplicar"}, {idTeleport, "Teleportar"}, {idSandboxSaves, "Arquivos do sandbox"}, {idRebuildFilter, "Atualizar filtro"}} {
		a.control(v.id, "BUTTON", v.label, 0x000b)
	}
	a.control(idPlace, "COMBOBOX", "", 0x0003|0x00200000)
	for _, p := range core.SandboxPlaces {
		sendMessage.Call(a.controls[idPlace], 0x143, 0, uintptr(unsafe.Pointer(wide(p.Name))))
	}
	a.control(idAction, "COMBOBOX", "", 0x0003|0x00200000)
	for _, v := range core.SandboxActions {
		sendMessage.Call(a.controls[idAction], 0x143, 0, uintptr(unsafe.Pointer(wide(v.Name))))
	}
	sendMessage.Call(a.controls[idAction], 0x14e, 0, 0)
	a.control(idFilter, "COMBOBOX", "", 0x0003|0x00200000)
	for _, label := range []string{"Campanha bloqueada · secundárias e santuários", "Compatibilidade · manter campanha original"} {
		sendMessage.Call(a.controls[idFilter], 0x143, 0, uintptr(unsafe.Pointer(wide(label))))
	}
}
func (a *app) fillSandboxControls() {
	index := 0
	for i, p := range core.SandboxPlaces {
		if p.ID == a.config.SandboxPlace {
			index = i
		}
	}
	sendMessage.Call(a.controls[idPlace], 0x14e, uintptr(index), 0)
	var mode uintptr
	if a.config.KeepCampaign {
		mode = 1
	}
	sendMessage.Call(a.controls[idFilter], 0x14e, mode, 0)
}
func (a *app) keepCampaign() bool {
	i, _, _ := sendMessage.Call(a.controls[idFilter], 0x147, 0, 0)
	return i == 1
}
func (a *app) selectedPlace() string {
	i, _, _ := sendMessage.Call(a.controls[idPlace], 0x147, 0, 0)
	if i < uintptr(len(core.SandboxPlaces)) {
		return core.SandboxPlaces[i].ID
	}
	return "hateno"
}
func (a *app) layoutSandbox(left, right int32, place func(int, int32, int32, int32, int32)) {
	p := a.px
	width := right - left
	place(idPlace, left+p(24), p(828), width-p(210), p(230))
	place(idTeleport, right-p(166), p(828), p(142), p(36))
	third := (width - p(68)) / 3
	place(idSandbox, left+p(24), p(884), third, p(42))
	place(idNewSandbox, left+p(34)+third, p(884), third, p(42))
	place(idSandboxSaves, left+p(44)+2*third, p(884), third, p(42))
	half := (width - p(58)) / 2
	place(idSeed, left+p(24), p(944), half, p(34))
	place(idAutoSeed, left+p(34)+half, p(944), half, p(34))
	place(idAction, left+p(24), p(1018), width-p(196), p(300))
	place(idApply, right-p(152), p(1018), p(128), p(36))
	place(idFilter, left+p(24), p(1090), width-p(196), p(160))
	place(idRebuildFilter, right-p(152), p(1090), p(128), p(36))
}
func (a *app) refreshSandbox(busy bool) {
	enable := func(id int, on bool) {
		var v uintptr
		if on {
			v = 1
		}
		enableWindow.Call(a.controls[id], v)
	}
	supported := a.config.Backend == core.Savage && core.IsFile(a.config.Executable)
	for _, id := range []int{idSandbox, idNewSandbox, idSeed, idAutoSeed} {
		enable(id, supported && !busy)
	}
	enable(idFilter, supported && !busy)
	enable(idRebuildFilter, supported && !busy)
	enable(idPlace, supported && !a.preparing && !a.installing && !a.controlBusy && (!a.session.Busy() || a.sandboxActive))
	enable(idSandboxSaves, a.config.DataDir != "")
	live := a.sandboxActive && a.session.Busy() && a.controlPort > 0 && !a.controlBusy
	for _, id := range []int{idAction, idApply, idTeleport} {
		enable(id, live)
	}
}
func (a *app) paintSandbox(dc uintptr, left, right int32) {
	p := a.px
	a.box(dc, rect{left, p(704), right, p(1184)}, colorCard, colorLine, 16)
	a.label(dc, "SANDBOX · EXPERIMENTAL", "label", colorGold, rect{left + p(24), p(724), right - p(24), p(748)}, 0)
	a.label(dc, "Save próprio, sem liberar torres ou runas automaticamente.\nApós iniciar, escolha Continuar. Teleporte parado, fora de cenas e carregamentos.", "small", colorMuted, rect{left + p(24), p(754), right - p(24), p(799)}, 0x10)
	a.label(dc, "Região inicial (novo sandbox) / destino do teleporte", "label", colorMuted, rect{left + p(24), p(802), right - p(24), p(826)}, 0)
	base := "Save base: campanha ativa · slot normal mais recente"
	if a.config.SandboxSeed != "" {
		base = "Save base: " + filepath.Base(filepath.Dir(a.config.SandboxSeed)) + "/game_data.sav"
	}
	a.label(dc, base, "small", colorMuted, rect{left + p(24), p(928), right - p(24), p(944)}, 0x8000)
	a.label(dc, "Ferramentas · use com o mundo carregado e o jogo renderizando", "label", colorMuted, rect{left + p(24), p(988), right - p(24), p(1014)}, 0)
	a.label(dc, "Perfil de missões · experimental", "label", colorMuted, rect{left + p(24), p(1064), right - p(24), p(1088)}, 0)
	a.label(dc, "O filtro é preparado antes de jogar, sem recriar seu save.\nPolítica e relatório ficam na pasta de dados da recomp.", "small", colorMuted, rect{left + p(24), p(1136), right - p(24), p(1178)}, 0x10)
}
func (a *app) chooseSeed() {
	if a.session.Busy() || a.preparing || a.installing {
		return
	}
	buffer := make([]uint16, 32768)
	filter := utf16.Encode([]rune("Save BOTW Switch (game_data.sav)\x00game_data.sav\x00\x00"))
	o := openFileName{Size: uint32(unsafe.Sizeof(openFileName{})), Owner: a.window, Filter: &filter[0], File: &buffer[0], MaxFile: uint32(len(buffer)), Title: wide("Escolha um save Switch já importado na recomp"), Flags: 0x1000 | 0x800 | 0x80000 | 0x8}
	r, _, _ := openFile.Call(uintptr(unsafe.Pointer(&o)))
	if r == 0 {
		return
	}
	a.config.SandboxSeed = syscall.UTF16ToString(buffer)
	if a.save() {
		a.setStatus("Save base selecionado. Será copiado ao criar um novo sandbox; o original será preservado.", false)
	}
	a.refresh()
}
func (a *app) startSandbox(rebuild bool) {
	if a.installing || a.preparing || a.session.Busy() || a.mpBusy() {
		return
	}
	if !a.save() {
		return
	}
	c := a.config
	if err := core.Validate(c); err != nil {
		a.setStatus(err.Error(), true)
		return
	}
	if c.Backend != core.Savage {
		a.setStatus("O sandbox desta versão exige Savage Wind para Switch 1.9.0.", true)
		return
	}
	if err := core.ValidateSandboxFlags(c); err != nil {
		a.setStatus(err.Error(), true)
		return
	}
	if core.PreparationNeeded(c) {
		a.setStatus("Prepare e teste o jogo no modo campanha antes de criar o sandbox.", true)
		return
	}
	if len((core.SystemFinder{}).Find(c.Executable, time.Time{})) > 0 {
		a.setStatus("Feche a recomp antes de preparar o sandbox.", true)
		return
	}
	a.preparing = true
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.setStatus("Preparando save independente do sandbox…", false)
	a.refresh()
	go func() {
		defer cancel()
		m, err := core.PrepareSandbox(c, rebuild)
		if err != nil {
			a.post(uiEvent{kind: "sandbox-failed", message: err.Error()})
			return
		}
		core.AppendLog(core.LogPath(a.root), fmt.Sprintf("Sandbox: %s; origem SHA256 %s; %d flags preparadas; opcionais ausentes %v; backup %s", core.SandboxDir(c), m.SourceSHA256, len(m.Patched), m.MissingOptional, m.Backup))
		if !c.KeepCampaign {
			r, err := core.PrepareCampaignFilter(ctx, c, false, func(p core.Progress) { a.post(uiEvent{kind: "progress", progress: p}) })
			if err != nil {
				a.post(uiEvent{kind: "sandbox-failed", message: "Filtro não aplicado: " + err.Error()})
				return
			}
			core.AppendLog(core.LogPath(a.root), filterSummary(r))
		}
		port, err := core.LocalControlPort()
		if err != nil {
			a.post(uiEvent{kind: "sandbox-failed", message: err.Error()})
			return
		}
		spec, err := core.BuildSandboxLaunch(c, port, os.Environ())
		if err != nil {
			a.post(uiEvent{kind: "sandbox-failed", message: err.Error()})
			return
		}
		a.post(uiEvent{kind: "sandbox-ready", spec: spec, port: port})
	}()
}
func filterSummary(r core.FilterReport) string {
	return fmt.Sprintf("Filtro preparado: %d principais removidas; %d secundárias e %d missões de santuário mantidas; %d fluxos bloqueados; %d gatilhos de mapa removidos.", len(r.RemovedMain), len(r.KeptSide), len(r.KeptShrine), len(r.BlockedFlows), r.RemovedMapTriggers)
}
func (a *app) rebuildFilter() {
	if a.installing || a.preparing || a.session.Busy() || a.mpBusy() {
		return
	}
	if !a.save() {
		return
	}
	c := a.config
	if core.PreparationNeeded(c) {
		a.setStatus("Prepare o jogo antes de gerar o filtro.", true)
		return
	}
	if len((core.SystemFinder{}).Find(c.Executable, time.Time{})) > 0 {
		a.setStatus("Feche a recomp antes de gerar o filtro.", true)
		return
	}
	a.preparing = true
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.setStatus("Atualizando o filtro sem alterar o save…", false)
	a.refresh()
	go func() {
		defer cancel()
		r, err := core.PrepareCampaignFilter(ctx, c, true, func(p core.Progress) { a.post(uiEvent{kind: "progress", progress: p}) })
		if err != nil {
			a.post(uiEvent{kind: "sandbox-failed", message: err.Error()})
			return
		}
		a.post(uiEvent{kind: "filter-ready", message: filterSummary(r)})
	}()
}
func (a *app) launchSandbox(spec core.LaunchSpec, port int) {
	a.sandboxGeneration++
	a.sandboxActive = true
	a.controlPort = port
	err := a.session.Start(spec, filepath.Join(a.root, "logs", "sandbox-processo.log"), func(e core.Event) { a.post(uiEvent{kind: e.Kind, message: e.Message}) })
	if err != nil {
		a.sandboxActive = false
		a.controlPort = 0
		a.setStatus(err.Error(), true)
		return
	}
	a.setStatus("Sandbox iniciado. Escolha Continuar na recomp; seu save de campanha está separado.", false)
	core.AppendLog(core.LogPath(a.root), fmt.Sprintf("Sandbox iniciado: %s; controle local %d", spec.Executable, port))
}
func (a *app) applySandbox(teleport bool) {
	if !a.sandboxActive || !a.session.Busy() || a.controlPort == 0 || a.controlBusy {
		return
	}
	index, _, _ := sendMessage.Call(a.controls[idAction], 0x147, 0, 0)
	place := a.selectedPlace()
	port := a.controlPort
	generation := a.sandboxGeneration
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	a.controlCancel = cancel
	a.controlBusy = true
	a.setStatus("Enviando solicitação ao sandbox…", false)
	a.refresh()
	go func() {
		defer cancel()
		client := core.ControlClient{Port: port}
		var msg string
		var err error
		if teleport {
			msg, err = client.Teleport(ctx, place, func(s string) { a.post(uiEvent{kind: "control-progress", message: s, generation: generation}) })
		} else {
			msg, err = client.Apply(ctx, int(index))
		}
		if err != nil {
			a.post(uiEvent{kind: "control-failed", message: err.Error(), generation: generation})
			return
		}
		a.post(uiEvent{kind: "control-done", message: msg, generation: generation})
	}()
}
