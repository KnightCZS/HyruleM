//go:build !windows

package launcher

import (
	"os/exec"
	"time"
)

func prepareCommand(cmd *exec.Cmd) {}

type SystemFinder struct{}

func (SystemFinder) Find(executable string, since time.Time) []ProcessInfo { return nil }
