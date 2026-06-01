//go:build linux || darwin || freebsd || openbsd || netbsd

package sandbox

import (
	"os"
	"os/exec"
	"os/user"
	"strconv"
)

func isUserInGroup(groupName string) (bool, int) {
	g, err := user.LookupGroup(groupName)
	if err != nil {
		return false, 0
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return false, 0
	}
	groups, err := os.Getgroups()
	if err != nil {
		return false, 0
	}
	for _, gID := range groups {
		if gID == gid {
			return true, gid
		}
	}
	return false, 0
}

func configureCmdGid(cmd *exec.Cmd, gid int) {
	// No-op for unprivileged execution (Go syscall.Credential requires root due to setgroups)
}
