package multiplayer

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
)

func (b *Bridge) RequestAppleDrop() bool {
	if !b.dropReady.Load() || !b.native || !b.Client.Connected() || !b.Client.Ground().Enabled || len(b.Client.Ground().Items) >= MaxGroundItems || !b.dropBusy.CompareAndSwap(false, true) {
		return false
	}
	select {
	case b.dropRequests <- struct{}{}:
		return true
	default:
		b.dropBusy.Store(false)
		return false
	}
}
func (b *Bridge) DropBusy() bool { return b.dropBusy.Load() }
func (b *Bridge) prepareDropFromUI(n *groundSync, origin string, sequence uint64) error {
	select {
	case <-b.dropRequests:
		pose, ok := b.Client.LocalPose()
		if !ok || !validOrigin(origin) {
			b.dropBusy.Store(false)
			return fmt.Errorf("aguarde o mundo e o experimento de materiais")
		}
		if sequence > n.dropNext {
			n.dropNext = sequence
		}
		if result := b.Client.Ground().DropResult; result != nil && result.Request > n.dropNext {
			n.dropNext = result.Request
		}
		n.dropNext++
		n.dropRequest = n.dropNext
		n.dropError = ""
		n.dropSource = GroundSource{Local: n.dropRequest, Drop: true, Origin: origin, Actor: "Item_Fruit_A", Position: pose.Position.Add(Vec3{math.Sin(pose.Yaw) * 2, .4, math.Cos(pose.Yaw) * 2})}
		n.dropSource.Spawn = n.dropSource.Position
		b.say("Reservando um lugar para largar 1 maçã…")
	default:
	}
	if n.dropRequest != 0 && n.dropItem == 0 {
		return b.Client.PrepareGroundDrop(n.dropRequest, n.dropSource)
	}
	return nil
}
func (b *Bridge) finishDrop(ctx context.Context, n *groundSync, request, item uint64, result int, sourceJSON string) error {
	if n.dropRequest == 0 {
		return nil
	}
	r := b.Client.Ground().DropResult
	if r != nil && r.Request == n.dropRequest {
		switch r.Status {
		case "prepared":
			n.dropItem = r.Item
		case "committed", "cancelled", "uncertain", "rejected":
			message := "Maçã largada na sessão"
			if r.Status == "cancelled" {
				message = dropFailureMessage(n.dropError)
			}
			if r.Status == "rejected" {
				message = "Drop recusado. Confira se a sessão tem espaço e se o jogador está no mundo."
			}
			if r.Status == "uncertain" {
				message = "Drop com resultado incerto; nenhuma nova concessão automática será feita."
			}
			b.say(message)
			n.dropRequest = 0
			n.dropItem = 0
			b.dropBusy.Store(false)
			return nil
		}
	}
	if request == n.dropRequest && item == n.dropItem && result != 0 {
		finish := GroundDropFinish{Request: request, Item: item, Result: result}
		if result == 1 {
			if err := json.Unmarshal([]byte(sourceJSON), &finish.Source); err != nil {
				return err
			}
		}
		return b.Client.FinishGroundDrop(finish)
	}
	return nil
}

func (b *Bridge) CanDropApple() bool {
	return b.native && b.dropReady.Load() && b.Client.Connected() && b.Client.Ground().Enabled && len(b.Client.Ground().Items) < MaxGroundItems && !b.DropBusy()
}

func dropFailureMessage(reason string) string {
	switch reason {
	case "apple-missing":
		return "Você não tem maçãs no inventário deste cliente."
	case "player-moved-or-outside-zone":
		return "Descarte cancelado. Fique perto do ponto onde soltou o item."
	case "inventory-layout-invalid", "inventory-unavailable":
		return "O runtime não conseguiu conferir o inventário; veja o diagnóstico."
	case "inventory-remove-not-applied":
		return "O jogo não confirmou a retirada da maçã; o descarte foi cancelado."
	case "actor-create-rejected", "create-handle-failed", "wake-failed", "reference-failed", "actor-reference-or-activation-failed", "activation-timeout", "spawn-position-not-confirmed":
		return "O jogo não confirmou a criação do objeto; veja o motivo no diagnóstico."
	default:
		return "Não foi possível largar a maçã; confira o inventário e o diagnóstico."
	}
}
