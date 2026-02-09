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

// Package kubelet provides logic for generating Kubelet configuration.
package kubelet

import (
	"bytes"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"

	"github.com/koob-foo/koob-os/cmd/koobadm/internal/config"
	"github.com/koob-foo/koob-os/pkg/sysutil"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// Config holds the variables for the Kubelet configuration template.
type Config struct {
	NodeName string
}

// InitKubeletConfig writes the kubelet configuration file to
// /var/lib/kubelet/config.yaml.
func InitKubeletConfig(nodeName string) error {
	configDir := config.ResolvePath("/var/lib/kubelet")
	if err := sysutil.Mkdir(configDir); err != nil {
		return fmt.Errorf("failed to ensure kubelet directory: %w", err)
	}

	cfg := Config{
		NodeName: nodeName,
	}

	tmpl, err := template.ParseFS(
		templateFS,
		"templates/kubelet-config.yaml.tmpl",
	)
	if err != nil {
		return fmt.Errorf("failed to parse kubelet template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	outputPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(outputPath, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write kubelet config: %w", err)
	}

	slog.Info("Kubelet configuration generated", slog.String("path", outputPath))
	return nil
}
