package multiplayer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	core "hyrulelauncher/internal/launcher"
	"time"
)

// Native presence and nearby map-enemy health use protocol 3. Full Gen1 combat
// and AI authority remain separately gated until their native hooks exist.
type nativePreview struct {
	session      string
	sequence     uint64
	lastStatus   time.Time
	state        string
	waitingSince time.Time
	lastWorld    time.Time
}

func (b *Bridge) openNative(ctx context.Context) (*nativePreview, error) {
	var caps struct {
		Runtime       string `json:"runtime"`
		GameVersion   string `json:"game_version"`
		PreviewABI    int    `json:"preview_abi"`
		NativePreview bool   `json:"native_preview"`
		SharedHealth  bool   `json:"shared_enemy_health"`
	}
	if err := b.command(ctx, "hm_caps", &caps); err != nil {
		return nil, fmt.Errorf("o runtime não possui o adaptador nativo: %w", err)
	}
	if caps.PreviewABI != core.NativePreviewABI {
		return nil, fmt.Errorf("o jogo está usando o runtime ABI %d; este launcher exige ABI %d. Feche os jogos e abra Iniciar-Nativo.cmd do pacote novo", caps.PreviewABI, core.NativePreviewABI)
	}
	if caps.Runtime != "SavageWind-HyruleM" || caps.GameVersion != "1.9.0" || !caps.NativePreview {
		return nil, fmt.Errorf("runtime incompatível, modo nativo desativado ou assinaturas incompletas")
	}
	if !caps.SharedHealth {
		return nil, fmt.Errorf("o adaptador de mundo não foi carregado; reconstrua a biblioteca com o novo runtime")
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	n := &nativePreview{session: hex.EncodeToString(nonce[:])}
	if err := b.command(ctx, "hm_native_open "+n.session, nil); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(6 * time.Second)
	for {
		var status struct {
			Ticks uint64 `json:"game_ticks"`
			Age   int64  `json:"tick_age_ms"`
		}
		if err := b.command(ctx, "hm_native_status", &status); err != nil {
			return nil, err
		}
		if status.Ticks > 0 && status.Age >= 0 && status.Age < 2000 {
			break
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("o callback nativo não está executando; reconstrua a biblioteca do jogo com este runtime e mantenha o mundo sem pausa")
		}
		if !wait(ctx, 100*time.Millisecond) {
			return nil, ctx.Err()
		}
	}
	return n, nil
}
func (b *Bridge) clearNative(ctx context.Context, n *nativePreview) error {
	n.sequence++
	return b.command(ctx, fmt.Sprintf("hm_native_clear %s %d", n.session, n.sequence), nil)
}
func nativePoseCommand(session string, seq uint64, pose Pose) (string, error) {
	if len(session) != 16 || seq == 0 || !pose.Valid() || !pose.Ready {
		return "", fmt.Errorf("pose nativa inválida")
	}
	if raw, e := hex.DecodeString(session); e != nil || len(raw) != 8 {
		return "", fmt.Errorf("sessão nativa inválida")
	}
	animation := "wait"
	if pose.Velocity.Len() > .2 {
		animation = "move"
	}
	return fmt.Sprintf("hm_native_remote %s %d %.4f %.4f %.4f %.6f %s", session, seq, pose.Position.X, pose.Position.Y, pose.Position.Z, pose.Yaw, animation), nil
}
func (b *Bridge) updateNative(ctx context.Context, n *nativePreview, now time.Time) error {
	remotes := b.Client.Remote(now)
	if len(remotes) == 0 {
		n.waitingSince = time.Time{}
		if err := b.clearNative(ctx, n); err != nil {
			return err
		}
	} else {
		if n.waitingSince.IsZero() {
			n.waitingSince = now
		}
		n.sequence++
		command, err := nativePoseCommand(n.session, n.sequence, remotes[0].Pose)
		if err != nil {
			return err
		}
		if err = b.command(ctx, command, nil); err != nil {
			return err
		}
	}
	if now.Sub(n.lastStatus) < time.Second {
		return nil
	}
	n.lastStatus = now
	var status struct {
		State     string `json:"state"`
		Error     string `json:"error"`
		Active    bool   `json:"active"`
		Applied   uint64 `json:"applied_sequence"`
		Model     bool   `json:"model_ready"`
		Animation bool   `json:"animation_ready"`
		TickAge   int64  `json:"tick_age_ms"`
		Paused    bool   `json:"game_paused"`
	}
	if err := b.command(ctx, "hm_native_status", &status); err != nil {
		return err
	}
	if status.State == "error" {
		return fmt.Errorf("ator remoto: %s", status.Error)
	}
	if status.Paused {
		n.waitingSince = now
		return nil
	}
	if status.TickAge < 0 || status.TickAge > 3000 {
		return fmt.Errorf("a atualização nativa parou; estado %s", status.State)
	}
	if len(remotes) > 0 && !status.Active && now.Sub(n.waitingSince) > 18*time.Second {
		return fmt.Errorf("o jogo não confirmou o ator remoto (estado %s); a conexão foi interrompida para evitar presença invisível", status.State)
	}
	if status.Active && (!status.Model || !status.Animation) {
		return fmt.Errorf("o ator remoto não confirmou modelo e animação")
	}
	if status.State != n.state {
		n.state = status.State
		if status.Active {
			b.say("Ator remoto criado · modelo registrado · sincronização de vida dos monstros em teste")
		} else if status.State == "spawning" {
			b.say("O jogo está criando o Link remoto…")
		}
	}
	return nil
}

// Send health observations separately from remote-player poses. Only native
// enemies with matching map hash, actor and original placement can be changed.
func (b *Bridge) updateWorldHealth(ctx context.Context, n *nativePreview, now time.Time) error {
	if now.Sub(n.lastWorld) < 250*time.Millisecond {
		return nil
	}
	n.lastWorld = now
	var raw struct {
		Enemies string `json:"enemies_json"`
	}
	if err := b.command(ctx, "hm_world_status", &raw); err != nil {
		return err
	}
	var local []EnemyHealth
	if err := json.Unmarshal([]byte(raw.Enemies), &local); err != nil {
		return fmt.Errorf("snapshot nativo de monstros inválido: %w", err)
	}
	if err := b.Client.PublishEnemyHealth(local); err != nil {
		return err
	}
	byKey := map[string]EnemyHealth{}
	for _, e := range b.Client.SharedEnemyHealth() {
		byKey[e.key()] = e
	}
	for _, e := range local {
		shared, ok := byKey[e.key()]
		if !ok || shared.Health >= e.Health {
			continue
		}
		cmd := fmt.Sprintf("hm_enemy_health %s %d %s %.3f %.3f %.3f %d", n.session, e.Hash, e.Actor, e.Spawn.X, e.Spawn.Y, e.Spawn.Z, shared.Health)
		if err := b.command(ctx, cmd, nil); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bridge) NativePreview() bool { return b.native }
