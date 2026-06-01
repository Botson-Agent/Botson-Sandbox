//go:build windows

package sandbox

import "os/exec"

func isUserInGroup(groupName string) (bool, int) {
	return false, 0
}

func configureCmdGid(cmd *exec.Cmd, gid int) {
	// No-op on Windows
}
