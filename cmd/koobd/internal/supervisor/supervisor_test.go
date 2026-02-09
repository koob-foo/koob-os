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

package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSupervisorImmediate(t *testing.T) {
	// Setup temporary root.
	tmpDir, err := os.MkdirTemp("", "supervisor-test-immediate")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	logFile := filepath.Join(tmpDir, "test.log")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use a simple command that writes to stdout and exits.
	s := &Supervisor{
		Name:    "test-immediate",
		Binary:  "/bin/sh",
		Args:    []string{"-c", "echo 'hello world'"},
		LogFile: logFile,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Run(ctx)
	}()

	// Wait for log file to contain the output.
	var found bool
	for i := 0; i < 50; i++ {
		data, err := os.ReadFile(logFile) // #nosec G304
		if err == nil && strings.Contains(string(data), "hello world") {
			found = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !found {
		t.Fatal("supervisor failed to start process or capture output")
	}

	cancel()
	err = <-errCh
	if err != nil && err != context.Canceled &&
		!strings.Contains(err.Error(), "context canceled") {
		t.Errorf("supervisor returned unexpected error: %v", err)
	}
}

func TestSupervisorReactive(t *testing.T) {
	// Setup temporary root.
	tmpDir, err := os.MkdirTemp("", "supervisor-test-reactive")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	triggerFile := filepath.Join(tmpDir, "trigger")
	logFile := filepath.Join(tmpDir, "test.log")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s := &Supervisor{
		Name:        "test-reactive",
		Binary:      "/bin/sh",
		Args:        []string{"-c", "echo 'triggered'"},
		TriggerFile: triggerFile,
		LogFile:     logFile,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Run(ctx)
	}()

	// Verify it hasn't started yet.
	time.Sleep(500 * time.Millisecond)
	if _, err := os.Stat(logFile); err == nil {
		t.Fatal("supervisor started before trigger file existed")
	}

	// Create trigger file.
	if err := os.WriteFile(triggerFile, []byte("go"), 0600); err != nil {
		t.Fatalf("failed to create trigger file: %v", err)
	}

	// Wait for log file to contain the output.
	var found bool
	for i := 0; i < 50; i++ {
		data, err := os.ReadFile(logFile) // #nosec G304
		if err == nil && strings.Contains(string(data), "triggered") {
			found = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !found {
		t.Fatal("supervisor failed to react to trigger file")
	}

	cancel()
	err = <-errCh
	if err != nil && err != context.Canceled &&
		!strings.Contains(err.Error(), "context canceled") {
		t.Errorf("supervisor returned unexpected error: %v", err)
	}
}
