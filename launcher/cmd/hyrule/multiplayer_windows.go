//go:build windows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	core "hyrulelauncher/internal/launcher"
	mp "hyrulelauncher/internal/multiplayer"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const idMultiplayer = 140
const wmMP = 0x8002
const (
	mpHost       = 501
	mpIP         = 502
	mpPort       = 503
	mpName1      = 510
	mpName2      = 511
	mpJoin1      = 512
	mpJoin2      = 513
	mpLeave1     = 514
	mpLeave2     = 515
	mpRename1    = 516
	mpRename2    = 517
	mpStatus1    = 518
	mpStatus2    = 519
	mpArrange    = 520
	mpFOV        = 521
	mpSetFOV     = 522
	mpHostStatus = 523
	mpBind       = 524
	mpNetworks   = 525
	mpSpawn      = 526
	mpLogs       = 527
)

type mpSlot struct {
	session       *core.Session
	finder        *core.InstanceFinder
	bridge        *mp.Bridge
	pid, port     int
	game, overlay uintptr
	pending       bool
	generation    uint64
	cancel        context.CancelFunc
}
type mpSettings struct {
	IP     string
	BindIP string
	Port   int
	Names  [2]string
	FOV    float64
}
type multiplayerUI struct {
	a            *app
	window       uintptr
	controls     map[int]uintptr
	slots        [2]mpSlot
	server       *mp.Server
	settings     mpSettings
	networks     []mp.LocalAddress
	binds        []string
	scroll       int32
	placements   map[int]rect
	staticID     int
	layingOut    bool
	dpi          float64
	bodyFont     uintptr
	hostStopping bool
	preparer     *core.MultiplayerPreparer
	mu           sync.Mutex
	queue        []func()
	closed       bool
}

func (a *app) mpBusy() bool {
	if a.multiplayer == nil {
		return false
	}
	for i := range a.multiplayer.slots {
		s := &a.multiplayer.slots[i]
		if s.pending || s.session.Busy() {
			return true
		}
	}
	return false
}
func (a *app) openMultiplayer() {
	if a.multiplayer != nil {
		showWindow.Call(a.multiplayer.window, 5)
		user32.NewProc("SetForegroundWindow").Call(a.multiplayer.window)
		return
	}
	if !a.save() {
		return
	}
	u := &multiplayerUI{a: a, controls: map[int]uintptr{}, placements: map[int]rect{}, preparer: core.NewMultiplayerPreparer(), settings: mpSettings{IP: "127.0.0.1", Port: 5050, BindIP: "0.0.0.0", Names: [2]string{"Jogador 1", "Jogador 2"}, FOV: 40}}
	if b, e := os.ReadFile(filepath.Join(a.root, "multiplayer.json")); e == nil {
		_ = json.Unmarshal(b, &u.settings)
	}
	if _, e := mp.Address(u.settings.IP, strconv.Itoa(u.settings.Port)); e != nil {
		u.settings.IP = "127.0.0.1"
		u.settings.Port = 5050
	}
	if u.settings.BindIP == "" {
		u.settings.BindIP = "0.0.0.0"
	}
	for i := range u.slots {
		f := core.NewInstanceFinder()
		u.slots[i].finder = f
		u.slots[i].session = core.NewSession(f)
	}
	a.multiplayer = u
	cursor, _, _ := loadCursor.Call(0, 32512)
	for name, proc := range map[string]uintptr{"HyruleMultiplayer": syscall.NewCallback(multiplayerProc), "HyruleRemoteAvatar": syscall.NewCallback(overlayProc)} {
		wc := wndClass{Size: uint32(unsafe.Sizeof(wndClass{})), Proc: proc, Instance: a.instance, Cursor: cursor, Background: a.bgBrush, ClassName: wide(name)}
		if r, _, _ := registerClass.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
			a.setStatus("Não foi possível criar o menu multiplayer.", true)
			a.multiplayer = nil
			return
		}
	}
	u.window, _, _ = createWindow.Call(0, uintptr(unsafe.Pointer(wide("HyruleMultiplayer"))), uintptr(unsafe.Pointer(wide("HyruleM · Multiplayer experimental"))), 0x02ef0000, 100, 50, uintptr(a.px(880)), uintptr(a.px(740)), a.window, 0, a.instance, 0)
	if u.window == 0 {
		a.multiplayer = nil
		a.setStatus("Falha ao abrir multiplayer.", true)
		return
	}
	u.dpi = a.dpi
	if value, _, _ := getDPI.Call(u.window); value > 0 {
		u.dpi = float64(value) / 96
	}
	u.updateFont()
	var work rect
	user32.NewProc("SystemParametersInfoW").Call(48, 0, uintptr(unsafe.Pointer(&work)), 0)
	width, height := u.px(880), u.px(740)
	if width > work.Right-work.Left {
		width = work.Right - work.Left
	}
	if height > work.Bottom-work.Top {
		height = work.Bottom - work.Top
	}
	if width > 0 && height > 0 {
		setWindowPos.Call(u.window, 0, uintptr(work.Left+(work.Right-work.Left-width)/2), uintptr(work.Top), uintptr(width), uintptr(height), 4|16)
	}
	u.build()
	setTimer.Call(u.window, 3, 33, 0)
	showWindow.Call(u.window, 5)
}
func (u *multiplayerUI) add(id int, class, label string, style uintptr, x, y, w, h int) {
	a := u.a
	hwnd, _, _ := createWindow.Call(0, uintptr(unsafe.Pointer(wide(class))), uintptr(unsafe.Pointer(wide(label))), 0x50000000|style, uintptr(u.px(x)), uintptr(u.px(y)), uintptr(u.px(w)), uintptr(u.px(h)), u.window, uintptr(id), a.instance, 0)
	u.controls[id] = hwnd
	u.placements[id] = rect{int32(x), int32(y), int32(w), int32(h)}
	sendMessage.Call(hwnd, 0x30, u.bodyFont, 1)
}
func (u *multiplayerUI) label(s string, x, y, w, h int) {
	u.staticID++
	u.add(900+u.staticID, "STATIC", s, 0, x, y, w, h)
}
func (u *multiplayerUI) build() {
	native := os.Getenv(core.NativePreviewEnv) == "1"
	if native {
		u.label("MULTIPLAYER · TESTE DE PRESENÇA NATIVA", 28, 24, 800, 28)
		u.label("Prévia 8: horário/clima do anfitrião, animações e equipamento visual.\r\nAtualize os dois clientes e use Iniciar-Nativo.cmd (ABI 5 / protocolo 5).", 28, 62, 805, 48)
	} else {
		u.label("MULTIPLAYER · PRIMEIRO PROTÓTIPO", 28, 24, 800, 28)
		u.label("Avatares visuais provisórios. Inicie no mundo aberto de Hateno.\r\nMantenha este launcher aberto e os jogos em modo janela.", 28, 62, 805, 48)
	}
	u.label("IP do servidor", 28, 128, 240, 20)
	u.add(mpIP, "EDIT", u.settings.IP, 0x00810080, 28, 154, 290, 32)
	u.label("Porta TCP", 334, 128, 130, 20)
	u.add(mpPort, "EDIT", strconv.Itoa(u.settings.Port), 0x00812080, 334, 154, 100, 32)
	u.add(mpHost, "BUTTON", "Hospedar servidor", 0x10000, 454, 154, 180, 34)
	u.add(mpArrange, "BUTTON", "Organizar 2 janelas", 0x10000, 650, 154, 180, 34)
	u.label("Interface para hospedar · inclui rede virtual / Radmin", 28, 204, 580, 22)
	u.add(mpBind, "COMBOBOX", "", 0x10000|0x3|0x200000, 28, 230, 420, 180)
	u.add(mpNetworks, "BUTTON", "Atualizar IPs", 0x10000, 460, 230, 150, 32)
	u.add(mpLogs, "BUTTON", "Abrir diagnóstico", 0x10000, 624, 230, 206, 32)
	u.add(mpHostStatus, "STATIC", "Servidor parado. Selecione sua interface e a porta para hospedar.", 0, 28, 276, 800, 46)
	u.loadNetworks()
	for i := 0; i < 2; i++ {
		y := 336 + i*120
		u.label(fmt.Sprintf("CLIENTE %d · nome", i+1), 28, y, 270, 22)
		u.add(mpName1+i, "EDIT", u.settings.Names[i], 0x00810080, 28, y+28, 270, 32)
		sendMessage.Call(u.controls[mpName1+i], 0xc5, 48, 0)
		u.add(mpRename1+i, "BUTTON", "Mudar nome", 0x10000, 312, y+28, 130, 34)
		label := "Conectar / abrir jogo"
		if i == 1 {
			label = "Conectar / abrir 2º jogo"
		}
		u.add(mpJoin1+i, "BUTTON", label, 0x10000, 458, y+28, 210, 34)
		u.add(mpLeave1+i, "BUTTON", "Desconectar rede", 0x10000, 682, y+28, 148, 34)
		u.add(mpStatus1+i, "STATIC", "Desconectado", 0, 28, y+73, 800, 44)
	}
	u.label("FOV vertical efetivo (fallback)", 28, 610, 280, 22)
	u.add(mpFOV, "EDIT", fmt.Sprintf("%.1f", u.settings.FOV), 0x00810080, 308, 606, 85, 32)
	u.add(mpSetFOV, "BUTTON", "Aplicar FOV", 0x10000, 410, 606, 130, 34)
	if native {
		enableWindow.Call(u.controls[mpFOV], 0)
		enableWindow.Call(u.controls[mpSetFOV], 0)
	}
	u.add(mpSpawn, "BUTTON", "Reunir em Hateno", 0x10000, 560, 606, 270, 34)
	if native {
		u.label("FOV não é usado pelo ator nativo. Este teste não usa o boneco externo.\r\nFechar este menu mantém a conexão. Fechar o launcher encerra a rede.", 28, 654, 800, 48)
	} else {
		u.label("Usado se a leitura automática falhar. Padrão: 40°. Desative HDR para o teste.\r\nFechar este menu mantém a conexão. Fechar o launcher encerra a rede.", 28, 654, 800, 48)
	}
	u.layout()
	u.refresh()
}
func (u *multiplayerUI) set(id int, s string) {
	setText.Call(u.controls[id], uintptr(unsafe.Pointer(wide(s))))
}
func (u *multiplayerUI) say(i int, s string) {
	u.set(mpStatus1+i, s)
	core.AppendLog(filepath.Join(u.a.root, "logs", fmt.Sprintf("multiplayer-%d.log", i+1)), s)
}
func (u *multiplayerUI) post(f func()) {
	u.mu.Lock()
	if u.closed {
		u.mu.Unlock()
		return
	}
	wake := len(u.queue) == 0
	u.queue = append(u.queue, f)
	u.mu.Unlock()
	if wake {
		postMessage.Call(u.window, wmMP, 0, 0)
	}
}
func (u *multiplayerUI) drain() {
	u.mu.Lock()
	q := u.queue
	u.queue = nil
	u.mu.Unlock()
	for _, f := range q {
		f()
	}
}
func (u *multiplayerUI) save() {
	u.settings.IP = textOf(u.controls[mpIP])
	if index, _, _ := sendMessage.Call(u.controls[mpBind], 0x147, 0, 0); index < uintptr(len(u.binds)) {
		u.settings.BindIP = u.binds[index]
	}
	u.settings.Port, _ = strconv.Atoi(textOf(u.controls[mpPort]))
	for i := 0; i < 2; i++ {
		u.settings.Names[i] = textOf(u.controls[mpName1+i])
	}
	if v, e := strconv.ParseFloat(textOf(u.controls[mpFOV]), 64); e == nil && v >= 10 && v <= 150 {
		u.settings.FOV = v
	}
	b, _ := json.MarshalIndent(u.settings, "", "  ")
	if e := os.WriteFile(filepath.Join(u.a.root, "multiplayer.json"), b, 0600); e != nil {
		u.set(mpHostStatus, e.Error())
	}
}
func (u *multiplayerUI) refresh() {
	hostInput := uintptr(0)
	if u.server == nil && !u.hostStopping {
		hostInput = 1
	}
	enableWindow.Call(u.controls[mpPort], hostInput)
	enableWindow.Call(u.controls[mpBind], hostInput)
	for i := range u.slots {
		s := &u.slots[i]
		enabled := uintptr(1)
		if s.pending || s.bridge != nil {
			enabled = 0
		}
		enableWindow.Call(u.controls[mpJoin1+i], enabled)
		leave := uintptr(0)
		if s.pending || s.bridge != nil {
			leave = 1
		}
		enableWindow.Call(u.controls[mpLeave1+i], leave)
	}
	u.a.refresh()
}
func (u *multiplayerUI) host() {
	u.save()
	if u.hostStopping {
		return
	}
	if u.server != nil {
		u.hostStopping = true
		enableWindow.Call(u.controls[mpHost], 0)
		s := u.server
		u.server = nil
		u.set(mpHost, "Hospedar servidor")
		u.set(mpHostStatus, "Servidor encerrando…")
		go func() {
			s.Close()
			u.post(func() {
				u.hostStopping = false
				enableWindow.Call(u.controls[mpHost], 1)
				u.set(mpHostStatus, "Servidor local parado")
				u.refresh()
			})
		}()
		return
	}
	address, err := mp.BindAddress(u.settings.BindIP, strconv.Itoa(u.settings.Port))
	if err != nil {
		u.set(mpHostStatus, err.Error())
		return
	}
	s, err := mp.Listen(address, mp.ServerConfig{Spawn: mp.Vec3{X: 3437.5, Y: 235.9, Z: 2101.5}, Log: func(s string) { core.AppendLog(filepath.Join(u.a.root, "logs", "servidor.log"), s) }})
	if err != nil {
		u.set(mpHostStatus, "Não foi possível hospedar: "+err.Error())
		return
	}
	u.server = s
	u.set(mpHost, "Parar servidor")
	connectIP := u.settings.BindIP
	if connectIP == "0.0.0.0" {
		connectIP = u.preferredIP()
	}
	u.set(mpIP, connectIP)
	u.settings.IP = connectIP
	u.set(mpHostStatus, fmt.Sprintf("Hospedando em %s · convide usando %s:%d. Se usar Radmin, selecione o IP do adaptador Radmin.", address, connectIP, u.settings.Port))
	u.refresh()
}
func (u *multiplayerUI) leave(i int) {
	s := &u.slots[i]
	s.generation++
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if s.bridge != nil {
		b := s.bridge
		s.bridge = nil
		go b.Close()
	}
	if s.overlay != 0 {
		showWindow.Call(s.overlay, 0)
	}
	u.say(i, "Rede desconectada. O jogo continua aberto; você pode reconectar.")
	u.refresh()
}
func (u *multiplayerUI) join(i int) {
	s := &u.slots[i]
	if s.pending || s.bridge != nil {
		return
	}
	if u.a.session.Busy() || u.a.installing || u.a.preparing {
		u.say(i, "Feche a sessão offline e aguarde a preparação terminar.")
		return
	}
	u.save()
	address, err := mp.Address(u.settings.IP, strconv.Itoa(u.settings.Port))
	if err != nil {
		u.say(i, err.Error())
		return
	}
	name, err := mp.NormalizeName(u.settings.Names[i])
	if err != nil {
		u.say(i, err.Error())
		return
	}
	if !u.a.save() {
		return
	}
	cfg := u.a.config
	s.generation++
	gen := s.generation
	s.pending = true
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	running := s.session.Busy()
	port := s.port
	if !running {
		for {
			port, err = core.LocalControlPort()
			if err != nil {
				u.say(i, err.Error())
				s.pending = false
				u.refresh()
				return
			}
			if port != u.slots[1-i].port {
				break
			}
		}
		s.port = port
	}
	u.say(i, "Conectando ao servidor…")
	u.refresh()
	go func() {
		var inst core.MultiplayerInstance
		var e error
		if !running {
			u.post(func() {
				if s.generation == gen {
					u.say(i, "Preparando dados e save deste cliente…")
				}
			})
			lastProgress := time.Time{}
			lastStage := ""
			inst, e = u.preparer.Prepare(ctx, cfg, i, port, os.Environ(), func(p core.Progress) {
				now := time.Now()
				if p.Stage == lastStage && now.Sub(lastProgress) < 150*time.Millisecond {
					return
				}
				lastProgress = now
				lastStage = p.Stage
				label := p.Stage
				if p.Total > 0 {
					label += fmt.Sprintf(" · %d%%", 100*p.Done/p.Total)
				}
				u.post(func() {
					if s.generation == gen {
						u.say(i, label)
					}
				})
			})
		}
		if e != nil || ctx.Err() != nil {
			u.post(func() {
				s.pending = false
				if s.generation == gen {
					cancel()
					if e != nil {
						u.say(i, "Falha na preparação: "+e.Error())
					}
				}
				u.refresh()
			})
			return
		}
		u.post(func() {
			if s.generation == gen {
				u.say(i, "Conectando a "+address+"…")
			}
		})
		c, e := mp.Connect(ctx, address, name, nil)
		if e == nil && !running && ctx.Err() == nil {
			e = s.session.Start(inst.Spec, filepath.Join(u.a.root, "logs", fmt.Sprintf("multiplayer-processo-%d.log", i+1)), func(ev core.Event) {
				u.post(func() {
					if ev.Kind == "started" || ev.Kind == "restarted" {
						s.pid = ev.PID
						s.game = 0
					}
					if ev.Kind == "ended" || ev.Kind == "failed" {
						u.leave(i)
						u.say(i, ev.Message)
					}
					u.refresh()
				})
			})
		}
		u.post(func() {
			s.pending = false
			if s.generation != gen || ctx.Err() != nil {
				if c != nil {
					go c.Close()
				}
				u.refresh()
				return
			}
			if e != nil {
				cancel()
				if c != nil {
					go c.Close()
					u.say(i, "Falha ao abrir o jogo: "+e.Error())
				} else {
					u.say(i, mp.NetworkError(address, e))
				}
				u.refresh()
				return
			}
			if !c.Connected() {
				cancel()
				u.say(i, c.Reason())
				u.refresh()
				return
			}
			s.bridge = mp.NewBridge(ctx, c, &core.ControlStream{Port: port}, u.settings.FOV, func(status string) {
				u.post(func() {
					if s.generation == gen {
						u.say(i, status)
					}
				})
			})
			bridge := s.bridge
			go func() {
				<-bridge.Done()
				u.post(func() {
					if s.generation == gen && s.bridge == bridge {
						s.bridge = nil
						cancel()
						u.say(i, c.Reason())
						u.refresh()
					}
				})
			}()
			u.refresh()
		})
	}()
}
func (u *multiplayerUI) shutdown() {
	u.mu.Lock()
	if u.closed {
		u.mu.Unlock()
		return
	}
	u.closed = true
	u.mu.Unlock()
	u.save()
	if u.bodyFont != 0 {
		deleteObject.Call(u.bodyFont)
		u.bodyFont = 0
	}
	for i := range u.slots {
		s := &u.slots[i]
		if s.cancel != nil {
			s.cancel()
		}
		if s.bridge != nil {
			go s.bridge.Close()
		}
		if s.overlay != 0 {
			destroyWindow.Call(s.overlay)
		}
	}
	if u.server != nil {
		go u.server.Close()
	}
}
func multiplayerProc(h uintptr, m uint32, w, l uintptr) uintptr {
	a := current
	if a == nil || a.multiplayer == nil {
		return defaultProc(h, m, w, l)
	}
	u := a.multiplayer
	switch m {
	case 0x14:
		return 1
	case wmPaint:
		var ps paintStruct
		dc, _, _ := beginPaint.Call(h, uintptr(unsafe.Pointer(&ps)))
		var r rect
		getClientRect.Call(h, uintptr(unsafe.Pointer(&r)))
		fillRect.Call(dc, uintptr(unsafe.Pointer(&r)), a.bgBrush)
		endPaint.Call(h, uintptr(unsafe.Pointer(&ps)))
		return 0
	case 0x2e0:
		u.dpi = float64(w&0xffff) / 96
		u.updateFont()
		suggested := (*rect)(unsafe.Pointer(l))
		setWindowPos.Call(h, 0, uintptr(suggested.Left), uintptr(suggested.Top), uintptr(suggested.Right-suggested.Left), uintptr(suggested.Bottom-suggested.Top), 4|16)
		u.layout()
		return 0
	case wmSize:
		if len(u.placements) > 0 {
			u.layout()
		}
		return 0
	case 0x115:
		u.scrollBy(w)
		return 0
	case 0x20a:
		u.scroll -= int32(int16(w>>16)) * u.px(45) / 120
		u.layout()
		return 0
	case wmMP:
		u.drain()
		return 0
	case wmClose:
		u.save()
		showWindow.Call(h, 0)
		return 0
	case wmColorEdit, wmColorStatic:
		setTextColor.Call(w, colorText)
		setBkColor.Call(w, colorBG)
		return a.bgBrush
	case 0x113:
		u.tick()
		return 0
	case wmCommand:
		if w>>16 != 0 {
			return 0
		}
		id := int(w & 0xffff)
		switch id {
		case mpNetworks:
			u.loadNetworks()
		case mpLogs:
			u.a.logs()
		case mpSpawn:
			for i := range u.slots {
				if b := u.slots[i].bridge; b != nil {
					b.RequestSpawn()
				}
			}
		case mpHost:
			u.host()
		case mpJoin1, mpJoin2:
			u.join(id - mpJoin1)
		case mpLeave1, mpLeave2:
			u.leave(id - mpLeave1)
		case mpRename1, mpRename2:
			i := id - mpRename1
			u.save()
			if b := u.slots[i].bridge; b != nil {
				name := u.settings.Names[i]
				go func() {
					e := b.Client.Rename(name)
					u.post(func() {
						if e != nil {
							u.say(i, e.Error())
						} else {
							u.say(i, "Nome atualizado")
						}
					})
				}()
			}
		case mpSetFOV:
			u.save()
			for i := range u.slots {
				if b := u.slots[i].bridge; b != nil {
					b.SetFOV(u.settings.FOV)
				}
			}
		case mpArrange:
			u.arrange()
		}
		return 0
	}
	return defaultProc(h, m, w, l)
}

func (u *multiplayerUI) preferredIP() string {
	for _, a := range u.networks {
		if strings.Contains(strings.ToLower(a.Interface), "radmin") {
			return a.IP
		}
	}
	if len(u.networks) > 0 {
		return u.networks[0].IP
	}
	return "127.0.0.1"
}
func (u *multiplayerUI) loadNetworks() {
	addresses, e := mp.LocalAddresses()
	if e != nil {
		u.set(mpHostStatus, "Falha ao enumerar IPs: "+e.Error())
		return
	}
	u.networks = addresses
	sendMessage.Call(u.controls[mpBind], 0x14b, 0, 0)
	u.binds = []string{"0.0.0.0"}
	sendMessage.Call(u.controls[mpBind], 0x143, 0, uintptr(unsafe.Pointer(wide("Todas as interfaces · LAN e VPN/Radmin"))))
	selected := uintptr(0)
	for _, a := range addresses {
		u.binds = append(u.binds, a.IP)
		sendMessage.Call(u.controls[mpBind], 0x143, 0, uintptr(unsafe.Pointer(wide(a.IP+" · "+a.Interface))))
		if a.IP == u.settings.BindIP {
			selected = uintptr(len(u.binds) - 1)
		}
	}
	sendMessage.Call(u.controls[mpBind], 0x14e, selected, 0)
}
func (u *multiplayerUI) layout() {
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
	maximum := u.px(720) - r.Bottom
	if maximum < 0 {
		maximum = 0
	}
	if u.scroll < 0 {
		u.scroll = 0
	}
	if u.scroll > maximum {
		u.scroll = maximum
	}
	si := scrollInfo{Size: uint32(unsafe.Sizeof(scrollInfo{})), Mask: 7, Max: u.px(720) - 1, Page: uint32(r.Bottom), Pos: u.scroll}
	setScrollInfo.Call(u.window, 1, uintptr(unsafe.Pointer(&si)), 0)
	for id, p := range u.placements {
		moveWindow.Call(u.controls[id], uintptr(u.px(int(p.Left))), uintptr(u.px(int(p.Top))-u.scroll), uintptr(u.px(int(p.Right))), uintptr(u.px(int(p.Bottom))), 0)
	}
	user32.NewProc("RedrawWindow").Call(u.window, 0, 0, 0x1|0x80)
}
func (u *multiplayerUI) scrollBy(w uintptr) {
	si := scrollInfo{Size: uint32(unsafe.Sizeof(scrollInfo{})), Mask: 16}
	getScrollInfo.Call(u.window, 1, uintptr(unsafe.Pointer(&si)))
	switch w & 0xffff {
	case 0:
		u.scroll -= u.px(35)
	case 1:
		u.scroll += u.px(35)
	case 2:
		u.scroll -= u.px(250)
	case 3:
		u.scroll += u.px(250)
	case 4, 5:
		u.scroll = si.Track
	}
	u.layout()
}

func (u *multiplayerUI) px(n int) int32 { return int32(float64(n) * u.dpi) }
func (u *multiplayerUI) updateFont() {
	height := u.px(14)
	font, _, _ := createFont.Call(uintptr(-height), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(wide("Segoe UI"))))
	if font == 0 {
		return
	}
	old := u.bodyFont
	u.bodyFont = font
	for _, h := range u.controls {
		sendMessage.Call(h, 0x30, font, 1)
	}
	if old != 0 {
		deleteObject.Call(old)
	}
}
