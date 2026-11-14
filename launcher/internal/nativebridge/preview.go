package nativebridge

import (
	"encoding/json"
	"fmt"
	core "hyrulelauncher/internal/launcher"
)

// Preview readiness is separate from complete Gen1 combat/AI capabilities.
// A running older adapter must be diagnosed before interpreting actor state.
type PreviewDiagnostic struct {
	ExpectedABI      int    `json:"expected_abi"`
	ActualABI        int    `json:"actual_abi"`
	Compatible       bool   `json:"compatible"`
	Status           string `json:"status"`
	Message          string `json:"message"`
	GameTicks        uint64 `json:"game_ticks"`
	RemoteActive     bool   `json:"remote_active"`
	ResourceFallback bool   `json:"resource_fallback"`
	FirstError       string `json:"first_error,omitempty"`
}

func DiagnosePreview(queries []ProbeCommand) PreviewDiagnostic {
	p := PreviewDiagnostic{ExpectedABI: core.NativePreviewABI, Status: "control_unavailable", Message: "Controle indisponível. Abra o cliente pelo launcher novo e mantenha o jogo aberto durante o diagnóstico."}
	byName := map[string]ProbeCommand{}
	for _, q := range queries {
		byName[q.Command] = q
	}
	q, ok := byName["hm_caps"]
	if !ok {
		return p
	}
	var caps struct {
		Runtime string `json:"runtime"`
		ABI     int    `json:"preview_abi"`
		Enabled bool   `json:"native_preview"`
		Health  bool   `json:"shared_enemy_health"`
	}
	if q.Error != "" || json.Unmarshal(q.Data, &caps) != nil {
		p.Status, p.Message = "adapter_missing", "O jogo não responde ao adaptador HyruleM. Feche os jogos e abra Iniciar-Nativo.cmd do pacote novo."
		return p
	}
	p.ActualABI = caps.ABI
	if caps.ABI != p.ExpectedABI {
		p.Status = "runtime_outdated"
		p.Message = fmt.Sprintf("O processo está usando ABI %d; a prévia exige ABI %d. Atualize o runtime selecionado com Iniciar-Nativo.cmd e reinicie os dois jogos.", caps.ABI, p.ExpectedABI)
		return p
	}
	if caps.Runtime != "SavageWind-HyruleM" || !caps.Enabled {
		p.Status, p.Message = "native_disabled", "Modo nativo desativado ou assinaturas incompletas. Abra o launcher do pacote nativo completo."
		return p
	}
	if !caps.Health {
		p.Status, p.Message = "library_outdated", "As assinaturas da biblioteca estão incompletas. Reconstrua a biblioteca do jogo com o gerador instalado pelo pacote novo."
		return p
	}
	p.Compatible = true
	var s struct {
		State            string `json:"state"`
		Error            string `json:"error"`
		Ticks            uint64 `json:"game_ticks"`
		Age              int64  `json:"tick_age_ms"`
		Active           bool   `json:"active"`
		Model            bool   `json:"model_ready"`
		Animation        bool   `json:"animation_ready"`
		Paused           bool   `json:"game_paused"`
		Received         uint64 `json:"received_sequence"`
		VisualOnly       bool   `json:"visual_only"`
		ResourceFallback bool   `json:"resource_fallback"`
		FirstError       string `json:"first_error"`
	}
	q, ok = byName["hm_native_status"]
	if !ok || q.Error != "" || json.Unmarshal(q.Data, &s) != nil {
		p.Status, p.Message = "status_unavailable", "O runtime não respondeu ao estado nativo. Gere o relatório mantendo o mundo carregado."
		return p
	}
	p.GameTicks, p.RemoteActive = s.Ticks, s.Active
	p.ResourceFallback, p.FirstError = s.ResourceFallback, s.FirstError
	switch {
	case s.Error != "" || s.State == "error":
		p.Status, p.Message = "actor_error", "O jogo recusou o ator remoto: "+s.Error
	case s.Paused:
		p.Status, p.Message = "paused", "O jogo está pausado. Retorne ao mundo aberto para testar o ator remoto."
	case s.Ticks == 0 || s.Age < 0 || s.Age > 3000:
		p.Status, p.Message = "callback_stalled", "O callback não está atualizando. Reconstrua a biblioteca com o gerador novo e teste sem pausa."
	case s.Active && s.Model && s.ResourceFallback:
		p.Status, p.Message = "visual_fallback", "Contingência visual ativa, sem animação. Falha da variante principal: "+s.FirstError
	case s.Active && s.Model && s.VisualOnly:
		p.Status, p.Message = "animation_unavailable", "Modelo ativo com movimento; a animação não está pronta."
	case s.Active && s.Model && s.Animation:
		p.Status, p.Message = "active", "Ator e componentes confirmados pelo jogo. Este estado ainda não comprova visibilidade na GPU."
	case s.Active:
		p.Status, p.Message = "model_missing", "O ator não confirmou os componentes de modelo e animação."
	case s.State == "activating":
		p.Status, p.Message = "activating", "O personagem foi criado e aguarda ativação pelo jogo. Ainda não confirmou modelo ou movimento."
	case s.Received == 0:
		p.Status, p.Message = "no_native_session", "Callback ativo, sem comandos de presença. Conecte os dois clientes pelo menu nativo."
	default:
		p.Status, p.Message = s.State, "Aguardando confirmação do ator remoto: "+s.State
	}
	return p
}
