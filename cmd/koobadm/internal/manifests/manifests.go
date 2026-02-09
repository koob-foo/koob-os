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

// Package manifests handles the generation of static pod manifests for core
// cluster components (etcd, API server, controller manager, and scheduler).
package manifests

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

// Config holds the variables used for populating cluster manifest templates.
type Config struct {
	NodeIP string
}

// InitManifests generates the static pod manifests for core cluster components.
func InitManifests(nodeIP string) error {
	manifestsDir := config.ResolvePath("/etc/kubernetes/manifests")
	if err := sysutil.Mkdir(manifestsDir); err != nil {
		return fmt.Errorf("failed to ensure manifests directory: %w", err)
	}

	cfg := Config{
		NodeIP: nodeIP,
	}

	// Etcd.
	if err := writeManifestFromTemplate(
		manifestsDir,
		"etcd.yaml",
		"templates/etcd.yaml.tmpl",
		cfg,
	); err != nil {
		return err
	}
	slog.Info("Manifest generated", slog.String("component", "etcd"))

	// API Server.
	if err := writeManifestFromTemplate(
		manifestsDir,
		"kube-apiserver.yaml",
		"templates/kube-apiserver.yaml.tmpl",
		cfg,
	); err != nil {
		return err
	}
	slog.Info("Manifest generated", slog.String("component", "kube-apiserver"))

	// Controller Manager.
	if err := writeManifestFromTemplate(
		manifestsDir,
		"kube-controller-manager.yaml",
		"templates/kube-controller-manager.yaml.tmpl",
		cfg,
	); err != nil {
		return err
	}
	slog.Info(
		"Manifest generated", slog.String(
			"component",
			"kube-controller-manager",
		),
	)

	// Scheduler.
	if err := writeManifestFromTemplate(
		manifestsDir,
		"kube-scheduler.yaml",
		"templates/kube-scheduler.yaml.tmpl",
		cfg,
	); err != nil {
		return err
	}
	slog.Info("Manifest generated", slog.String("component", "kube-scheduler"))

	return nil
}

// writeManifestFromTemplate parses a template, executes it with the provided
// configuration, and writes the resulting manifest to the specified directory.
func writeManifestFromTemplate(
	dir,
	filename,
	templatePath string,
	cfg Config,
) error {
	tmpl, err := template.ParseFS(templateFS, templatePath)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", templatePath, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return fmt.Errorf(
			"failed to execute template %s: %w",
			templatePath,
			err,
		)
	}

	outputPath := filepath.Join(dir, filename)
	if err := os.WriteFile(outputPath, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write manifest %s: %w", outputPath, err)
	}
	return nil
}
