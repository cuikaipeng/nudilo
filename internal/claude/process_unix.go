//go:build !windows

package claude

import "os/exec"

func quietProcess(*exec.Cmd) {}
