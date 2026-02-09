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

// Package kubeconfig provides tests for the kubeconfig generation logic.
package kubeconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koob-foo/koob-os/cmd/koobadm/internal/config"
	"github.com/koob-foo/koob-os/cmd/koobadm/internal/pki"
	"github.com/koob-foo/koob-os/pkg/sysutil"
)

func TestInitKubeConfigs(t *testing.T) {
	// Setup temporary root.
	tmpDir, err := os.MkdirTemp("", "koobadm-kubeconfig-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	oldRoot := config.RootPath
	config.RootPath = tmpDir
	defer func() { config.RootPath = oldRoot }()

	// Mock PKI files (required by InitKubeConfigs).
	pkiDir := pki.Dir()
	if err := sysutil.Mkdir(pkiDir); err != nil {
		t.Fatalf("failed to create pki dir: %v", err)
	}

	mockCerts := []string{
		"ca.crt", "admin.crt", "admin.key",
		"kubelet.crt", "kubelet.key",
		"controller-manager.crt", "controller-manager.key",
		"scheduler.crt", "scheduler.key",
	}
	for _, f := range mockCerts {
		err := os.WriteFile(
			filepath.Join(pkiDir, f),
			[]byte("mock-data"),
			0600,
		)
		if err != nil {
			t.Fatalf("failed to write mock cert %s: %v", f, err)
		}
	}

	// Run InitKubeConfigs.
	nodeName := "test-node"
	endpoint := "https://1.2.3.4:6443"
	err = InitKubeConfigs(nodeName, "", endpoint)
	if err != nil {
		t.Fatalf("InitKubeConfigs failed: %v", err)
	}

	// Verify Files exist.
	kubernetesDir := config.ResolvePath("/etc/kubernetes")
	expectedConfigs := []string{
		"admin.conf",
		"kubelet.conf",
		"controller-manager.conf",
		"scheduler.conf",
	}

	for _, f := range expectedConfigs {
		path := filepath.Join(kubernetesDir, f)
		data, err := os.ReadFile(path) // #nosec G304
		if err != nil {
			t.Errorf("failed to read expected config %s: %v", f, err)
			continue
		}

		// Basic content validation.
		content := string(data)
		if !strings.Contains(content, "kind: Config") {
			t.Errorf("config %s missing 'kind: Config'", f)
		}
		if !strings.Contains(content, endpoint) {
			t.Errorf("config %s missing endpoint %s", f, endpoint)
		}
	}
}
