package launcher

import (
	"os"
	"os/exec"
	"strconv"
)

func prepareNativeBuild(cmd *exec.Cmd) {
	prepareCommand(cmd)
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		kill := exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
		prepareCommand(kill)
		if err := kill.Run(); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}
