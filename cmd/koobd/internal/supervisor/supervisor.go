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

// Package supervisor provides reactive supervision for system processes.
package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Supervisor watches a trigger file and maintains a running process.
type Supervisor struct {
	Name        string
	Binary      string
	Args        []string
	TriggerFile string
	LogFile     string
}

// Run watches for the optional TriggerFile and then starts the Binary in a
// loop. It returns an error if the process cannot be started initially or if
// the context is cancelled. This function follows the "Effective Go" principle
// of staying silent and returning errors to the caller.
func (s *Supervisor) Run(ctx context.Context) error {
	if s.TriggerFile != "" {
		if err := s.waitForTrigger(ctx); err != nil {
			return err
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := s.runOnce(ctx); err != nil {
				return err
			}

			// Wait before restart to avoid thrashing.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
	}
}

// waitForTrigger blocks until the TriggerFile exists or the context is
// cancelled.
func (s *Supervisor) waitForTrigger(ctx context.Context) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := os.Stat(s.TriggerFile); err == nil {
				return nil
			}
		}
	}
}

// runOnce executes the binary once and waits for it to complete.
func (s *Supervisor) runOnce(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, s.Binary, s.Args...) // #nosec G204

	if s.LogFile != "" {
		logfile, err := os.Create(s.LogFile)
		if err != nil {
			return fmt.Errorf(
				"failed to create log file %s: %w",
				s.LogFile,
				err,
			)
		}
		defer func() { _ = logfile.Close() }()
		cmd.Stdout = logfile
		cmd.Stderr = logfile
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %w", s.Binary, err)
	}

	return cmd.Wait()
}
