package launcher

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// The runtime accepts one text line and returns one JSON line on a loopback TCP
// endpoint. The UI exposes a fixed action list, never arbitrary memory commands.
type ControlReply struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error json.RawMessage `json:"error"`
}
type ControlClient struct {
	Port    int
	Timeout time.Duration
}

func LocalControlPort() (int, error) {
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err = l.Close(); err != nil {
		return 0, err
	}
	return port, nil
}
func (c ControlClient) Command(ctx context.Context, command string) (ControlReply, error) {
	var reply ControlReply
	if c.Port < 1024 || c.Port > 65535 || command == "" || len(command) > 256 || strings.ContainsAny(command, "\r\n\x00") {
		return reply, fmt.Errorf("comando local inválido")
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(c.Port)))
	if err != nil {
		return reply, fmt.Errorf("controle do sandbox indisponível; aguarde o jogo carregar e mantenha a janela dele renderizando: %w", err)
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	if err = conn.SetDeadline(deadline); err != nil {
		return reply, err
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if _, err = fmt.Fprintf(conn, "%s\n", command); err != nil {
		return reply, err
	}
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), 128*1024)
	if !scanner.Scan() {
		if err = scanner.Err(); err != nil {
			return reply, err
		}
		return reply, fmt.Errorf("controle encerrou sem responder")
	}
	if err = json.Unmarshal(scanner.Bytes(), &reply); err != nil {
		return reply, fmt.Errorf("resposta inválida do controle: %w", err)
	}
	if !reply.OK {
		var message string
		if json.Unmarshal(reply.Error, &message) != nil {
			message = string(reply.Error)
		}
		return reply, fmt.Errorf("Savage Wind: %s", message)
	}
	if len(reply.Data) == 0 {
		return reply, fmt.Errorf("resposta do controle sem dados")
	}
	return reply, nil
}

type SandboxAction struct {
	Name     string
	Commands []string
}

var SandboxActions = []SandboxAction{
	{"Consultar posição", []string{"pos"}},
	{"Horário: 12:00", []string{"time set 12"}},
	{"Horário: 22:00", []string{"time set 22"}},
	{"Congelar horário", []string{"time freeze"}},
	{"Liberar horário", []string{"time unfreeze"}},
	{"Clima: céu limpo", []string{"weather clear"}},
	{"Clima: chuva", []string{"weather rain"}},
	{"Clima: natural", []string{"weather game"}},
	{"Definir 1.000 rupias", []string{"rupees 1000"}},
	{"Ativar voo", []string{"fly on"}},
	{"Desativar voo", []string{"fly off"}},
	{"Kit: espada, escudo e maçãs", []string{"give Weapon_Sword_001 1", "give Weapon_Shield_001 1", "give Item_Fruit_A 10"}},
}

func (c ControlClient) Apply(ctx context.Context, index int) (string, error) {
	if index < 0 || index >= len(SandboxActions) {
		return "", fmt.Errorf("ação inválida")
	}
	a := SandboxActions[index]
	// Only act with a loaded player; the title screen also presents frames.
	if _, err := c.Command(ctx, "pos"); err != nil {
		return "", err
	}
	var reply ControlReply
	for i, cmd := range a.Commands {
		var err error
		reply, err = c.Command(ctx, cmd)
		if err != nil {
			return "", fmt.Errorf("%s (%d/%d): %w", a.Name, i+1, len(a.Commands), err)
		}
	}
	if index == 0 {
		var p struct {
			X, Y, Z *float64
			Place   string
		}
		if err := json.Unmarshal(reply.Data, &p); err != nil || p.X == nil || p.Y == nil || p.Z == nil {
			return "", fmt.Errorf("posição não reconhecida na resposta do jogo")
		}
		return fmt.Sprintf("Posição: X %.1f · Y %.1f · Z %.1f", *p.X, *p.Y, *p.Z), nil
	}
	if strings.HasPrefix(a.Commands[0], "give ") {
		return "Kit solicitado. Confira o inventário e os logs; armas exigem espaço livre.", nil
	}
	return a.Name + ": solicitação aceita pelo jogo.", nil
}

// Teleport is asynchronous in the runtime. Success is reported only after the
// runtime finishes loading the surrounding map, never just on queue acceptance.
func (c ControlClient) Teleport(ctx context.Context, place string, notify func(string)) (string, error) {
	p, err := FindPlace(place)
	if err != nil {
		return "", err
	}
	if _, err = c.Command(ctx, "pos"); err != nil {
		return "", err
	}
	r, err := c.Command(ctx, fmt.Sprintf("goto %.4f %.4f %.4f", p.X, p.Y, p.Z))
	if err != nil {
		return "", err
	}
	var first struct {
		State string `json:"state"`
	}
	if err = json.Unmarshal(r.Data, &first); err != nil {
		return "", err
	}
	if first.State == "already_there" || first.State == "done" {
		return "Destino alcançado: " + p.Name, nil
	}
	if first.State == "failed" {
		return "", fmt.Errorf("o jogo não conseguiu iniciar o teleporte")
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("teleporte ainda sem conclusão confirmada; consulte a posição antes de repetir: %w", ctx.Err())
		case <-ticker.C:
		}
		r, e := c.Command(ctx, "teleport status")
		if e != nil {
			return "", e
		}
		var status map[string]any
		if e = json.Unmarshal(r.Data, &status); e != nil {
			return "", e
		}
		// Current runtime reports its phase as state; tolerate the documented name
		// status used by earlier builds, but never infer success from an unknown key.
		state, _ := status["state"].(string)
		if state == "" {
			state, _ = status["status"].(string)
		}
		if state == "" {
			state, _ = status["stage"].(string)
		}
		if state == "done" {
			return "Teleporte concluído: " + p.Name, nil
		}
		if state == "failed" {
			return "", fmt.Errorf("o teleporte falhou: %s", r.Data)
		}
		if state == "" {
			return "", fmt.Errorf("formato de teleporte não reconhecido: %s", r.Data)
		}
		if notify != nil {
			notify("Teleporte: " + state + "…")
		}
	}
}
