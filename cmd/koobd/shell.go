// Copyright 2026 Koob Foo {{{
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License. }}}

package main

import (
	"log/slog"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

// StartShell launches the Gosh shell as a one-shot process. Once exited,
// the shell is not restarted, "sealing" the system console.
func StartShell() {
	slog.Info("koob OS IS ONLINE. SHELL ACTIVE...")

	// We need a persistent handle to /dev/console for CTTY setup.
	console, err := os.OpenFile("/dev/console", os.O_RDWR, 0)
	if err != nil {
		slog.Warn("Could not open /dev/console; using defaults", "error", err)
	} else {
		defer func() {
			if err := console.Close(); err != nil {
				slog.Debug("Failed to close console handle",
					slog.Any("error", err))
			}
		}()
	}

	// Use /bin/bb gosh directly for multicall dispatch.
	shell := exec.Command("/bin/bb", "gosh")

	if console != nil {
		shell.Stdin = console
		shell.Stdout = console
		shell.Stderr = console
		shell.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true,
			Ctty:   int(console.Fd()),
		}

		setSaneTermios(int(console.Fd()))
	} else {
		shell.Stdin = os.Stdin
		shell.Stdout = os.Stdout
		shell.Stderr = os.Stderr
		shell.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true,
		}
	}

	shell.Env = append(os.Environ(),
		"TERM=linux", // Vital for terminal detection
		"HOME=/",
		"PS1=# ",
	)

	if err := shell.Run(); err != nil {
		slog.Error("Shell exited with error", "error", err)
	}

	slog.Info("SYSTEM SEALED. CONSOLE ACCESS DISABLED.")
}

// setSaneTermios configures basic line editing and echo for the console.
func setSaneTermios(fd int) {
	t, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		slog.Warn("Could not get termios", "error", err)
		return
	}

	// Sane flags for serial console
	t.Iflag |= unix.ICRNL | unix.IXON
	t.Oflag |= unix.OPOST | unix.ONLCR
	t.Cflag |= unix.CS8 | unix.CREAD | unix.CLOCAL
	t.Lflag |= unix.ISIG | unix.ICANON | unix.IEXTEN | unix.ECHO |
		unix.ECHOE | unix.ECHOK

	if err := unix.IoctlSetTermios(fd, unix.TCSETS, t); err != nil {
		slog.Warn("Could not set termios", "error", err)
	}
}
