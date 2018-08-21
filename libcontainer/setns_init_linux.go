// +build linux

package libcontainer

import (
	"fmt"
	"os"

	"github.com/opencontainers/runc/libcontainer/apparmor"
	"github.com/opencontainers/runc/libcontainer/keys"
	"github.com/opencontainers/runc/libcontainer/seccomp"
	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/sirupsen/logrus"

	"golang.org/x/sys/unix"
)

// linuxSetnsInit performs the container's initialization for running a new process
// inside an existing container.
type linuxSetnsInit struct {
	pipe          *os.File
	consoleSocket *os.File
	config        *initConfig
}

func (l *linuxSetnsInit) getSessionRingName() string {
	return fmt.Sprintf("_ses.%s", l.config.ContainerId)
}

func (l *linuxSetnsInit) Init() error {
	if !l.config.Config.NoNewKeyring {
		// do not inherit the parent's session keyring
		if _, err := keys.JoinSessionKeyring(l.getSessionRingName()); err != nil {
			logrus.Debugf("Child failed: keyrings %#v", l.config)
			return err
		}
	}
	if l.config.CreateConsole {
		if err := setupConsole(l.consoleSocket, l.config, false); err != nil {
			logrus.Debugf("Child failed: concsole %#v", l.config)
			return err
		}
		if err := system.Setctty(); err != nil {
			logrus.Debugf("Child failed: setctty %#v", l.config)
			return err
		}
	}
	if l.config.NoNewPrivileges {
		if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
			logrus.Debugf("Child failed: no new privileges %#v", l.config)
			return err
		}
	}
	// Without NoNewPrivileges seccomp is a privileged operation, so we need to
	// do this before dropping capabilities; otherwise do it as late as possible
	// just before execve so as few syscalls take place after it as possible.
	if l.config.Config.Seccomp != nil && !l.config.NoNewPrivileges {
		if err := seccomp.InitSeccomp(l.config.Config.Seccomp); err != nil {
			logrus.Debugf("Child failed: seccomp %#v", l.config)
			return err
		}
	}
	if err := finalizeNamespace(l.config); err != nil {
		logrus.Debugf("Child failed: finalizeNamespace %#v", l.config)
		return err
	}
	if err := apparmor.ApplyProfile(l.config.AppArmorProfile); err != nil {
		logrus.Debugf("Child failed: apparmor %#v", l.config)
		return err
	}
	if err := label.SetProcessLabel(l.config.ProcessLabel); err != nil {
		logrus.Debugf("Child failed: ProcessLabel %#v", l.config)
		return err
	}
	// Set seccomp as close to execve as possible, so as few syscalls take
	// place afterward (reducing the amount of syscalls that users need to
	// enable in their seccomp profiles).
	if l.config.Config.Seccomp != nil && l.config.NoNewPrivileges {
		if err := seccomp.InitSeccomp(l.config.Config.Seccomp); err != nil {
			logrus.Debugf("Child failed: keyrings %#v", l.config)
			return newSystemErrorWithCause(err, "init seccomp")
		}
	}

	logrus.Debugf("Child about to exec user process, so setup complete %#v", l.config)

	return system.Execv(l.config.Args[0], l.config.Args[0:], os.Environ())
}
