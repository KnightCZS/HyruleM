package multiplayer

import (
	"math"
	"strings"
	"testing"
)

func TestNativeCommandsRejectInvalidTelemetry(t *testing.T) {
	pose := Pose{Ready: true, Position: Vec3{3434.5, 235.9, 2101.1}, Velocity: Vec3{X: 2}, Yaw: 1}
	command, err := nativePoseCommand("0123456789abcdef", 42, pose)
	if err != nil || !strings.HasSuffix(command, " move") || len(command) > 256 || strings.ContainsAny(command, "\r\n") {
		t.Fatal(command, err)
	}
	for _, bad := range []Pose{{Ready: false}, {Ready: true, Yaw: math.NaN()}, {Ready: true, Position: Vec3{X: math.Inf(1)}}} {
		if _, err := nativePoseCommand("0123456789abcdef", 1, bad); err == nil {
			t.Fatal("invalid pose accepted")
		}
	}
	if _, err := nativePoseCommand("0123456789abcd\nx", 1, pose); err == nil {
		t.Fatal("session injection accepted")
	}
}
