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

// Package main provides the koobd daemon, the init system and
// supervisor for koob OS.
package main

import (
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/koob-foo/koob-os/pkg/sysutil"
)

func main() {
	if err := run(); err != nil {
		slog.Error("Fatal daemon error", "error", err)
		os.Exit(1)
	}
}

// run contains the core boot sequence for the koobd daemon.
func run() error {
	// Initialize structured logging with colored console handler.
	slog.SetDefault(slog.New(sysutil.NewConsoleHandler(slog.LevelInfo)))

	slog.Info("koob OS boot sequence starting...")

	// 0. Stabilize PID 1.
	// As PID 1, we must reap zombies to prevent system resource exhaustion.
	go reapZombies()

	// Filesystem & Environment
	setupMounts()
	setBaseEnv()

	// Kernel Subsystems
	setupCgroups()
	configureSysctls()

	// Networking & Identity
	initHardwareNet()
	configureDynamicNet()

	// Supervised Processes
	startManagedServices()
	StartShell()

	// Prevent PID 1 from exiting.
	select {}
}

// reapZombies loops indefinitely, reaping any orphaned child processes
// that have terminated.
func reapZombies() {
	slog.Info("Watching for zombies...")
	for {
		var status syscall.WaitStatus
		// Wait4 with pid=-1 reaps any child. WNOHANG ensures we don't block.
		pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)
		if err != nil {
			if err == syscall.ECHILD {
				// No children left to reap right now. Sleep briefly.
				time.Sleep(1 * time.Second)
				continue
			}
			slog.Debug("Wait4 error during reaping", slog.Any("error", err))
			time.Sleep(1 * time.Second)
			continue
		}

		if pid > 0 {
			slog.Debug("Reaped zombie process",
				slog.Int("pid", pid),
				slog.Int("exit_status", status.ExitStatus()))
			continue // Check for more immediately.
		}

		// pid == 0 means no zombies pending.
		time.Sleep(1 * time.Second)
	}
}
