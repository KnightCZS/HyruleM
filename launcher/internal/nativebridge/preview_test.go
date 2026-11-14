package nativebridge

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestIssue4OldRuntimeTakesPriorityOverGen1AndIdleState(t *testing.T) {
	// Protocol-only reproduction of issue #4. No game memory or file bytes.
	queries := []ProbeCommand{
		{Command: "hm_caps", Data: json.RawMessage(`{"runtime":"SavageWind-HyruleM","preview_abi":1,"native_preview":true,"native_actors":false}`)},
		{Command: "hm_native_status", Data: json.RawMessage(`{"state":"idle","applied_sequence":0,"received_sequence":0,"active":false}`)},
		{Command: "hm_world_status", Error: "Unknown command"},
	}
	p := DiagnosePreview(queries)
	if p.Status != "runtime_outdated" || p.Compatible || p.ActualABI != 1 || p.ExpectedABI != 3 || !strings.Contains(p.Message, "Iniciar-Nativo.cmd") {
		t.Fatal("installation failure misdiagnosed as gameplay", p)
	}
}

func TestPreviewDiagnosisSeparatesCallbackModelAndGen1(t *testing.T) {
	caps := ProbeCommand{Command: "hm_caps", Data: json.RawMessage(`{"runtime":"SavageWind-HyruleM","preview_abi":3,"native_preview":true,"shared_enemy_health":true,"native_actors":false}`)}
	for _, tt := range []struct{ state, want string }{
		{`{"game_ticks":0,"tick_age_ms":-1}`, "callback_stalled"},
		{`{"game_ticks":50,"tick_age_ms":0,"received_sequence":0}`, "no_native_session"},
		{`{"game_ticks":50,"tick_age_ms":0,"active":true}`, "model_missing"},
		{`{"game_ticks":50,"tick_age_ms":0,"active":true,"model_ready":true,"animation_ready":true}`, "active"},
		{`{"game_ticks":50,"tick_age_ms":0,"error":"native-spawn-rejected"}`, "actor_error"},
	} {
		t.Run(tt.want, func(t *testing.T) {
			p := DiagnosePreview([]ProbeCommand{caps, {Command: "hm_native_status", Data: json.RawMessage(tt.state)}})
			if !p.Compatible || p.Status != tt.want {
				t.Fatal(p)
			}
		})
	}
	if p := DiagnosePreview(nil); p.Status != "control_unavailable" {
		t.Fatal("offline report treated as actor failure", p)
	}
}
