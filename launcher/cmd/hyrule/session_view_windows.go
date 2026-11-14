//go:build windows

package main

import (
	"fmt"
	core "hyrulelauncher/internal/launcher"
	"os"
	"unsafe"
)

func (u *multiplayerUI) theme() *app { return &app{dpi: u.dpi, fonts: u.fonts} }
func (u *multiplayerUI) cardBrush() uintptr {
	if u.panelBrush == 0 {
		u.panelBrush, _, _ = createBrush.Call(colorCard)
	}
	return u.panelBrush
}
func (u *multiplayerUI) sessionGeometry() (int, int, int) {
	var r rect
	getClientRect.Call(u.window, uintptr(unsafe.Pointer(&r)))
	width := int(float64(r.Right) / u.dpi)
	if width < 860 {
		width = 860
	}
	return width, width - 334, width - 306
}
func (u *multiplayerUI) layoutSession() {
	if u.layingOut {
		return
	}
	u.layingOut = true
	defer func() { u.layingOut = false }()
	var r rect
	getClientRect.Call(u.window, uintptr(unsafe.Pointer(&r)))
	if r.Bottom < 1 {
		return
	}
	width, leftRight, hostX := u.sessionGeometry()
	total := 714
	if u.localTest {
		total = 980
	}
	maximum := u.px(total) - r.Bottom
	if maximum < 0 {
		maximum = 0
	}
	if u.scroll < 0 {
		u.scroll = 0
	}
	if u.scroll > maximum {
		u.scroll = maximum
	}
	si := scrollInfo{Size: uint32(unsafe.Sizeof(scrollInfo{})), Mask: 7, Max: u.px(total) - 1, Page: uint32(r.Bottom), Pos: u.scroll}
	setScrollInfo.Call(u.window, 1, uintptr(unsafe.Pointer(&si)), 0)
	shown := map[int]bool{}
	place := func(id, x, y, w, h int) {
		shown[id] = true
		u.placements[id] = rect{int32(x), int32(y), int32(w), int32(h)}
		moveWindow.Call(u.controls[id], uintptr(u.px(x)), uintptr(u.px(y)-u.scroll), uintptr(u.px(w)), uintptr(u.px(h)), 0)
	}
	place(mpIP, 52, 190, leftRight-220, 36)
	place(mpPort, leftRight-144, 190, 120, 36)
	for i := 0; i < 2; i++ {
		if i == 1 && !u.localTest {
			continue
		}
		y := 270 + i*304
		place(mpName1+i, 52, y+72, leftRight-220, 36)
		place(mpRename1+i, leftRight-144, y+72, 120, 36)
		place(mpJoin1+i, 52, y+124, leftRight-232, 44)
		place(mpLeave1+i, leftRight-156, y+124, 132, 44)
		place(mpStatus1+i, 52, y+180, leftRight-76, 48)
	}
	place(mpLocalTest, 28, 516, leftRight-210, 36)
	if u.localTest {
		place(mpArrange, leftRight-162, 516, 162, 36)
	}
	bottom := 574
	if u.localTest {
		bottom = 836
	}
	place(mpSpawn, 28, bottom, 216, 38)
	if os.Getenv(core.NativePreviewEnv) != "1" {
		place(mpFOV, 28, bottom+66, 80, 34)
		place(mpSetFOV, 120, bottom+66, 126, 34)
	}
	place(mpBind, hostX+24, 192, width-hostX-76, 180)
	place(mpNetworks, hostX+24, 244, width-hostX-76, 34)
	place(mpHostPort, hostX+24, 322, width-hostX-76, 36)
	place(mpItemsMode, hostX+24, 366, width-hostX-76, 34)
	place(mpHost, hostX+24, 416, width-hostX-76, 44)
	place(mpHostStatus, hostX+24, 480, width-hostX-76, 100)
	place(mpLogs, hostX+24, 630, width-hostX-76, 38)
	for id, h := range u.controls {
		v := uintptr(0)
		if shown[id] {
			v = 5
		}
		showWindow.Call(h, v)
	}
	if u.localTest {
		u.set(mpLocalTest, "− Ocultar teste local")
	} else {
		u.set(mpLocalTest, "+ Testar dois jogos neste PC")
	}
	user32.NewProc("RedrawWindow").Call(u.window, 0, 0, 0x1|0x80|0x20)
}
func (u *multiplayerUI) paint(dc uintptr) {
	var r rect
	getClientRect.Call(u.window, uintptr(unsafe.Pointer(&r)))
	fillRect.Call(dc, uintptr(unsafe.Pointer(&r)), u.a.bgBrush)
	v := u.theme()
	p := u.px
	width, leftRight, hostX := u.sessionGeometry()
	setViewport.Call(dc, 0, uintptr(-u.scroll), 0)
	label := func(s, font string, c uintptr, x, y, right, bottom int) {
		v.label(dc, s, font, c, rect{p(x), p(y), p(right), p(bottom)}, 0x10)
	}
	label("Jogue em companhia.", "title", colorText, 28, 28, width-28, 72)
	label("Conecte a um servidor ou crie um encontro na sua rede.", "body", colorMuted, 28, 84, width-28, 114)
	v.box(dc, rect{p(28), p(138), p(leftRight), p(246)}, colorCard, colorLine, 14)
	label("CONEXÃO DIRETA", "label", colorGold, 52, 154, leftRight-24, 172)
	label("IP do servidor", "small", colorMuted, 52, 173, leftRight-170, 190)
	label("Porta", "small", colorMuted, leftRight-144, 173, leftRight-24, 190)
	for i := 0; i < 2; i++ {
		if i == 1 && !u.localTest {
			continue
		}
		y := 270 + i*304
		v.box(dc, rect{p(28), p(y), p(leftRight), p(y + 230)}, colorCard, colorLine, 14)
		title := "SEU JOGADOR"
		if i == 1 {
			title = "SEGUNDO JOGO · TESTE LOCAL"
		}
		label(title, "label", colorGold, 52, y+18, leftRight-24, y+40)
		label("Nome em jogo", "small", colorMuted, 52, y+48, leftRight-24, y+70)
	}
	v.box(dc, rect{p(hostX), p(138), p(width - 28), p(686)}, colorCard, colorLine, 14)
	label("HOSPEDAR", "label", colorGold, hostX+24, 154, width-52, 172)
	label("Interface · LAN ou VPN/Radmin", "small", colorMuted, hostX+24, 173, width-52, 190)
	label("Porta do seu servidor", "small", colorMuted, hostX+24, 296, width-52, 320)
	foot := 626
	if os.Getenv(core.NativePreviewEnv) != "1" {
		foot = 700
	}
	if u.localTest {
		foot = 890
		if os.Getenv(core.NativePreviewEnv) != "1" {
			foot = 970
		}
	}
	label("Fechar este menu mantém a sessão.\nSair do launcher encerra a conexão de rede.", "small", colorMuted, 28, foot, leftRight, foot+44)
	if u.server != nil {
		label(fmt.Sprintf("%d / 2 jogadores", u.server.Count()), "small", colorGreen, hostX+24, 596, width-52, 620)
	} else {
		label("Sessão local · até 2 jogadores", "small", colorMuted, hostX+24, 596, width-52, 620)
	}
	setViewport.Call(dc, 0, 0, 0)
}
