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

// Package manifests provides tests for the control plane manifest generation.
package manifests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koob-foo/koob-os/cmd/koobadm/internal/config"
)

func TestInitManifests(t *testing.T) {
	// Setup temporary root.
	tmpDir, err := os.MkdirTemp("", "koobadm-manifests-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	oldRoot := config.RootPath
	config.RootPath = tmpDir
	defer func() { config.RootPath = oldRoot }()

	nodeIP := "10.0.0.5"
	err = InitManifests(nodeIP)
	if err != nil {
		t.Fatalf("InitManifests failed: %v", err)
	}

	manifestsDir := config.ResolvePath("/etc/kubernetes/manifests")
	expectedManifests := []string{
		"etcd.yaml",
		"kube-apiserver.yaml",
		"kube-controller-manager.yaml",
		"kube-scheduler.yaml",
	}

	for _, f := range expectedManifests {
		path := filepath.Join(manifestsDir, f)
		data, err := os.ReadFile(path) // #nosec G304
		if err != nil {
			t.Errorf("failed to read expected manifest %s: %v", f, err)
			continue
		}

		content := string(data)
		if !strings.Contains(content, "kind: Pod") {
			t.Errorf("manifest %s missing 'kind: Pod'", f)
		}

		if f == "kube-apiserver.yaml" && !strings.Contains(content, nodeIP) {
			t.Errorf("apiserver manifest missing nodeIP %s", nodeIP)
		}
	}
}
