package multiplayer

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func validScene(scene string) bool {
	if len(scene) < 1 || len(scene) > 64 || strings.Contains(scene, "..") {
		return false
	}
	for _, c := range scene {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '/') {
			return false
		}
	}
	return true
}
func sceneWorld(world, scene string) string {
	if scene == "" {
		return world
	}
	sum := sha256.Sum256([]byte(world + "\x00" + scene))
	return hex.EncodeToString(sum[:12])
}
