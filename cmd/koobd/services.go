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
	"context"
	"log/slog"

	"github.com/koob-foo/koob-os/cmd/koobd/internal/supervisor"
)

// startManagedServices initializes and starts the core system services
// required for a functional Kubernetes node.
func startManagedServices() {
	containerd := &supervisor.Supervisor{
		Name:    "containerd",
		Binary:  "/bin/containerd",
		LogFile: "/var/log/containerd.log",
	}
	go runService(containerd)

	kubelet := &supervisor.Supervisor{
		Name:   "kubelet",
		Binary: "/bin/kubelet",
		Args: []string{
			"--kubeconfig", "/etc/kubernetes/kubelet.conf",
			"--config", "/var/lib/kubelet/config.yaml",
			"--v=2",
		},
		TriggerFile: "/etc/kubernetes/kubelet.conf",
		LogFile:     "/var/log/kubelet.log",
	}
	go runService(kubelet)
}

// runService manages the lifecycle of a single supervised process.
func runService(s *supervisor.Supervisor) {
	if s.TriggerFile != "" {
		slog.Info("Service watching for trigger file",
			slog.String("service", s.Name),
			slog.String("trigger", s.TriggerFile))
	} else {
		slog.Info("Service starting immediately",
			slog.String("service", s.Name))
	}

	if err := s.Run(context.Background()); err != nil {
		slog.Error("Service failure",
			slog.String("service", s.Name),
			slog.Any("error", err))
	}
}
