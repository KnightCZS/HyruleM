//go:build windows

package launcher

import (
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var kernel = syscall.NewLazyDLL("kernel32.dll")
var snapshot = kernel.NewProc("CreateToolhelp32Snapshot")
var processFirst = kernel.NewProc("Process32FirstW")
var processNext = kernel.NewProc("Process32NextW")
var queryImage = kernel.NewProc("QueryFullProcessImageNameW")
var processTimes = kernel.NewProc("GetProcessTimes")

type processEntry struct {
	Size        uint32
	Usage       uint32
	PID         uint32
	DefaultHeap uintptr
	ModuleID    uint32
	Threads     uint32
	ParentPID   uint32
	Priority    int32
	Flags       uint32
	Exe         [260]uint16
}
type SystemFinder struct{}

func prepareCommand(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
func (SystemFinder) Find(executable string, since time.Time) []ProcessInfo {
	h, _, _ := snapshot.Call(2, 0)
	if h == ^uintptr(0) {
		return nil
	}
	defer syscall.CloseHandle(syscall.Handle(h))
	p := processEntry{Size: uint32(unsafe.Sizeof(processEntry{}))}
	ok, _, _ := processFirst.Call(h, uintptr(unsafe.Pointer(&p)))
	var result []ProcessInfo
	for ok != 0 {
		if !strings.EqualFold(syscall.UTF16ToString(p.Exe[:]), filepath.Base(executable)) {
			ok, _, _ = processNext.Call(h, uintptr(unsafe.Pointer(&p)))
			continue
		}
		ph, err := syscall.OpenProcess(0x1000, false, p.PID)
		if err == nil {
			buf := make([]uint16, 32768)
			n := uint32(len(buf))
			success, _, _ := queryImage.Call(uintptr(ph), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
			if success != 0 && SamePath(syscall.UTF16ToString(buf[:n]), executable) {
				var created, exited, kernelTime, userTime syscall.Filetime
				t, _, _ := processTimes.Call(uintptr(ph), uintptr(unsafe.Pointer(&created)), uintptr(unsafe.Pointer(&exited)), uintptr(unsafe.Pointer(&kernelTime)), uintptr(unsafe.Pointer(&userTime)))
				began := time.Time{}
				if t != 0 {
					began = time.Unix(0, created.Nanoseconds())
				}
				if since.IsZero() || (!began.IsZero() && !began.Before(since)) {
					result = append(result, ProcessInfo{PID: int(p.PID), ParentPID: int(p.ParentPID), Executable: executable, Started: began})
				}
			}
			syscall.CloseHandle(ph)
		}
		ok, _, _ = processNext.Call(h, uintptr(unsafe.Pointer(&p)))
	}
	return result
}
