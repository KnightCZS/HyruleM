//go:build windows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"

	core "hyrulelauncher/internal/launcher"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	gdi32            = syscall.NewLazyDLL("gdi32.dll")
	shell32          = syscall.NewLazyDLL("shell32.dll")
	comdlg32         = syscall.NewLazyDLL("comdlg32.dll")
	ole32            = syscall.NewLazyDLL("ole32.dll")
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	registerClass    = user32.NewProc("RegisterClassExW")
	createWindow     = user32.NewProc("CreateWindowExW")
	defWindow        = user32.NewProc("DefWindowProcW")
	showWindow       = user32.NewProc("ShowWindow")
	updateWindow     = user32.NewProc("UpdateWindow")
	getMessage       = user32.NewProc("GetMessageW")
	translateMessage = user32.NewProc("TranslateMessage")
	dispatchMessage  = user32.NewProc("DispatchMessageW")
	postMessage      = user32.NewProc("PostMessageW")
	sendMessage      = user32.NewProc("SendMessageW")
	destroyWindow    = user32.NewProc("DestroyWindow")
	postQuit         = user32.NewProc("PostQuitMessage")
	beginPaint       = user32.NewProc("BeginPaint")
	endPaint         = user32.NewProc("EndPaint")
	getClientRect    = user32.NewProc("GetClientRect")
	invalidate       = user32.NewProc("InvalidateRect")
	setText          = user32.NewProc("SetWindowTextW")
	getText          = user32.NewProc("GetWindowTextW")
	getTextLength    = user32.NewProc("GetWindowTextLengthW")
	enableWindow     = user32.NewProc("EnableWindow")
	moveWindow       = user32.NewProc("MoveWindow")
	fillRect         = user32.NewProc("FillRect")
	drawText         = user32.NewProc("DrawTextW")
	messageBox       = user32.NewProc("MessageBoxW")
	loadCursor       = user32.NewProc("LoadCursorW")
	setTimer         = user32.NewProc("SetTimer")
	getDPI           = user32.NewProc("GetDpiForWindow")
	setScrollInfo    = user32.NewProc("SetScrollInfo")
	getScrollInfo    = user32.NewProc("GetScrollInfo")
	setWindowPos     = user32.NewProc("SetWindowPos")
	isDialogMessage  = user32.NewProc("IsDialogMessageW")
	setViewport      = gdi32.NewProc("SetViewportOrgEx")
	createBrush      = gdi32.NewProc("CreateSolidBrush")
	createPen        = gdi32.NewProc("CreatePen")
	selectObject     = gdi32.NewProc("SelectObject")
	deleteObject     = gdi32.NewProc("DeleteObject")
	roundRect        = gdi32.NewProc("RoundRect")
	setTextColor     = gdi32.NewProc("SetTextColor")
	setBkColor       = gdi32.NewProc("SetBkColor")
	setBkMode        = gdi32.NewProc("SetBkMode")
	createFont       = gdi32.NewProc("CreateFontW")
	shellExecute     = shell32.NewProc("ShellExecuteW")
	browseFolder     = shell32.NewProc("SHBrowseForFolderW")
	folderPath       = shell32.NewProc("SHGetPathFromIDListW")
	openFile         = comdlg32.NewProc("GetOpenFileNameW")
	getModule        = kernel32.NewProc("GetModuleHandleW")
	createMutex      = kernel32.NewProc("CreateMutexW")
	lastError        = kernel32.NewProc("GetLastError")
)

const (
	wmDestroy       = 2
	wmSize          = 5
	wmPaint         = 15
	wmClose         = 16
	wmCommand       = 0x111
	wmDrawItem      = 0x2b
	wmColorEdit     = 0x133
	wmColorList     = 0x134
	wmColorStatic   = 0x138
	wmEvent         = 0x8001
	idBackend       = 101
	idExe           = 102
	idBrowseExe     = 103
	idData          = 104
	idBrowseData    = 105
	idGame          = 106
	idBrowseGame    = 107
	idPlay          = 110
	idSetup         = 111
	idDownload      = 112
	idLogs          = 113
	idSaves         = 114
	idSave          = 115
	idSandbox       = 120
	idNewSandbox    = 121
	idSeed          = 122
	idAutoSeed      = 123
	idPlace         = 124
	idAction        = 125
	idApply         = 126
	idTeleport      = 127
	idSandboxSaves  = 128
	idFilter        = 129
	idRebuildFilter = 130
)

type point struct{ X, Y int32 }
type scrollInfo struct {
	Size, Mask uint32
	Min, Max   int32
	Page       uint32
	Pos, Track int32
}
type rect struct{ Left, Top, Right, Bottom int32 }
type msg struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Point          point
	Private        uint32
}
type wndClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	SmallIcon                          uintptr
}
type paintStruct struct {
	DC                   uintptr
	Erase                int32
	Paint                rect
	Restore, Incremental int32
	Reserved             [32]byte
}
type drawItem struct {
	Type, ID, ItemID, Action, State uint32
	Window, DC                      uintptr
	Rect                            rect
	Data                            uintptr
}
type browseInfo struct {
	Owner, Root        uintptr
	DisplayName, Title *uint16
	Flags              uint32
	Callback, LParam   uintptr
	Image              int32
}
type openFileName struct {
	Size                         uint32
	Owner, Instance              uintptr
	Filter, CustomFilter         *uint16
	MaxCustomFilter, FilterIndex uint32
	File                         *uint16
	MaxFile                      uint32
	FileTitle                    *uint16
	MaxFileTitle                 uint32
	InitialDir, Title            *uint16
	Flags                        uint32
	FileOffset, FileExtension    uint16
	DefaultExt                   *uint16
	CustomData, Hook             uintptr
	TemplateName                 *uint16
	Reserved                     uintptr
	ReservedNumber, FlagsEx      uint32
}
type uiEvent struct {
	kind, message string
	progress      core.Progress
	install       core.InstallResult
	spec          core.LaunchSpec
	port          int
	generation    uint64
}
type app struct {
	layingOut          bool
	multiplayer        *multiplayerUI
	window, instance   uintptr
	controls           map[int]uintptr
	root, configPath   string
	config             core.Config
	session            *core.Session
	status             string
	statusError        bool
	installing         bool
	preparing          bool
	controlBusy        bool
	sandboxActive      bool
	controlPort        int
	controlCancel      context.CancelFunc
	sandboxGeneration  uint64
	cancel             context.CancelFunc
	queueMu            sync.Mutex
	queue              []uiEvent
	fonts              map[string]uintptr
	bgBrush, editBrush uintptr
	dpi                float64
	initial            bool
	smokePath          string
	scroll             int32
}

var current *app

func wide(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func textOf(h uintptr) string {
	n, _, _ := getTextLength.Call(h)
	b := make([]uint16, n+1)
	getText.Call(h, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
	return syscall.UTF16ToString(b)
}
func rgb(r, g, b byte) uintptr { return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16 }

var (
	colorBG    = rgb(13, 19, 26)
	colorCard  = rgb(23, 32, 42)
	colorEdit  = rgb(15, 23, 32)
	colorText  = rgb(233, 236, 231)
	colorMuted = rgb(152, 167, 177)
	colorGold  = rgb(218, 188, 115)
	colorLine  = rgb(43, 56, 68)
	colorRed   = rgb(236, 137, 126)
	colorGreen = rgb(145, 204, 170)
)

func (a *app) px(n int) int32 { return int32(float64(n) * a.dpi) }
func main() {
	runtime.LockOSThread()
	root, err := core.AppDir()
	if err != nil {
		return
	}
	_ = os.MkdirAll(root, 0700)
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(core.Version)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--diagnostico" {
		data, _ := json.MarshalIndent(map[string]any{"Version": core.Version, "Root": root, "Windows": runtime.GOOS, "Architecture": runtime.GOARCH}, "", "  ")
		_ = os.WriteFile(filepath.Join(root, "diagnostico-inicial.json"), data, 0600)
		return
	}
	awareness := user32.NewProc("SetProcessDpiAwarenessContext")
	if awareness.Find() == nil {
		awareness.Call(^uintptr(3))
	}
	ole32.NewProc("CoInitializeEx").Call(0, 2)
	defer ole32.NewProc("CoUninitialize").Call()
	mutex, _, mutexErr := createMutex.Call(0, 0, uintptr(unsafe.Pointer(wide("Local\\HyruleLauncher-Offline-v01"))))
	if mutexErr == syscall.Errno(183) {
		messageBox.Call(0, uintptr(unsafe.Pointer(wide("O Hyrule Launcher já está aberto."))), uintptr(unsafe.Pointer(wide("Hyrule Launcher"))), 0x40)
		if mutex != 0 {
			syscall.CloseHandle(syscall.Handle(mutex))
		}
		return
	}
	if mutex != 0 {
		defer syscall.CloseHandle(syscall.Handle(mutex))
	}
	configPath := filepath.Join(root, "config.json")
	cfg, loadErr := core.LoadConfig(configPath)
	if loadErr != nil {
		cfg = core.DefaultConfig()
	}
	instance, _, _ := getModule.Call(0)
	a := &app{instance: instance, controls: map[int]uintptr{}, root: root, configPath: configPath, config: cfg, session: core.NewSession(core.SystemFinder{}), dpi: 1, fonts: map[string]uintptr{}, initial: true}
	if len(os.Args) == 3 && os.Args[1] == "--smoke-test" {
		a.smokePath = os.Args[2]
	}
	current = a
	a.bgBrush, _, _ = createBrush.Call(colorBG)
	a.editBrush, _, _ = createBrush.Call(colorEdit)
	cursor, _, _ := loadCursor.Call(0, 32512)
	icon, _, _ := user32.NewProc("LoadIconW").Call(instance, 1)
	name := wide("HyruleLauncherMain")
	wc := wndClass{Size: uint32(unsafe.Sizeof(wndClass{})), Proc: syscall.NewCallback(windowProc), Instance: instance, Icon: icon, SmallIcon: icon, Cursor: cursor, Background: a.bgBrush, ClassName: name}
	if r, _, _ := registerClass.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return
	}
	a.window, _, _ = createWindow.Call(0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(wide("Hyrule Launcher | Offline"))), 0x02ef0000, 0x80000000, 0x80000000, 1060, 820, 0, 0, instance, 0)
	if a.window == 0 {
		return
	}
	if getDPI.Find() == nil {
		if dpi, _, _ := getDPI.Call(a.window); dpi != 0 {
			a.dpi = float64(dpi) / 96
		}
	}
	a.makeFonts()
	var work rect
	user32.NewProc("SystemParametersInfoW").Call(48, 0, uintptr(unsafe.Pointer(&work)), 0)
	width, height := a.px(1060), a.px(820)
	if work.Right > 0 && width > work.Right-work.Left {
		width = work.Right - work.Left
	}
	if work.Bottom > 0 && height > work.Bottom-work.Top {
		height = work.Bottom - work.Top
	}
	setWindowPos.Call(a.window, 0, uintptr(work.Left), uintptr(work.Top), uintptr(width), uintptr(height), 4|16)
	a.buildControls()
	a.initial = false
	a.fillControls()
	a.refresh()
	if loadErr != nil {
		a.setStatus(loadErr.Error(), true)
		core.AppendLog(core.LogPath(root), loadErr.Error())
	}
	core.AppendLog(core.LogPath(root), "Hyrule Launcher "+core.Version+" iniciado")
	showWindow.Call(a.window, 1)
	updateWindow.Call(a.window)
	setTimer.Call(a.window, 1, 1000, 0)
	if a.smokePath != "" {
		a.openMultiplayer()
		setTimer.Call(a.window, 2, 2500, 0)
	}
	var m msg
	for {
		r, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if a.multiplayer != nil {
			if handled, _, _ := isDialogMessage.Call(a.multiplayer.window, uintptr(unsafe.Pointer(&m))); handled != 0 {
				continue
			}
		}
		if handled, _, _ := isDialogMessage.Call(a.window, uintptr(unsafe.Pointer(&m))); handled != 0 {
			continue
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&m)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}
func (a *app) makeFonts() {
	for key, spec := range map[string][2]int{"body": {14, 400}, "small": {12, 400}, "title": {29, 600}, "brand": {24, 600}, "label": {12, 600}, "button": {14, 600}} {
		height := a.px(spec[0])
		f, _, _ := createFont.Call(uintptr(-height), 0, 0, 0, uintptr(spec[1]), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(wide("Segoe UI"))))
		a.fonts[key] = f
	}
}
func (a *app) control(id int, class, label string, style uintptr) uintptr {
	h, _, _ := createWindow.Call(0, uintptr(unsafe.Pointer(wide(class))), uintptr(unsafe.Pointer(wide(label))), 0x50010000|style, 0, 0, 10, 10, a.window, uintptr(id), a.instance, 0)
	a.controls[id] = h
	sendMessage.Call(h, 0x30, a.fonts["body"], 1)
	return h
}
func (a *app) buildControls() {
	a.control(idBackend, "COMBOBOX", "", 0x0003|0x00200000)
	for _, s := range []string{"Savage Wind · Switch", "BotW-Native · Wii U"} {
		sendMessage.Call(a.controls[idBackend], 0x143, 0, uintptr(unsafe.Pointer(wide(s))))
	}
	a.control(idExe, "EDIT", "", 0x00800000|0x0080)
	a.control(idData, "EDIT", "", 0x00800000|0x0080)
	a.control(idGame, "EDIT", "", 0x00800000|0x0080)
	for id, hint := range map[int]string{idExe: "Selecione o executável da recomp", idData: "Pasta que guarda os dados da recomp", idGame: "Pasta com code, content e meta"} {
		sendMessage.Call(a.controls[id], 0x1501, 0, uintptr(unsafe.Pointer(wide(hint))))
	}
	installLabel := "Baixar Savage Wind · 94 MB"
	if bundledArchive() != "" {
		installLabel = "Instalar Savage Wind · pacote incluído"
	}
	for _, button := range []struct {
		id    int
		label string
	}{{idBrowseExe, "Procurar"}, {idBrowseData, "Procurar"}, {idBrowseGame, "Procurar"}, {idPlay, "Jogar campanha"}, {idSetup, "Preparar jogo"}, {idDownload, installLabel}, {idLogs, "Abrir logs"}, {idSaves, "Pasta de saves"}, {idSave, "Salvar caminhos"}} {
		a.control(button.id, "BUTTON", button.label, 0x000b)
	}
	a.buildSandboxControls()
	a.control(idMultiplayer, "BUTTON", "Multiplayer · 2 jogadores", 0x000b)
	a.control(141, "BUTTON", "Cancelar preparação", 0x000b)
}
func (a *app) fillControls() {
	a.initial = true
	index := uintptr(0)
	if a.config.Backend == core.Native {
		index = 1
	}
	sendMessage.Call(a.controls[idBackend], 0x14e, index, 0)
	setText.Call(a.controls[idExe], uintptr(unsafe.Pointer(wide(a.config.Executable))))
	setText.Call(a.controls[idData], uintptr(unsafe.Pointer(wide(a.config.DataDir))))
	setText.Call(a.controls[idGame], uintptr(unsafe.Pointer(wide(a.config.GameDir))))
	a.fillSandboxControls()
	a.initial = false
	a.layout()
}
func (a *app) gather() core.Config {
	index, _, _ := sendMessage.Call(a.controls[idBackend], 0x147, 0, 0)
	backend := core.Savage
	if index == 1 {
		backend = core.Native
	}
	c := core.Config{Schema: 1, Backend: backend, Executable: textOf(a.controls[idExe]), DataDir: textOf(a.controls[idData]), GameDir: textOf(a.controls[idGame]), SandboxSeed: a.config.SandboxSeed, SandboxPlace: a.selectedPlace(), KeepCampaign: a.keepCampaign()}
	c.Normalize()
	return c
}
func (a *app) save() bool {
	a.config = a.gather()
	if err := core.SaveConfig(a.configPath, a.config); err != nil {
		a.setStatus("Não foi possível salvar os caminhos: "+err.Error(), true)
		return false
	}
	return true
}
func (a *app) setStatus(s string, bad bool) {
	a.status = s
	a.statusError = bad
	invalidate.Call(a.window, 0, 0)
}
func (a *app) layout() {
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
	maxScroll := a.px(1300) - r.Bottom
	if maxScroll < 0 {
		maxScroll = 0
	}
	if a.scroll > maxScroll {
		a.scroll = maxScroll
	}
	if a.scroll < 0 {
		a.scroll = 0
	}
	si := scrollInfo{Size: uint32(unsafe.Sizeof(scrollInfo{})), Mask: 7, Max: a.px(1300) - 1, Page: uint32(r.Bottom), Pos: a.scroll}
	setScrollInfo.Call(a.window, 1, uintptr(unsafe.Pointer(&si)), 1)
	left := a.px(260)
	right := r.Right - a.px(38)
	width := right - left
	place := func(id int, x, y, width, height int32) {
		offset := a.scroll
		if id == idMultiplayer || id == 141 {
			offset = 0
		}
		moveWindow.Call(a.controls[id], uintptr(x), uintptr(y-offset), uintptr(width), uintptr(height), 0)
	}
	place(idBackend, left+a.px(24), a.px(202), width-a.px(48), a.px(160))
	for _, row := range []struct{ edit, button, y int }{{idExe, idBrowseExe, 274}, {idData, idBrowseData, 348}, {idGame, idBrowseGame, 422}} {
		place(row.edit, left+a.px(24), a.px(row.y), width-a.px(158), a.px(34))
		place(row.button, right-a.px(122), a.px(row.y), a.px(98), a.px(34))
	}
	half := (width - a.px(14)) / 2
	place(idPlay, left, a.px(520), half, a.px(48))
	place(idSetup, left+half+a.px(14), a.px(520), half, a.px(48))
	place(idDownload, left, a.px(584), width, a.px(42))
	third := (width - a.px(20)) / 3
	place(idLogs, left, a.px(642), third, a.px(38))
	place(idSaves, left+third+a.px(10), a.px(642), third, a.px(38))
	place(idSave, left+2*(third+a.px(10)), a.px(642), third, a.px(38))
	place(idMultiplayer, a.px(38), a.px(460), a.px(169), a.px(62))
	place(141, a.px(38), a.px(540), a.px(169), a.px(38))
	a.layoutSandbox(left, right, place)
	native := a.config.Backend == core.Native
	for _, id := range []int{idGame, idBrowseGame} {
		visible := uintptr(0)
		if native {
			visible = 5
		}
		showWindow.Call(a.controls[id], visible)
	}
	user32.NewProc("RedrawWindow").Call(a.window, 0, 0, 0x1|0x80|0x20)
}
func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func (a *app) refresh() {
	if a.initial {
		return
	}
	a.config = a.gather()
	busy := a.installing || a.preparing || a.session.Busy() || a.mpBusy()
	for id, h := range a.controls {
		enabled := uintptr(1)
		if busy {
			enabled = 0
		}
		if id == idLogs || id == idSaves || id == idMultiplayer {
			enabled = 1
		}
		enableWindow.Call(h, enabled)
	}
	if !core.IsFile(a.config.Executable) {
		enableWindow.Call(a.controls[idPlay], 0)
	}
	if core.SetupProgram(a.config) == "" {
		enableWindow.Call(a.controls[idSetup], 0)
	}
	enableCancel := uintptr(0)
	if a.cancel != nil && (a.preparing || a.installing) {
		enableCancel = 1
	}
	enableWindow.Call(a.controls[141], enableCancel)
	a.refreshSandbox(busy)
	if a.status == "" {
		a.statusError = false
		if !core.IsFile(a.config.Executable) {
			a.status = "Selecione sua recomp ou use o download oficial abaixo."
		} else if core.PreparationNeeded(a.config) {
			a.status = "Instalação encontrada. Faça a preparação inicial do jogo."
		} else {
			a.status = "Pronto para iniciar. O carregamento será feito pela recomp."
		}
	}
	invalidate.Call(a.window, 0, 0)
}
func (a *app) post(e uiEvent) {
	a.queueMu.Lock()
	wake := len(a.queue) == 0
	if e.kind == "progress" && len(a.queue) > 0 && a.queue[len(a.queue)-1].kind == "progress" {
		a.queue[len(a.queue)-1] = e
	} else {
		a.queue = append(a.queue, e)
	}
	a.queueMu.Unlock()
	if wake {
		postMessage.Call(a.window, wmEvent, 0, 0)
	}
}
func (a *app) drain() {
	a.queueMu.Lock()
	events := a.queue
	a.queue = nil
	a.queueMu.Unlock()
	if len(events) == 0 {
		return
	}
	refresh := false
	for _, e := range events {
		if e.kind != "progress" {
			refresh = true
		}
		if strings.HasPrefix(e.kind, "control-") && e.generation != a.sandboxGeneration {
			continue
		}
		switch e.kind {
		case "progress":
			label := e.progress.Stage
			if e.progress.Total > 0 {
				label += fmt.Sprintf(" · %d%%", 100*e.progress.Done/e.progress.Total)
			}
			a.setStatus(label, false)
		case "sandbox-ready":
			a.preparing = false
			a.cancel = nil
			a.launchSandbox(e.spec, e.port)
		case "launch-done":
			a.preparing = false
			if a.session.Busy() {
				a.setStatus(e.message, false)
			}
		case "launch-failed":
			a.preparing = false
			a.sandboxActive = false
			a.controlPort = 0
			a.setStatus(e.message, true)
		case "sandbox-failed":
			a.preparing = false
			a.cancel = nil
			a.setStatus(e.message, true)
			core.AppendLog(core.LogPath(a.root), e.message)
		case "filter-ready":
			a.preparing = false
			a.cancel = nil
			a.setStatus(e.message, false)
			core.AppendLog(core.LogPath(a.root), e.message)
		case "control-done", "control-failed":
			a.controlBusy = false
			a.controlCancel = nil
			a.setStatus(e.message, e.kind == "control-failed")
			core.AppendLog(core.LogPath(a.root), e.message)
		case "installed":
			a.installing = false
			a.cancel = nil
			a.config = core.Config{Schema: 1, Backend: core.Savage, Executable: e.install.Executable, DataDir: filepath.Dir(e.install.Executable)}
			a.fillControls()
			if a.save() {
				a.setStatus("Savage Wind instalado. Clique em Preparar jogo e selecione seus arquivos de BOTW.", false)
			}
			core.AppendLog(core.LogPath(a.root), "Recomp instalada e verificada: "+e.install.Executable)
		case "install-failed":
			a.installing = false
			a.cancel = nil
			a.setStatus(e.message, true)
			core.AppendLog(core.LogPath(a.root), e.message)
		default:
			if e.kind == "ended" || e.kind == "failed" {
				if a.controlCancel != nil {
					a.controlCancel()
					a.controlCancel = nil
				}
				a.controlBusy = false
				a.sandboxGeneration++
				a.sandboxActive = false
				a.controlPort = 0
			}
			a.setStatus(e.message, e.kind == "failed")
			core.AppendLog(core.LogPath(a.root), e.message)
		}
	}
	if refresh {
		a.refresh()
	}
}
func (a *app) start(setup bool) {
	if a.installing || a.preparing || a.session.Busy() || a.mpBusy() {
		return
	}
	if !a.save() {
		return
	}
	spec, err := core.BuildLaunch(a.config, setup, os.Environ())
	if err != nil {
		a.setStatus(err.Error(), true)
		return
	}
	if err = os.MkdirAll(a.config.DataDir, 0700); err != nil {
		a.setStatus("Não foi possível acessar a pasta de dados: "+err.Error(), true)
		return
	}
	if !setup && a.config.Backend == core.Savage && core.PreparationNeeded(a.config) {
		spec, err = core.BuildLaunch(a.config, true, os.Environ())
		if err != nil {
			a.setStatus(err.Error(), true)
			return
		}
	}
	core.AppendLog(core.LogPath(a.root), fmt.Sprintf("Iniciando %s %v (pasta: %s)", spec.Executable, spec.Args, spec.Dir))
	capture := filepath.Join(a.root, "logs", "processo.log")
	a.sandboxActive = false
	a.controlPort = 0
	a.sandboxGeneration++
	a.preparing = true
	go func() {
		err := a.session.Start(spec, capture, func(e core.Event) { a.post(uiEvent{kind: e.Kind, message: e.Message}) })
		if err != nil {
			a.post(uiEvent{kind: "launch-failed", message: err.Error()})
		} else {
			a.post(uiEvent{kind: "launch-done", message: "Programa iniciado. Aguarde o carregamento da recomp."})
		}
	}()
	a.refresh()
}
func (a *app) download() {
	if a.installing || a.preparing || a.session.Busy() || a.mpBusy() {
		return
	}
	a.installing = true
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.setStatus("Preparando a instalação do Savage Wind…", false)
	a.refresh()
	go func() {
		result, err := core.InstallSavageFrom(ctx, a.root, bundledArchive(), func(p core.Progress) { a.post(uiEvent{kind: "progress", progress: p}) })
		if err != nil {
			a.post(uiEvent{kind: "install-failed", message: err.Error()})
			return
		}
		a.post(uiEvent{kind: "installed", install: result})
	}()
}
func bundledArchive() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	candidate := filepath.Join(filepath.Dir(exe), "runtime", "SavageWind-0.5.3-alpha.5-windows.zip")
	if core.IsFile(candidate) {
		return candidate
	}
	return ""
}
func (a *app) chooseExe() {
	buffer := make([]uint16, 32768)
	filter := utf16.Encode([]rune("Programas da recomp (*.exe)\x00*.exe\x00Todos os arquivos\x00*.*\x00\x00"))
	o := openFileName{Size: uint32(unsafe.Sizeof(openFileName{})), Owner: a.window, Filter: &filter[0], File: &buffer[0], MaxFile: uint32(len(buffer)), Title: wide("Selecione savagewind.exe ou BotWRecomp.exe"), Flags: 0x1000 | 0x800 | 0x80000 | 0x8}
	r, _, _ := openFile.Call(uintptr(unsafe.Pointer(&o)))
	if r == 0 {
		return
	}
	p := syscall.UTF16ToString(buffer)
	backend := core.Identify(p)
	if backend == core.Native && !strings.EqualFold(filepath.Base(p), "BotWRecomp.exe") {
		p = filepath.Join(filepath.Dir(p), "BotWRecomp.exe")
	}
	a.config = core.Config{Schema: 1, Backend: backend, Executable: p, GameDir: a.config.GameDir, DataDir: core.SuggestDataDir(p, backend)}
	a.fillControls()
	a.save()
	a.status = ""
	a.refresh()
}
func (a *app) chooseFolder(title string) string {
	display := make([]uint16, 260)
	b := browseInfo{Owner: a.window, DisplayName: &display[0], Title: wide(title), Flags: 0x1 | 0x40}
	pidl, _, _ := browseFolder.Call(uintptr(unsafe.Pointer(&b)))
	if pidl == 0 {
		return ""
	}
	defer ole32.NewProc("CoTaskMemFree").Call(pidl)
	buffer := make([]uint16, 32768)
	ok, _, _ := folderPath.Call(pidl, uintptr(unsafe.Pointer(&buffer[0])))
	if ok == 0 {
		return ""
	}
	return syscall.UTF16ToString(buffer)
}
func (a *app) openPath(path string) {
	r, _, _ := shellExecute.Call(a.window, uintptr(unsafe.Pointer(wide("open"))), uintptr(unsafe.Pointer(wide(path))), 0, 0, 1)
	if r <= 32 {
		a.setStatus("Não foi possível abrir este arquivo ou pasta.", true)
	}
}
func (a *app) logs() {
	c := a.gather()
	body := "HYRULE LAUNCHER " + core.Version + "\r\n\r\nLAUNCHER\r\n" + core.ReadLogTail(core.LogPath(a.root), 48000) + "\r\n\r\nPROCESSO\r\n" + core.ReadLogTail(filepath.Join(a.root, "logs", "processo.log"), 48000)
	body += "\r\n\r\nSANDBOX\r\n" + core.ReadLogTail(filepath.Join(a.root, "logs", "sandbox-processo.log"), 48000)
	if r, err := core.CampaignFilterReport(c); err == nil {
		summary, _ := json.MarshalIndent(map[string]any{"Version": r.Launcher, "Signature": r.Signature, "RemovedMain": r.RemovedMain, "SideQuests": len(r.KeptSide), "ShrineQuests": len(r.KeptShrine), "BlockedFlows": r.BlockedFlows, "RemovedMapTriggers": r.RemovedMapTriggers, "Outputs": len(r.Outputs), "Policy": r.Policy, "Enabled": !c.KeepCampaign}, "", "  ")
		body += "\r\n\r\nFILTRO DE CAMPANHA\r\n" + string(summary)
	}
	if c.DataDir != "" {
		body += "\r\n\r\nRECOMP: " + core.RuntimeLog(c) + "\r\n" + core.ReadLogTail(core.RuntimeLog(c), 96000)
	}
	for _, name := range []string{"multiplayer-1.log", "multiplayer-2.log", "multiplayer-processo-1.log", "multiplayer-processo-2.log", "servidor.log"} {
		body += "\r\n\r\n" + name + "\r\n" + core.ReadLogTail(filepath.Join(a.root, "logs", name), 48000)
	}
	if c.DataDir != "" {
		for i := 0; i < 2; i++ {
			private := c
			private.DataDir = core.InstanceDataDir(c, i)
			body += "\r\n\r\nRUNTIME CLIENTE " + fmt.Sprint(i+1) + "\r\n" + core.ReadLogTail(core.RuntimeLog(private), 48000)
		}
	}
	file := filepath.Join(a.root, "logs", "diagnostico.txt")
	_ = os.MkdirAll(filepath.Dir(file), 0700)
	if err := os.WriteFile(file, []byte(body), 0600); err != nil {
		a.setStatus(err.Error(), true)
		return
	}
	a.openPath(file)
}
func windowProc(h uintptr, message uint32, w, l uintptr) uintptr {
	a := current
	if a == nil {
		return defaultProc(h, message, w, l)
	}
	switch message {
	case 0x14:
		return 1
	case wmPaint:
		var ps paintStruct
		dc, _, _ := beginPaint.Call(h, uintptr(unsafe.Pointer(&ps)))
		a.paintBuffered(dc)
		endPaint.Call(h, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmSize:
		if len(a.controls) > 0 {
			a.layout()
		}
		return 0
	case 0x24:
		// MINMAXINFO: reserved, max size, max pos, min track, max track.
		values := (*[10]int32)(unsafe.Pointer(l))
		values[6] = a.px(870)
		values[7] = a.px(660)
		return 0
	case 0x115:
		si := scrollInfo{Size: uint32(unsafe.Sizeof(scrollInfo{})), Mask: 16}
		getScrollInfo.Call(h, 1, uintptr(unsafe.Pointer(&si)))
		switch w & 0xffff {
		case 0:
			a.scroll -= a.px(35)
		case 1:
			a.scroll += a.px(35)
		case 2:
			a.scroll -= a.px(300)
		case 3:
			a.scroll += a.px(300)
		case 4, 5:
			a.scroll = si.Track
		}
		a.layout()
		return 0
	case 0x20a:
		delta := int16(w >> 16)
		a.scroll -= int32(delta) * a.px(45) / 120
		a.layout()
		return 0
	case 0x2e0:
		a.dpi = float64(w&0xffff) / 96
		for _, f := range a.fonts {
			deleteObject.Call(f)
		}
		a.makeFonts()
		for _, control := range a.controls {
			sendMessage.Call(control, 0x30, a.fonts["body"], 1)
		}

		suggested := (*rect)(unsafe.Pointer(l))
		setWindowPos.Call(h, 0, uintptr(suggested.Left), uintptr(suggested.Top), uintptr(suggested.Right-suggested.Left), uintptr(suggested.Bottom-suggested.Top), 4|16)
		a.layout()
		return 0
	case wmDrawItem:
		a.paintButton((*drawItem)(unsafe.Pointer(l)))
		return 1
	case wmColorEdit, wmColorStatic, wmColorList:
		setTextColor.Call(w, colorText)
		setBkColor.Call(w, colorEdit)
		return a.editBrush
	case wmCommand:
		if a.initial {
			return 0
		}
		id := int(w & 0xffff)
		notification := uint16(w >> 16)
		if id == idBackend && notification == 1 {
			old := a.config.Backend
			a.config = a.gather()
			if old != a.config.Backend && a.config.Executable != "" {
				if core.Identify(a.config.Executable) != a.config.Backend {
					a.config.Executable = ""
					a.config.DataDir = ""
				} else {
					a.config.DataDir = core.SuggestDataDir(a.config.Executable, a.config.Backend)
				}
			}
			a.fillControls()
			a.save()
			a.status = ""
			a.refresh()
			return 0
		}
		if notification != 0 {
			return 0
		}
		switch id {
		case 141:
			if a.cancel != nil {
				a.cancel()
				a.setStatus("Cancelando preparação; aguarde a tarefa encerrar…", false)
			}
		case idMultiplayer:
			a.openMultiplayer()
		case idBrowseExe:
			a.chooseExe()
		case idBrowseData:
			if p := a.chooseFolder("Selecione a pasta de dados da recomp (saves e configuração)"); p != "" {
				setText.Call(a.controls[idData], uintptr(unsafe.Pointer(wide(p))))
				a.save()
				a.status = ""
				a.refresh()
			}
		case idBrowseGame:
			if p := a.chooseFolder("Selecione BOTW Wii U: a pasta com code, content e meta"); p != "" {
				setText.Call(a.controls[idGame], uintptr(unsafe.Pointer(wide(p))))
				a.save()
				a.status = ""
				a.refresh()
			}
		case idSandbox:
			a.startSandbox(false)
		case idRebuildFilter:
			a.rebuildFilter()
		case idNewSandbox:
			a.startSandbox(true)
		case idSeed:
			a.chooseSeed()
		case idAutoSeed:
			a.config.SandboxSeed = ""
			a.save()
			a.setStatus("Save base automático: perfil de campanha ativo, slot normal mais recente.", false)
			a.refresh()
		case idApply:
			a.applySandbox(false)
		case idTeleport:
			a.applySandbox(true)
		case idSandboxSaves:
			c := a.gather()
			if c.DataDir != "" && core.IsDir(core.SandboxDir(c)) {
				a.openPath(core.SandboxDir(c))
			} else {
				a.setStatus("Crie o sandbox primeiro.", true)
			}
		case idPlay, 1:
			a.start(false)
		case idSetup:
			a.start(true)
		case idDownload:
			a.download()
		case idLogs:
			a.logs()
		case idSave:
			if a.save() {
				a.setStatus("Caminhos salvos. Sua configuração será usada na próxima abertura.", false)
				a.refresh()
			}
		case idSaves:
			c := a.gather()
			if c.DataDir == "" {
				a.setStatus("Selecione a pasta de dados primeiro.", true)
			} else {
				p := core.SaveDir(c)
				if c.Backend == core.Savage {
					var err error
					p, err = core.CampaignRoot(c)
					if err != nil {
						a.setStatus(err.Error(), true)
						return 0
					}
				}
				if err := os.MkdirAll(p, 0700); err != nil {
					a.setStatus(err.Error(), true)
				} else {
					a.openPath(p)
				}
			}
		}
		return 0
	case wmEvent:
		a.drain()
		return 0
	case 0x113:
		if w == 2 && a.smokePath != "" {
			mpCount := 0
			if a.multiplayer != nil && a.multiplayer.window != 0 {
				mpCount = len(a.multiplayer.controls)
			}
			_ = os.WriteFile(a.smokePath, []byte("window-created\ncontrols="+fmt.Sprint(len(a.controls))+"\nmultiplayer-controls="+fmt.Sprint(mpCount)+"\nstatus="+a.status+"\n"), 0600)
			destroyWindow.Call(h)
			return 0
		}
		return 0
	case wmClose:
		if a.multiplayer != nil {
			a.multiplayer.shutdown()
		}
		if a.controlCancel != nil {
			a.controlCancel()
		}
		if a.cancel != nil {
			a.cancel()
		}
		if len(a.controls) > 0 {
			a.save()
		}
		destroyWindow.Call(h)
		return 0
	case wmDestroy:
		for _, f := range a.fonts {
			deleteObject.Call(f)
		}
		deleteObject.Call(a.bgBrush)
		deleteObject.Call(a.editBrush)
		postQuit.Call(0)
		return 0
	}
	return defaultProc(h, message, w, l)
}
func defaultProc(h uintptr, m uint32, w, l uintptr) uintptr {
	r, _, _ := defWindow.Call(h, uintptr(m), w, l)
	return r
}
func (a *app) box(dc uintptr, r rect, color, border uintptr, radius int) {
	brush, _, _ := createBrush.Call(color)
	pen, _, _ := createPen.Call(0, 1, border)
	oldB, _, _ := selectObject.Call(dc, brush)
	oldP, _, _ := selectObject.Call(dc, pen)
	roundRect.Call(dc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(a.px(radius)), uintptr(a.px(radius)))
	selectObject.Call(dc, oldB)
	selectObject.Call(dc, oldP)
	deleteObject.Call(brush)
	deleteObject.Call(pen)
}
func (a *app) label(dc uintptr, s, font string, color uintptr, r rect, flags uintptr) {
	old, _, _ := selectObject.Call(dc, a.fonts[font])
	setTextColor.Call(dc, color)
	setBkMode.Call(dc, 1)
	drawText.Call(dc, uintptr(unsafe.Pointer(wide(s))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), flags)
	selectObject.Call(dc, old)
}
func (a *app) paint(dc uintptr) {
	var r rect
	getClientRect.Call(a.window, uintptr(unsafe.Pointer(&r)))
	fillRect.Call(dc, uintptr(unsafe.Pointer(&r)), a.bgBrush)
	if len(a.fonts) == 0 {
		return
	}
	display := r
	p := a.px
	left := p(260)
	right := r.Right - p(38)
	a.box(dc, rect{p(20), p(20), p(224), r.Bottom - p(20)}, rgb(18, 27, 36), rgb(25, 37, 49), 18)
	a.label(dc, "△", "title", colorGold, rect{p(44), p(43), p(190), p(85)}, 0)
	a.label(dc, "HYRULE", "brand", colorText, rect{p(44), p(98), p(205), p(134)}, 0)
	a.label(dc, "L A U N C H E R", "small", colorMuted, rect{p(45), p(139), p(205), p(165)}, 0)
	a.box(dc, rect{p(38), p(210), p(207), p(252)}, rgb(39, 48, 52), rgb(65, 66, 54), 10)
	a.label(dc, "Campanha + sandbox", "button", colorGold, rect{p(54), p(210), p(202), p(252)}, 0x24)
	a.label(dc, "SEU MUNDO", "label", colorMuted, rect{p(45), p(295), p(205), p(319)}, 0)
	a.label(dc, "Campanha preservada.\nSandbox separado.\nExplore Hyrule.", "body", colorText, rect{p(45), p(330), p(201), p(430)}, 0x10)
	a.label(dc, "VERSÃO "+core.Version, "small", colorMuted, rect{p(45), p(178), p(205), p(202)}, 0)
	a.label(dc, "Windows · x64", "small", colorMuted, rect{p(45), p(586), p(205), p(610)}, 0)
	setViewport.Call(dc, 0, uintptr(-a.scroll), 0)
	if r.Bottom < a.px(1300) {
		r.Bottom = a.px(1300)
	}
	a.label(dc, "Hyrule, do seu jeito.", "title", colorText, rect{left, p(38), right, p(86)}, 0x24)
	a.label(dc, "Campanha offline e ferramentas para explorar o mundo livremente.", "body", colorMuted, rect{left, p(94), right, p(133)}, 0x10)
	a.box(dc, rect{left, p(154), right, p(494)}, colorCard, colorLine, 16)
	a.label(dc, "RECOMPILAÇÃO", "label", colorMuted, rect{left + p(24), p(173), right - p(24), p(198)}, 0)
	a.label(dc, "Programa da recomp", "label", colorMuted, rect{left + p(24), p(249), right - p(24), p(274)}, 0)
	a.label(dc, "Pasta de dados · saves e configuração", "label", colorMuted, rect{left + p(24), p(323), right - p(24), p(348)}, 0)
	if a.config.Backend == core.Native {
		a.label(dc, "Arquivos do jogo · Wii U v208", "label", colorMuted, rect{left + p(24), p(397), right - p(24), p(422)}, 0)
	} else {
		a.label(dc, "Primeira vez? Use Preparar jogo para selecionar os arquivos\nextraídos de BOTW para Switch 1.9.0.", "small", colorMuted, rect{left + p(24), p(410), right - p(24), p(467)}, 0x10)
	}
	statusColor := colorGreen
	if a.statusError {
		statusColor = colorRed
	}
	a.paintSandbox(dc, left, right)
	setViewport.Call(dc, 0, 0, 0)
	a.box(dc, rect{left, display.Bottom - p(72), right, display.Bottom - p(12)}, colorBG, colorLine, 8)
	a.label(dc, a.status, "small", statusColor, rect{left + p(12), display.Bottom - p(64), right - p(12), display.Bottom - p(18)}, 0x10)
}
func (a *app) paintButton(d *drawItem) {
	background, border, foreground := rgb(31, 43, 55), colorLine, colorText
	if int(d.ID) == idPlay || int(d.ID) == idSandbox {
		background = colorGold
		border = colorGold
		foreground = colorBG
	}
	if d.State&1 != 0 {
		background = rgb(83, 82, 64)
	}
	if d.State&4 != 0 {
		background = rgb(27, 36, 45)
		foreground = rgb(103, 119, 130)
		border = colorLine
	}
	if d.State&16 != 0 {
		border = colorGold
	}
	a.box(d.DC, d.Rect, background, border, 10)
	a.label(d.DC, textOf(d.Window), "button", foreground, d.Rect, 0x25)
}
