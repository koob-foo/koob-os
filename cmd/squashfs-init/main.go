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

// Package main provides the early bootloader for mounting the SquashFS root
// and transitioning to the main system daemon via switch_root.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/koob-foo/koob-os/pkg/sysutil"
)

func main() {
	if err := run(); err != nil {
		slog.Error("System halted", slog.Any("error", err))
		os.Exit(1)
	}
}

// run orchestrates the early boot sequence.
func run() error {
	// Mount /proc first to read kernel command line.
	// We use the raw syscall because we need /proc to detect other mounts.
	if err := sysutil.Mkdir("/proc"); err != nil {
		fmt.Printf("[ WARN ] Could not create /proc: %v\n", err)
	}
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		fmt.Printf("[ WARN ] Could not mount /proc: %v\n", err)
	}
	// Setup Logging Level based on cmdline.
	level := slog.LevelInfo
	if data, err := os.ReadFile("/proc/cmdline"); err == nil {
		raw := string(data)
		if strings.Contains(raw, "quiet") {
			level = slog.LevelWarn
		} else if strings.Contains(raw, "debug") {
			level = slog.LevelDebug
		}
	}
	slog.SetDefault(slog.New(sysutil.NewConsoleHandler(level)))

	// Setup PATH so we can find mount, switch_root etc.
	p := "/usr/local/bin:/usr/bin:/bin:/usr/local/sbin:/usr/sbin:/sbin:/bbin"
	if err := os.Setenv("PATH", p); err != nil {
		slog.Warn("Failed to set PATH", slog.Any("error", err))
	}

	fmt.Println(`
  _               _        ___  ____
 | | _____   ___ | |__    / _ \/ ___|
 | |/ / _ \ / _ \| '_ \  | | | \___ \
 |   < (_) | (_) | |_) | | |_| |___) |
 |_|\_\___/ \___/|_.__/   \___/|____/`)

	slog.Info("koob OS SquashFS bootloader (Monolithic)")

	// Mount other essential filesystems via sysutil.
	if err := sysutil.MountIfMissing("sysfs", "/sys", "sysfs", 0, ""); err != nil {
		slog.Warn("Failed to mount sysfs", slog.Any("error", err))
	}
	if err := sysutil.MountIfMissing("devtmpfs", "/dev", "devtmpfs", 0, ""); err != nil {
		slog.Warn("Failed to mount devtmpfs", slog.Any("error", err))
	}

	// Locate and Mount SquashFS Root.
	const squashPath = "/rootfs.squashfs"
	if _, err := os.Stat(squashPath); err != nil {
		return fmt.Errorf("rootfs.squashfs missing from initramfs: %w", err)
	}

	const squashMount = "/mnt/squashfs"
	slog.Info("Mounting SquashFS (Read-Only)...")
	// Use standard mount command for loop device support in early environment.
	cmd := exec.Command(
		"mount",
		"-t", "squashfs",
		"-o", "loop,ro",
		squashPath,
		squashMount,
	)
	if err := sysutil.Mkdir(squashMount); err != nil {
		return fmt.Errorf("failed to create squashfs mount point: %w", err)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to mount squashfs: %w", err)
	}

	// Setup OverlayFS for Read-Write access.
	const tmpfsMount = "/mnt/tmpfs"
	if err := sysutil.Mount("tmpfs", tmpfsMount, "tmpfs", 0, ""); err != nil {
		return fmt.Errorf("failed to mount tmpfs for overlay: %w", err)
	}

	upperDir := filepath.Join(tmpfsMount, "upper")
	workDir := filepath.Join(tmpfsMount, "work")
	if err := sysutil.Mkdir(upperDir); err != nil {
		return fmt.Errorf("failed to create upper directory: %w", err)
	}
	if err := sysutil.Mkdir(workDir); err != nil {
		return fmt.Errorf("failed to create work directory: %w", err)
	}

	const newRoot = "/newroot"
	slog.Info("Mounting OverlayFS (Read-Write)...")
	opts := fmt.Sprintf(
		"lowerdir=%s,upperdir=%s,workdir=%s",
		squashMount,
		upperDir,
		workDir,
	)
	if err := sysutil.Mount(
		"overlay",
		newRoot,
		"overlay",
		0,
		opts,
	); err != nil {
		return fmt.Errorf("failed to mount overlayfs: %w", err)
	}

	// Transition to New Root.
	slog.Info("Migrating backing mounts to new root...")
	backingBase := filepath.Join(newRoot, "mnt")

	// Move SquashFS to /newroot/mnt/squashfs.
	sqDest := filepath.Join(backingBase, "squashfs")
	if err := sysutil.Mount(
		squashMount,
		sqDest,
		"",
		syscall.MS_MOVE,
		"",
	); err != nil {
		slog.Warn("Failed to move squashfs mount", slog.Any("error", err))
	}

	// Move Tmpfs to /newroot/mnt/tmpfs.
	tmpDest := filepath.Join(backingBase, "tmpfs")
	if err := sysutil.Mount(
		tmpfsMount,
		tmpDest,
		"",
		syscall.MS_MOVE,
		"",
	); err != nil {
		slog.Warn("Failed to move tmpfs mount", slog.Any("error", err))
	}

	const initPath = "/bin/koobd"
	fullInitPath := filepath.Join(newRoot, initPath)
	if _, err := os.Stat(fullInitPath); err != nil {
		return fmt.Errorf("missing init binary at %s", fullInitPath)
	}

	slog.Info("Switching root",
		slog.String("newRoot", newRoot),
		slog.String("init", initPath))

	binary, err := exec.LookPath("switch_root")
	if err != nil {
		return fmt.Errorf("switch_root check failed: %w", err)
	}

	// Handover to switch_root (replaces process as PID 1).
	args := []string{"switch_root", newRoot, initPath}
	// #nosec G204 - Handover to switch_root is required for PID 1 transition
	return syscall.Exec(binary, args, os.Environ())
}
