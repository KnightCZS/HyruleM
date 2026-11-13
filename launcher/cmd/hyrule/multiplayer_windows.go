//go:build windows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	core "hyrulelauncher/internal/launcher"
	mp "hyrulelauncher/internal/multiplayer"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
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
	IP    string
	Port  int
	Names [2]string
	FOV   float64
}
type multiplayerUI struct {
	a        *app
	window   uintptr
	controls map[int]uintptr
	slots    [2]mpSlot
	server   *mp.Server
	settings mpSettings
	mu       sync.Mutex
	queue    []func()
	closed   bool
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
	u := &multiplayerUI{a: a, controls: map[int]uintptr{}, settings: mpSettings{IP: "127.0.0.1", Port: 5050, Names: [2]string{"Jogador 1", "Jogador 2"}, FOV: 40}}
	if b, e := os.ReadFile(filepath.Join(a.root, "multiplayer.json")); e == nil {
		_ = json.Unmarshal(b, &u.settings)
	}
	if _, e := mp.Address(u.settings.IP, strconv.Itoa(u.settings.Port)); e != nil {
		u.settings.IP = "127.0.0.1"
		u.settings.Port = 5050
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
	u.window, _, _ = createWindow.Call(0, uintptr(unsafe.Pointer(wide("HyruleMultiplayer"))), uintptr(unsafe.Pointer(wide("HyruleM · Multiplayer experimental"))), 0x00ca0000, 100, 70, uintptr(a.px(880)), uintptr(a.px(680)), a.window, 0, a.instance, 0)
	if u.window == 0 {
		a.multiplayer = nil
		a.setStatus("Falha ao abrir multiplayer.", true)
		return
	}
	u.build()
	setTimer.Call(u.window, 3, 33, 0)
	showWindow.Call(u.window, 5)
}
func (u *multiplayerUI) add(id int, class, label string, style uintptr, x, y, w, h int) {
	a := u.a
	hwnd, _, _ := createWindow.Call(0, uintptr(unsafe.Pointer(wide(class))), uintptr(unsafe.Pointer(wide(label))), 0x50000000|style, uintptr(a.px(x)), uintptr(a.px(y)), uintptr(a.px(w)), uintptr(a.px(h)), u.window, uintptr(id), a.instance, 0)
	u.controls[id] = hwnd
	sendMessage.Call(hwnd, 0x30, a.fonts["body"], 1)
}
func (u *multiplayerUI) label(s string, x, y, w, h int) { u.add(0, "STATIC", s, 0, x, y, w, h) }
func (u *multiplayerUI) build() {
	u.label("MULTIPLAYER · PRIMEIRO PROTÓTIPO", 28, 24, 800, 28)
	u.label("Avatares visuais provisórios. Inicie no mundo aberto de Hateno.\r\nMantenha este launcher aberto e os jogos em modo janela.", 28, 62, 805, 48)
	u.label("IP do servidor", 28, 128, 240, 20)
	u.add(mpIP, "EDIT", u.settings.IP, 0x00810080, 28, 154, 290, 32)
	u.label("Porta TCP", 334, 128, 130, 20)
	u.add(mpPort, "EDIT", strconv.Itoa(u.settings.Port), 0x00812080, 334, 154, 100, 32)
	u.add(mpHost, "BUTTON", "Hospedar servidor", 0x10000, 454, 154, 180, 34)
	u.add(mpArrange, "BUTTON", "Organizar 2 janelas", 0x10000, 650, 154, 180, 34)
	u.add(mpHostStatus, "STATIC", "Servidor local parado. Hospedar não conecta o jogador automaticamente.", 0, 28, 204, 800, 42)
	for i := 0; i < 2; i++ {
		y := 258 + i*132
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
	u.label("FOV vertical efetivo (fallback)", 28, 534, 280, 22)
	u.add(mpFOV, "EDIT", fmt.Sprintf("%.1f", u.settings.FOV), 0x00810080, 308, 530, 85, 32)
	u.add(mpSetFOV, "BUTTON", "Aplicar FOV", 0x10000, 410, 530, 130, 34)
	u.label("Usado se a leitura automática falhar. Padrão: 40°. Desative HDR para o teste.\r\nFechar este menu mantém a conexão. Fechar o launcher encerra a rede.", 28, 578, 800, 48)
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
	u.queue = append(u.queue, f)
	u.mu.Unlock()
	postMessage.Call(u.window, wmMP, 0, 0)
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
	if u.server != nil {
		s := u.server
		u.server = nil
		u.set(mpHost, "Hospedar servidor")
		u.set(mpHostStatus, "Servidor encerrando…")
		go func() { s.Close(); u.post(func() { u.set(mpHostStatus, "Servidor local parado") }) }()
		return
	}
	_, err := mp.Address("0.0.0.0", strconv.Itoa(u.settings.Port))
	if err != nil {
		u.set(mpHostStatus, err.Error())
		return
	}
	s, err := mp.Listen(net.JoinHostPort("0.0.0.0", strconv.Itoa(u.settings.Port)), mp.ServerConfig{Spawn: mp.Vec3{X: 3437.5, Y: 235.9, Z: 2101.5}, Log: func(s string) { core.AppendLog(filepath.Join(u.a.root, "logs", "servidor.log"), s) }})
	if err != nil {
		u.set(mpHostStatus, "Não foi possível hospedar: "+err.Error())
		return
	}
	u.server = s
	u.set(mpHost, "Parar servidor")
	u.set(mpHostStatus, fmt.Sprintf("Servidor ativo · TCP %d · até 2 jogadores · neste PC use 127.0.0.1", u.settings.Port))
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
		c, e := mp.Connect(ctx, address, name, func(reason string) {
			u.post(func() {
				if s.generation == gen {
					u.say(i, reason)
				}
			})
		})
		var inst core.MultiplayerInstance
		if e == nil && !running {
			inst, e = core.PrepareMultiplayerInstance(cfg, i, port, os.Environ())
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
				u.say(i, e.Error())
				if c != nil {
					go c.Close()
				}
				u.refresh()
				return
			}
			if !running {
				if e = s.session.Start(inst.Spec, filepath.Join(u.a.root, "logs", fmt.Sprintf("multiplayer-processo-%d.log", i+1)), func(ev core.Event) {
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
				}); e != nil {
					go c.Close()
					cancel()
					u.say(i, e.Error())
					u.refresh()
					return
				}
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
					if s.bridge == bridge {
						s.bridge = nil
						u.refresh()
					}
				})
			}()
			u.refresh()
		})
	}()
}
func (u *multiplayerUI) shutdown() {
	u.save()
	u.mu.Lock()
	u.closed = true
	u.mu.Unlock()
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
