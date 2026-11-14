//go:build windows

package main

import (
	core "hyrulelauncher/internal/launcher"
	"path/filepath"
	"unsafe"
)

const (
	idHome         = 160
	idSandboxPage  = 161
	idSettingsPage = 162
	idHomeJoin     = 163
)

// Pages keep all native controls alive, so changing sections never loses paths,
// combo selections or a running session. Only the active page enters tab order.
func (a *app) changePage(page int) {
	a.page = page
	a.scroll = 0
	a.layout()
	invalidate.Call(a.window, 0, 0)
}
func (a *app) layoutDashboard() {
	if a.layingOut {
		return
	}
	a.layingOut = true
	defer func() { a.layingOut = false }()
	var r rect
	getClientRect.Call(a.window, uintptr(unsafe.Pointer(&r)))
	if r.Right <= 0 || r.Bottom <= 0 {
		return
	}
	p := a.px
	left, right := p(248), r.Right-p(32)
	width := right - left
	content := 680
	if a.page == idSandboxPage {
		content = 810
	}
	if a.page == idSettingsPage {
		content = 725
	}
	viewport := r.Bottom - p(92)
	maximum := p(content) - viewport
	if maximum < 0 {
		maximum = 0
	}
	if a.scroll < 0 {
		a.scroll = 0
	}
	if a.scroll > maximum {
		a.scroll = maximum
	}
	si := scrollInfo{Size: uint32(unsafe.Sizeof(scrollInfo{})), Mask: 7, Max: p(content+92) - 1, Page: uint32(r.Bottom), Pos: a.scroll}
	setScrollInfo.Call(a.window, 1, uintptr(unsafe.Pointer(&si)), 1)
	visible := map[int]bool{}
	place := func(id int, x, y, w, h int32) {
		heightVisible := h
		if id == idBackend || id == idPlace || id == idAction || id == idFilter {
			heightVisible = p(36)
		}
		visible[id] = y-a.scroll >= 0 && y-a.scroll+heightVisible <= r.Bottom-p(86)
		moveWindow.Call(a.controls[id], uintptr(x), uintptr(y-a.scroll), uintptr(w), uintptr(h), 0)
	}
	fixed := func(id, y, h int) {
		visible[id] = true
		moveWindow.Call(a.controls[id], uintptr(p(24)), uintptr(p(y)), uintptr(p(184)), uintptr(p(h)), 0)
	}
	fixed(idHome, 162, 44)
	fixed(idMultiplayer, 214, 44)
	fixed(idSandboxPage, 266, 44)
	fixed(idSettingsPage, 318, 44)
	fixed(idLogs, 424, 38)
	fixed(141, 470, 38)
	switch a.page {
	case idSettingsPage:
		place(idBackend, left+p(24), p(190), width-p(48), p(180))
		for _, v := range []struct{ edit, button, y int }{{idExe, idBrowseExe, 276}, {idData, idBrowseData, 358}, {idGame, idBrowseGame, 440}} {
			if v.edit == idGame && a.config.Backend != core.Native {
				continue
			}
			place(v.edit, left+p(24), p(v.y), width-p(154), p(36))
			place(v.button, right-p(116), p(v.y), p(92), p(36))
		}
		place(idSave, left, p(526), p(174), p(42))
		place(idSetup, left+p(188), p(526), p(174), p(42))
		place(idDownload, left, p(606), width, p(42))
		place(idSaves, left, p(664), p(174), p(36))
	case idSandboxPage:
		place(idSandbox, left+p(24), p(244), p(186), p(46))
		place(idNewSandbox, left+p(224), p(244), p(170), p(46))
		place(idPlace, left+p(24), p(190), width-p(204), p(230))
		place(idTeleport, right-p(164), p(190), p(140), p(36))
		half := (width - p(62)) / 2
		place(idSeed, left+p(24), p(366), half, p(36))
		place(idAutoSeed, left+p(38)+half, p(366), half, p(36))
		place(idFilter, left+p(24), p(448), width-p(208), p(180))
		place(idRebuildFilter, right-p(168), p(448), p(144), p(36))
		place(idAction, left+p(24), p(594), width-p(194), p(280))
		place(idApply, right-p(154), p(594), p(130), p(36))
		place(idSandboxSaves, left+p(24), p(692), p(208), p(36))
	default:
		place(idHomeJoin, left+p(28), p(278), p(224), p(48))
		half := (width - p(18)) / 2
		place(idPlay, left+p(24), p(496), half-p(48), p(44))
		place(idSandbox, left+half+p(42), p(496), half-p(48), p(44))
		place(idSaves, left, p(600), p(172), p(36))
	}
	for id, h := range a.controls {
		v := uintptr(0)
		if visible[id] {
			v = 5
		}
		showWindow.Call(h, v)
	}
	user32.NewProc("RedrawWindow").Call(a.window, 0, 0, 0x1|0x80|0x20)
}
func (a *app) paintDashboard(dc uintptr) {
	var r rect
	getClientRect.Call(a.window, uintptr(unsafe.Pointer(&r)))
	fillRect.Call(dc, uintptr(unsafe.Pointer(&r)), a.bgBrush)
	if len(a.fonts) == 0 {
		return
	}
	p := a.px
	left, right := p(248), r.Right-p(32)
	w := right - left
	a.box(dc, rect{0, 0, p(224), r.Bottom}, rgb(16, 22, 28), rgb(16, 22, 28), 0)
	a.label(dc, "H / M", "brand", colorGold, rect{p(28), p(30), p(206), p(66)}, 0)
	a.label(dc, "HYRULEM", "label", colorText, rect{p(29), p(76), p(206), p(98)}, 0)
	a.label(dc, "UM MUNDO. NOVAS HISTÓRIAS.", "small", colorMuted, rect{p(29), p(106), p(206), p(140)}, 0x10)
	a.label(dc, "FERRAMENTAS", "label", colorMuted, rect{p(29), p(393), p(206), p(415)}, 0)
	a.label(dc, "PRÉVIA EXPERIMENTAL\n"+core.Version, "small", colorMuted, rect{p(29), r.Bottom - p(76), p(206), r.Bottom - p(24)}, 0x10)
	setViewport.Call(dc, 0, uintptr(-a.scroll), 0)
	title, sub := "Vamos a Hyrule?", "Entre com seus amigos ou continue sua própria aventura."
	if a.page == idSettingsPage {
		title = "Sua instalação"
		sub = "Configure uma vez. Os caminhos ficam salvos para as próximas sessões."
	}
	if a.page == idSandboxPage {
		title = "Seu mundo livre"
		sub = "Escolha o ponto de partida e prepare as regras da sua aventura."
	}
	a.label(dc, title, "title", colorText, rect{left, p(32), right, p(76)}, 0)
	a.label(dc, sub, "body", colorMuted, rect{left, p(86), right, p(124)}, 0x10)
	switch a.page {
	case idSettingsPage:
		a.box(dc, rect{left, p(144), right, p(498)}, colorCard, colorLine, 14)
		for _, v := range []struct {
			s string
			y int
		}{{"PLATAFORMA", 164}, {"EXECUTÁVEL DO JOGO", 248}, {"PASTA DE DADOS", 330}} {
			a.label(dc, v.s, "label", colorMuted, rect{left + p(24), p(v.y), right - p(24), p(v.y + 22)}, 0)
		}
		if a.config.Backend == core.Native {
			a.label(dc, "PASTA DO JOGO · WII U", "label", colorMuted, rect{left + p(24), p(412), right - p(24), p(436)}, 0)
		} else {
			a.label(dc, "BOTW Switch 1.9.0 · seus arquivos são preparados localmente.\nUse Preparar jogo se esta for sua primeira instalação.", "body", colorMuted, rect{left + p(24), p(428), right - p(24), p(480)}, 0x10)
		}
		a.label(dc, "Precisa instalar o Savage Wind?", "label", colorMuted, rect{left, p(578), right, p(600)}, 0)
	case idSandboxPage:
		a.box(dc, rect{left, p(144), right, p(318)}, colorCard, colorLine, 14)
		a.label(dc, "PONTO DE PARTIDA", "label", colorGold, rect{left + p(24), p(164), right - p(24), p(188)}, 0)
		a.box(dc, rect{left, p(338), right, p(516)}, colorCard, colorLine, 14)
		a.label(dc, "SAVE E MISSÕES", "label", colorGold, rect{left + p(24), p(344), right - p(24), p(364)}, 0)
		a.label(dc, "Perfil de missões", "small", colorMuted, rect{left + p(24), p(418), right - p(24), p(442)}, 0)
		a.box(dc, rect{left, p(542), right, p(752)}, colorCard, colorLine, 14)
		a.label(dc, "FERRAMENTAS EM JOGO", "label", colorGold, rect{left + p(24), p(560), right - p(24), p(584)}, 0)
		a.label(dc, "As ações ficam disponíveis com o sandbox aberto.\nUse fora de cenas e carregamentos.", "body", colorMuted, rect{left + p(24), p(642), right - p(24), p(686)}, 0x10)
	default:
		a.box(dc, rect{left, p(148), right, p(358)}, rgb(24, 46, 43), rgb(46, 75, 66), 16)
		a.label(dc, "MULTIPLAYER", "label", colorGold, rect{left + p(28), p(170), right - p(28), p(194)}, 0)
		a.label(dc, "A próxima história é de vocês.", "hero", colorText, rect{left + p(28), p(204), right - p(28), p(244)}, 0)
		a.label(dc, "Conecte por IP ou hospede uma sessão para dois jogadores.", "body", colorMuted, rect{left + p(28), p(246), right - p(28), p(274)}, 0)
		half := (w - p(18)) / 2
		for i := 0; i < 2; i++ {
			x := left + int32(i)*(half+p(18))
			a.box(dc, rect{x, p(382), x + half, p(566)}, colorCard, colorLine, 14)
		}
		a.label(dc, "CAMPANHA", "label", colorMuted, rect{left + p(24), p(402), left + half - p(24), p(426)}, 0)
		a.label(dc, "Sua aventura offline", "body", colorText, rect{left + p(24), p(440), left + half - p(24), p(468)}, 0)
		x := left + half + p(42)
		a.label(dc, "SANDBOX", "label", colorMuted, rect{x, p(402), right - p(24), p(426)}, 0)
		a.label(dc, "Explore no seu ritmo", "body", colorText, rect{x, p(440), right - p(24), p(468)}, 0)
		ready := "Configure sua instalação para começar."
		if core.IsFile(a.config.Executable) {
			ready = "Instalação: " + filepath.Base(a.config.Executable)
			if core.PreparationNeeded(a.config) {
				ready += " · preparação necessária"
			}
		}
		a.label(dc, ready, "small", colorMuted, rect{left + p(190), p(602), right, p(638)}, 0x10)
	}
	setViewport.Call(dc, 0, 0, 0)
	statusColor := colorGreen
	if a.statusError {
		statusColor = colorRed
	}
	a.box(dc, rect{left, r.Bottom - p(76), right, r.Bottom - p(16)}, colorBG, colorLine, 10)
	a.label(dc, a.status, "small", statusColor, rect{left + p(16), r.Bottom - p(66), right - p(16), r.Bottom - p(24)}, 0x10)
}
