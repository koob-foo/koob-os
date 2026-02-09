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
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/koob-foo/koob-os/pkg/sysutil"
)

// setupCgroups initializes the cgroup v2 unified hierarchy, cleans up legacy
// v1 mounts, and sets up the required sub-groups for Kubernetes.
func setupCgroups() {
	slog.Info("Initializing Cgroups (Smart Mode)...")

	// Purge legacy V1 mounts (Hard Mode).
	purgeV1Mounts()

	// Try to unmount cgroup2 if already present (e.g., from previous failed run).
	if err := syscall.Unmount("/sys/fs/cgroup", 0); err != nil {
		slog.Debug("Pre-mount cgroup2 unmount (expected for first boot)",
			slog.Any("error", err))
	}

	// Mount cgroup2 unified hierarchy.
	flags := uintptr(syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC)
	if err := sysutil.MountIfMissing(
		"none",
		"/sys/fs/cgroup",
		"cgroup2",
		flags,
		"",
	); err != nil {
		slog.Error("Failed to mount cgroup2",
			slog.Any("error", err))
		return
	}
	slog.Info("Mounted cgroup2 on /sys/fs/cgroup")

	// Make cgroup2 shared explicitly.
	flagsSize := uintptr(syscall.MS_SHARED)
	if err := syscall.Mount("", "/sys/fs/cgroup", "", flagsSize, ""); err != nil {
		slog.Error("Failed to make cgroup2 shared",
			slog.Any("error", err))
	}

	// Verify Controllers.
	rootPath := "/sys/fs/cgroup"
	controllers, err := availableControllers(rootPath)
	if err != nil {
		slog.Warn("Could not read root controllers", slog.Any("error", err))
	} else {
		slog.Info("Available root cgroup controllers",
			slog.String("controllers", controllers))
	}

	// Create Sub-groups.
	groups := []string{
		"/sys/fs/cgroup/init",
		"/sys/fs/cgroup/system",
		"/sys/fs/cgroup/podruntime",
		"/sys/fs/cgroup/kubepods",
		"/sys/fs/cgroup/kubepods/besteffort",
		"/sys/fs/cgroup/kubepods/burstable",
	}
	for _, g := range groups {
		if err := sysutil.Mkdir(g); err != nil {
			slog.Error("Failed to create cgroup",
				slog.String("path", g),
				slog.Any("error", err))
		}
	}

	// Move Self to /init.
	moveSelfTo("/sys/fs/cgroup/init")

	// Delegate Controllers.
	delegateControllers("/sys/fs/cgroup", controllers)

	// Also delegate down to our sub-trees if needed.
	delegateGroups := []string{
		"/sys/fs/cgroup/system",
		"/sys/fs/cgroup/podruntime",
		"/sys/fs/cgroup/kubepods",
		"/sys/fs/cgroup/kubepods/besteffort",
		"/sys/fs/cgroup/kubepods/burstable",
	}
	for _, g := range delegateGroups {
		delegateControllers(g, controllers)
	}
}

// availableControllers reads the cgroup.controllers file to determine
// supported kernel controllers.
func availableControllers(path string) (string, error) {
	target := filepath.Join(path, "cgroup.controllers")
	// #nosec G304 - Path is trusted early-boot configuration.
	cData, err := os.ReadFile(target)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(cData)), nil
}

// moveSelfTo moves the current process's PID into a target cgroup.
func moveSelfTo(path string) {
	pid := os.Getpid()
	pidStr := fmt.Sprintf("%d", pid)
	target := filepath.Join(path, "cgroup.procs")
	if err := os.WriteFile(target, []byte(pidStr), 0600); err != nil {
		slog.Error("Could not move PID to cgroup",
			slog.Int("pid", pid),
			slog.String("path", path),
			slog.Any("error", err))
	} else {
		slog.Debug("Moved process to cgroup",
			slog.Int("pid", pid),
			slog.String("path", path))
	}
}

// delegateControllers enables specified controllers in the target cgroup's
// subtree_control.
func delegateControllers(path string, controllers string) {
	if controllers == "" {
		return
	}
	// Format: "+cpu +memory +io"
	parts := strings.Fields(controllers)
	var toEnable string
	for _, c := range parts {
		toEnable += "+" + c + " "
	}
	toEnable = strings.TrimSpace(toEnable)

	subtreePath := filepath.Join(path, "cgroup.subtree_control")
	if err := os.WriteFile(subtreePath, []byte(toEnable), 0600); err != nil {
		slog.Warn("Failed controller delegation",
			slog.String("path", path),
			slog.Any("error", err))
	} else {
		// Verify it actually stuck.
		// #nosec G304 - Path is trusted early-boot configuration.
		content, err := os.ReadFile(subtreePath)
		if err != nil {
			slog.Warn("Failed to verify controller delegation",
				slog.String("path", subtreePath),
				slog.Any("error", err))
			return
		}
		actual := strings.TrimSpace(string(content))
		slog.Debug("Delegated cgroup controllers",
			slog.String("path", path),
			slog.String("actual", actual))
	}
}

// purgeV1Mounts scans /proc/mounts and unmounts any legacy v1 cgroup
// filesystems to ensure a clean v2-only environment.
func purgeV1Mounts() {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		slog.Error("Could not read /proc/mounts for cgroup purge",
			slog.Any("error", err))
		return
	}
	defer func() { _ = f.Close() }()

	var mounts []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// format: device mountpoint fstype options dump pass
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 3 && fields[2] == "cgroup" {
			mounts = append(mounts, fields[1])
		}
	}

	for _, m := range mounts {
		slog.Info("Purging v1 cgroup mount", slog.String("mount", m))
		// Try normal unmount first.
		if err := syscall.Unmount(m, 0); err != nil {
			slog.Warn("Unmount failed, retrying with DETACH",
				slog.String("mount", m),
				slog.Any("error", err))
			if err := syscall.Unmount(m, syscall.MNT_DETACH); err != nil {
				slog.Error("Forced unmount failed",
					slog.String("mount", m),
					slog.Any("error", err))
			}
		}
	}
}
