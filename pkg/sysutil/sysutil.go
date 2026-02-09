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

// Package sysutil provides low-level system utilities for filesystem
// operations.
package sysutil

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"syscall"
)

// ANSI color codes for terminal output.
const (
	colorInfo  = "\033[1;36m" // Cyan
	colorWarn  = "\033[1;33m" // Yellow
	colorFail  = "\033[1;31m" // Red
	colorReset = "\033[0m"
)

// Standardized colored prefixes for log messages.
const (
	prefixInfo  = colorInfo + "[ INFO ]" + colorReset
	prefixWarn  = colorWarn + "[ WARN ]" + colorReset
	prefixFail  = colorFail + "[ FAIL ]" + colorReset
	prefixDebug = colorInfo + "[ DEBUG ]" + colorReset
)

// ConsoleHandler implements slog.Handler to provide colored console output.
type ConsoleHandler struct {
	level slog.Level
}

// NewConsoleHandler returns a new slog.Handler for colored console output.
func NewConsoleHandler(level slog.Level) slog.Handler {
	return &ConsoleHandler{level: level}
}

// Enabled reports whether the handler handles records at the given level.
func (h *ConsoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle formats the record for the system console with colored prefixes.
func (h *ConsoleHandler) Handle(_ context.Context, r slog.Record) error {
	p := prefixInfo
	switch r.Level {
	case slog.LevelWarn:
		p = prefixWarn
	case slog.LevelError:
		p = prefixFail
	case slog.LevelDebug:
		p = prefixDebug
	}

	fmt.Printf("%s %s\n", p, r.Message)
	r.Attrs(func(a slog.Attr) bool {
		fmt.Printf("   %s=%v\n", a.Key, a.Value)
		return true
	})
	return nil
}

// WithAttrs returns the original handler (no-op for our console format).
func (h *ConsoleHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }

// WithGroup returns the original handler (no-op for our console format).
func (h *ConsoleHandler) WithGroup(_ string) slog.Handler { return h }

// Mount ensures the target exists and performs a syscall mount.
func Mount(source, target, fstype string, flags uintptr, data string) error {
	if err := Mkdir(target); err != nil {
		return err
	}
	if err := syscall.Mount(source, target, fstype, flags, data); err != nil {
		return fmt.Errorf("failed to mount %s: %w", target, err)
	}
	return nil
}

// Mkdir ensures a directory exists with consistent permissions.
func Mkdir(path string) error {
	if err := os.MkdirAll(path, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	return nil
}

// IsMounted checks if a path is already a mount point.
func IsMounted(path string) bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == path {
			return true
		}
	}
	return false
}

// MountIfMissing mounts a filesystem only if it is not already mounted.
func MountIfMissing(
	source,
	target,
	fstype string,
	flags uintptr,
	data string,
) error {
	if IsMounted(target) {
		return nil
	}
	return Mount(source, target, fstype, flags, data)
}
