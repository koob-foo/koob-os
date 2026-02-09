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
	"syscall"

	"github.com/koob-foo/koob-os/pkg/sysutil"
)

// setupMounts orchestrates the initialization of the virtual filesystem
// ecosystem, including API filesystems, device nodes, and runtime storage.
func setupMounts() {
	slog.Info("Configuring VFS Ecosystem...")

	mountAPIFilesystems()
	setupDevNodes()
	setupRuntimeStorage()
	setupBPF()
}

// mountAPIFilesystems ensures core kernel filesystems (proc, sys) are mounted
// and configured with correct propagation flags for container workloads.
func mountAPIFilesystems() {
	// Make Root Shared (Crucial for container mount propagation).
	// We always try this as it's safe and required.
	flags := uintptr(syscall.MS_SHARED | syscall.MS_REC)
	if err := syscall.Mount("", "/", "", flags, ""); err != nil {
		slog.Error("Failed to make root shared",
			slog.Any("error", err))
	}

	// Mount API Filesystems (Idempotent).
	if err := sysutil.MountIfMissing(
		"proc",
		"/proc",
		"proc",
		0,
		"",
	); err != nil {
		slog.Error("Failed to mount /proc",
			slog.Any("error", err))
	}

	if err := sysutil.MountIfMissing(
		"sysfs",
		"/sys",
		"sysfs",
		0,
		"",
	); err != nil {
		slog.Error("Failed to mount /sys",
			slog.Any("error", err))
	}

	// Make /sys shared for BPF propagation.
	flags = uintptr(syscall.MS_SHARED | syscall.MS_REC)
	if err := syscall.Mount("", "/sys", "", flags, ""); err != nil {
		slog.Error("Failed to make /sys shared",
			slog.Any("error", err))
	}
}

// setupDevNodes ensures /dev is populated via devtmpfs and configures
// pseudoterminal support.
func setupDevNodes() {
	if err := sysutil.Mkdir("/dev"); err != nil {
		slog.Warn("Failed to ensure /dev", slog.Any("error", err))
	}

	// Check if /dev is already mounted.
	if err := sysutil.MountIfMissing(
		"devtmpfs",
		"/dev",
		"devtmpfs",
		0,
		"",
	); err != nil {
		slog.Error("Could not mount /dev",
			slog.Any("error", err))
	}

	if err := sysutil.Mkdir("/dev/pts"); err != nil {
		slog.Warn("Failed to ensure /dev/pts", slog.Any("error", err))
	}
	if err := sysutil.MountIfMissing(
		"devpts",
		"/dev/pts",
		"devpts",
		0,
		"",
	); err != nil {
		slog.Error("Could not mount /dev/pts",
			slog.Any("error", err))
	}
}

// setupRuntimeStorage initializes ephemeral storage locations used by
// container runtimes and system services.
func setupRuntimeStorage() {
	dirs := []string{"/var/run", "/var/log", "/run", "/var/lib/dhcp"}
	for _, d := range dirs {
		if err := sysutil.Mkdir(d); err != nil {
			slog.Warn("Failed to ensure directory",
				slog.String("path", d),
				slog.Any("error", err))
		}
	}

	if err := sysutil.MountIfMissing(
		"tmpfs",
		"/run",
		"tmpfs",
		0,
		"",
	); err != nil {
		slog.Error("Failed to mount /run",
			slog.Any("error", err))
	}

	// Make /run shared for submount propagation (e.g. BPF/CNI).
	flags := uintptr(syscall.MS_SHARED | syscall.MS_REC)
	if err := syscall.Mount("", "/run", "", flags, ""); err != nil {
		slog.Error("Failed to make /run shared",
			slog.Any("error", err))
	}

	// Cilium/CNI Compatibility Mounts (tmpfs).
	cniDirs := []string{"/etc/sysctl.d", "/opt/cni/bin", "/etc/cni/net.d"}
	for _, d := range cniDirs {
		if err := sysutil.Mkdir(d); err != nil {
			slog.Warn("Failed to ensure CNI directory",
				slog.String("path", d),
				slog.Any("error", err))
		}
		if err := sysutil.MountIfMissing(
			"tmpfs",
			d,
			"tmpfs",
			0,
			"",
		); err != nil {
			slog.Error("Failed to mount tmpfs",
				slog.String("path", d),
				slog.Any("error", err))
		}
	}

	// containerd storage (tmpfs to avoid nested overlay issues).
	if err := sysutil.Mkdir("/var/lib/containerd"); err != nil {
		slog.Warn("Failed to ensure containerd directory",
			slog.Any("error", err))
	}
	if err := sysutil.MountIfMissing(
		"tmpfs",
		"/var/lib/containerd",
		"tmpfs",
		0,
		"",
	); err != nil {
		slog.Error("Failed to mount containerd tmpfs",
			slog.Any("error", err))
	}
}

// setupBPF ensures bpf filesystem is mounted and propagated for networking
// components.
func setupBPF() {
	if err := sysutil.Mkdir("/sys/fs/bpf"); err != nil {
		slog.Warn("Failed to ensure bpf directory",
			slog.Any("error", err))
	}
	if err := sysutil.MountIfMissing(
		"bpf",
		"/sys/fs/bpf",
		"bpf",
		0,
		"mode=0700",
	); err != nil {
		slog.Error("Failed to mount BPF",
			slog.Any("error", err))
	}
	// Make BPF shared.
	flags := uintptr(syscall.MS_SHARED | syscall.MS_REC)
	if err := syscall.Mount("", "/sys/fs/bpf", "", flags, ""); err != nil {
		slog.Error("Failed to make BPF shared",
			slog.Any("error", err))
	}
}

// setBaseEnv configures primary system environment variables.
func setBaseEnv() {
	path := "/usr/local/bin:/usr/bin:/bin:/usr/local/sbin:/usr/sbin:/sbin"
	if err := os.Setenv("PATH", path); err != nil {
		slog.Error("Failed to set PATH env", slog.Any("error", err))
	}
	if err := os.Setenv("LD_LIBRARY_PATH", "/lib:/usr/lib:/usr/local/lib"); err != nil {
		slog.Error("Failed to set LD_LIBRARY_PATH env", slog.Any("error", err))
	}
}
