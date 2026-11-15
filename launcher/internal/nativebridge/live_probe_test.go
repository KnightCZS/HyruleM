package nativebridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	core "hyrulelauncher/internal/launcher"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestLiveProbeContainsFeatureCountersWithoutGameReadsOrMutations(t *testing.T) {
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	_, p, _ := net.SplitHostPort(l.Addr().String())
	port, _ := strconv.Atoi(p)
	seen := make(chan []string, 1)
	go func() {
		var commands []string
		for i := 0; i < 8; i++ {
			c, e := l.Accept()
			if e != nil {
				seen <- commands
				return
			}
			s := bufio.NewScanner(c)
			if s.Scan() {
				cmd := s.Text()
				commands = append(commands, cmd)
				var data any = map[string]any{}
				switch cmd {
				case "hm_caps":
					data = map[string]any{"runtime": "SavageWind-HyruleM", "preview_abi": core.NativePreviewABI, "native_preview": true, "actor_job_barrier": true, "shared_enemy_health": true}
				case "hm_native_status":
					data = map[string]any{"state": "active", "active": true, "model_ready": true, "game_ticks": 90, "tick_age_ms": 0, "pose_capture_status": "captured", "skeleton_captured": 85, "skeleton_received": 85, "body_pose_applied": 85, "skeleton_applied": 170}
				case "hm_local_state":
					data = map[string]any{"ready": true, "skeleton_bones": 85}
				case "hm_world_status":
					data = map[string]any{"enemies_json": "[]", "pending_zero_events": 1}
				case "hm_items_status":
					data = map[string]any{"items_json": "[]", "world_items_active": true, "drop_error": "apple-missing"}
				}
				_ = json.NewEncoder(c).Encode(map[string]any{"ok": true, "data": data})
			}
			c.Close()
		}
		seen <- commands
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	r, err := ProbeSessionAt(ctx, core.Config{DataDir: t.TempDir()}, t.TempDir(), 2, port)
	if err != nil || r.Slot != 2 || r.Preview.Status != "active" || r.Preview.SkeletonReceived != 85 || r.Preview.BodyPoseApplied != 85 {
		t.Fatal(r, err)
	}
	if len(r.Files) != 0 || r.Inspection != nil {
		t.Fatal("live probe read game files or inspected private memory")
	}
	allowed := map[string]bool{"version": true, "hm_caps": true, "hm_native_status": true, "hm_local_state": true, "hm_world_status": true, "hm_items_status": true, "time": true, "weather": true}
	for _, cmd := range <-seen {
		if !allowed[cmd] {
			t.Fatal("unexpected live command", cmd)
		}
	}
	b, _ := json.Marshal(r)
	if !json.Valid(b) {
		t.Fatal(fmt.Sprintf("invalid report: %s", b))
	}
}

func TestPoseDiagnosisSeparatesCaptureNetworkAndBodyApplication(t *testing.T) {
	caps, _ := json.Marshal(map[string]any{"runtime": "SavageWind-HyruleM", "preview_abi": core.NativePreviewABI, "native_preview": true, "actor_job_barrier": true, "shared_enemy_health": true})
	for _, tt := range []struct {
		capture, receive, body int
		reason, want           string
	}{
		{0, 0, 0, "sdk-model-unavailable", "pose_capture_unavailable"},
		{85, 0, 0, "captured", "pose_not_received"},
		{85, 85, 0, "captured", "pose_not_applied"},
		{85, 85, 85, "captured", "active"},
	} {
		state, _ := json.Marshal(map[string]any{"active": true, "model_ready": true, "game_ticks": 90, "tick_age_ms": 0, "pose_capture_status": tt.reason, "skeleton_captured": tt.capture, "skeleton_received": tt.receive, "body_pose_applied": tt.body, "skeleton_applied": tt.body})
		r := DiagnosePreview([]ProbeCommand{{Command: "hm_caps", Data: caps}, {Command: "hm_native_status", Data: state}})
		if r.Status != tt.want {
			t.Fatal(tt, r)
		}
	}
}
