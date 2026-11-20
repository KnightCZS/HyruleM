package nativebridge

import (
	"encoding/json"
	"fmt"
	core "hyrulelauncher/internal/launcher"
)

// Preview readiness is separate from complete Gen1 combat/AI capabilities.
// A running older adapter must be diagnosed before interpreting actor state.
type PreviewDiagnostic struct {
	WorldRole                int    `json:"world_role"`
	WorldEpoch               uint64 `json:"world_epoch"`
	WorldCaptured            int    `json:"world_captured"`
	WorldMatched             int    `json:"world_matched"`
	WorldMissing             int    `json:"world_missing"`
	WorldApplied             int    `json:"world_applied"`
	WorldAISkipped           int    `json:"world_ai_skipped"`
	WorldAnimationsApplied   int    `json:"world_animations_applied"`
	VisibilityCaptured       int    `json:"visibility_captured"`
	VisibilityApplied        int    `json:"visibility_applied"`
	EffectPropertiesCaptured int    `json:"effect_properties_captured"`
	EffectPropertiesApplied  int    `json:"effect_properties_applied"`
	ArmorReferences          int    `json:"armor_references"`
	ArmorHeadBones           int    `json:"armor_head_bones"`
	ArmorUpperBones          int    `json:"armor_upper_bones"`
	ArmorLowerBones          int    `json:"armor_lower_bones"`
	EffectsSourceUsers       int    `json:"effects_source_users"`
	EffectsCaptured          int    `json:"effects_captured"`
	EffectsApplied           int    `json:"effects_applied"`
	EffectsStopped           int    `json:"effects_stopped"`
	EffectsMissingUser       int    `json:"effects_missing_user"`
	EffectLastKey            string `json:"effect_last_key,omitempty"`
	ExpectedABI              int    `json:"expected_abi"`
	ActualABI                int    `json:"actual_abi"`
	Compatible               bool   `json:"compatible"`
	Status                   string `json:"status"`
	Message                  string `json:"message"`
	GameTicks                uint64 `json:"game_ticks"`
	RemoteActive             bool   `json:"remote_active"`
	ResourceFallback         bool   `json:"resource_fallback"`
	FirstError               string `json:"first_error,omitempty"`
	SkeletonCaptured         int    `json:"skeleton_captured"`
	SkeletonApplied          int    `json:"skeleton_applied"`
	SkeletonReceived         int    `json:"skeleton_received"`
	BodyPoseApplied          int    `json:"body_pose_applied"`
	PoseCaptureStatus        string `json:"pose_capture_status,omitempty"`
	ArmorSource              string `json:"armor_source,omitempty"`
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
		Barrier bool   `json:"actor_job_barrier"`
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
	if !caps.Barrier {
		p.Status, p.Message = "library_outdated", "A biblioteca não possui a atualização após as tarefas dos atores. Recompile-a com o gerador da prévia 7 antes de conectar."
		return p
	}
	p.Compatible = true
	var s struct {
		WorldRole                int    `json:"world_role"`
		WorldEpoch               uint64 `json:"world_epoch"`
		WorldCaptured            int    `json:"world_captured"`
		WorldMatched             int    `json:"world_matched"`
		WorldMissing             int    `json:"world_missing"`
		WorldApplied             int    `json:"world_applied"`
		WorldAISkipped           int    `json:"world_ai_skipped"`
		WorldAnimationsApplied   int    `json:"world_animations_applied"`
		VisibilityCaptured       int    `json:"visibility_captured"`
		VisibilityApplied        int    `json:"visibility_applied"`
		EffectPropertiesCaptured int    `json:"effect_properties_captured"`
		EffectPropertiesApplied  int    `json:"effect_properties_applied"`
		ArmorReferences          int    `json:"armor_references"`
		ArmorHeadBones           int    `json:"armor_head_bones"`
		ArmorUpperBones          int    `json:"armor_upper_bones"`
		ArmorLowerBones          int    `json:"armor_lower_bones"`
		EffectsSourceUsers       int    `json:"effects_source_users"`
		EffectsCaptured          int    `json:"effects_captured"`
		EffectsApplied           int    `json:"effects_applied"`
		EffectsStopped           int    `json:"effects_stopped"`
		EffectsMissingUser       int    `json:"effects_missing_user"`
		EffectLastKey            string `json:"effect_last_key"`
		SkeletonCaptured         int    `json:"skeleton_captured"`
		SkeletonApplied          int    `json:"skeleton_applied"`
		SkeletonReceived         int    `json:"skeleton_received"`
		BodyPoseApplied          int    `json:"body_pose_applied"`
		PoseCaptureStatus        string `json:"pose_capture_status,omitempty"`
		ArmorSource              string `json:"armor_source"`
		State                    string `json:"state"`
		Error                    string `json:"error"`
		Ticks                    uint64 `json:"game_ticks"`
		Age                      int64  `json:"tick_age_ms"`
		Active                   bool   `json:"active"`
		Model                    bool   `json:"model_ready"`
		Animation                bool   `json:"animation_ready"`
		Paused                   bool   `json:"game_paused"`
		Received                 uint64 `json:"received_sequence"`
		VisualOnly               bool   `json:"visual_only"`
		ResourceFallback         bool   `json:"resource_fallback"`
		FirstError               string `json:"first_error"`
	}
	q, ok = byName["hm_native_status"]
	if !ok || q.Error != "" || json.Unmarshal(q.Data, &s) != nil {
		p.Status, p.Message = "status_unavailable", "O runtime não respondeu ao estado nativo. Gere o relatório mantendo o mundo carregado."
		return p
	}
	p.WorldRole, p.WorldEpoch, p.WorldCaptured, p.WorldMatched, p.WorldMissing, p.WorldApplied, p.WorldAISkipped, p.WorldAnimationsApplied = s.WorldRole, s.WorldEpoch, s.WorldCaptured, s.WorldMatched, s.WorldMissing, s.WorldApplied, s.WorldAISkipped, s.WorldAnimationsApplied
	p.VisibilityCaptured, p.VisibilityApplied, p.EffectPropertiesCaptured, p.EffectPropertiesApplied, p.ArmorReferences = s.VisibilityCaptured, s.VisibilityApplied, s.EffectPropertiesCaptured, s.EffectPropertiesApplied, s.ArmorReferences
	p.ArmorHeadBones, p.ArmorUpperBones, p.ArmorLowerBones = s.ArmorHeadBones, s.ArmorUpperBones, s.ArmorLowerBones
	p.EffectsSourceUsers, p.EffectsCaptured, p.EffectsApplied, p.EffectsStopped, p.EffectsMissingUser, p.EffectLastKey = s.EffectsSourceUsers, s.EffectsCaptured, s.EffectsApplied, s.EffectsStopped, s.EffectsMissingUser, s.EffectLastKey
	p.GameTicks, p.RemoteActive = s.Ticks, s.Active
	p.SkeletonCaptured, p.SkeletonApplied, p.ArmorSource = s.SkeletonCaptured, s.SkeletonApplied, s.ArmorSource
	p.SkeletonReceived, p.BodyPoseApplied, p.PoseCaptureStatus = s.SkeletonReceived, s.BodyPoseApplied, s.PoseCaptureStatus
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
	case s.Active && s.Model && s.SkeletonReceived > 0 && s.BodyPoseApplied == 0:
		p.Status, p.Message = "pose_not_applied", "A pose chegou pela rede, mas não encontrou ossos compatíveis no corpo remoto."
	case s.Active && s.Model && s.PoseCaptureStatus != "" && s.SkeletonCaptured == 0:
		p.Status, p.Message = "pose_capture_unavailable", "O ator existe, mas a captura esquelética local falhou: "+s.PoseCaptureStatus
	case s.Active && s.Model && s.PoseCaptureStatus != "" && s.SkeletonReceived == 0:
		p.Status, p.Message = "pose_not_received", "O ator existe, mas a pose esquelética do outro cliente ainda não chegou."
	case s.Active && s.Model && s.SkeletonApplied > 0:
		p.Status, p.Message = "active", "Ator ativo com pose esquelética recebida. O desenho final e as interações ainda exigem teste no jogo."
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
