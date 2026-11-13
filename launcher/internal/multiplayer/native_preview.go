package multiplayer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// This adapter only carries native presence over protocol 1. The combat/world
// authority protocol remains gated until its real game hooks exist.
type nativePreview struct {
	session    string
	sequence   uint64
	lastStatus time.Time
	state      string
}

func (b *Bridge) openNative(ctx context.Context) (*nativePreview, error) {
	var caps struct {
		Runtime       string `json:"runtime"`
		GameVersion   string `json:"game_version"`
		PreviewABI    int    `json:"preview_abi"`
		NativePreview bool   `json:"native_preview"`
	}
	if err := b.command(ctx, "hm_caps", &caps); err != nil {
		return nil, fmt.Errorf("o runtime não possui o adaptador nativo: %w", err)
	}
	if caps.Runtime != "SavageWind-HyruleM" || caps.GameVersion != "1.9.0" || caps.PreviewABI != 1 || !caps.NativePreview {
		return nil, fmt.Errorf("runtime incompatível ou assinaturas nativas incompletas")
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	n := &nativePreview{session: hex.EncodeToString(nonce[:])}
	if err := b.command(ctx, "hm_native_open "+n.session, nil); err != nil {
		return nil, err
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
		if err := b.clearNative(ctx, n); err != nil {
			return err
		}
	} else {
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
		State   string `json:"state"`
		Error   string `json:"error"`
		Active  bool   `json:"active"`
		Applied uint64 `json:"applied_sequence"`
	}
	if err := b.command(ctx, "hm_native_status", &status); err != nil {
		return err
	}
	if status.State == "error" {
		return fmt.Errorf("ator remoto: %s", status.Error)
	}
	if status.State != n.state {
		n.state = status.State
		if status.Active {
			b.say("Ator remoto criado no jogo · teste nativo de presença; combate e monstros ainda indisponíveis")
		} else if status.State == "spawning" {
			b.say("O jogo está criando o Link remoto…")
		}
	}
	return nil
}

func (b *Bridge) NativePreview() bool { return b.native }
