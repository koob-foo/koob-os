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

// Package kubelet provides tests for the kubelet configuration logic.
package kubelet

import (
	"os"
	"strings"
	"testing"

	"github.com/koob-foo/koob-os/cmd/koobadm/internal/config"
)

func TestInitKubeletConfig(t *testing.T) {
	// Setup temporary root.
	tmpDir, err := os.MkdirTemp("", "koobadm-kubelet-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	oldRoot := config.RootPath
	config.RootPath = tmpDir
	defer func() { config.RootPath = oldRoot }()

	err = InitKubeletConfig("test-node")
	if err != nil {
		t.Fatalf("InitKubeletConfig failed: %v", err)
	}

	configPath := config.ResolvePath("/var/lib/kubelet/config.yaml")
	data, err := os.ReadFile(configPath) // #nosec G304
	if err != nil {
		t.Fatalf("failed to read generated config: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "kind: KubeletConfiguration") {
		t.Error("config missing 'kind: KubeletConfiguration'")
	}
	if !strings.Contains(content, "cgroupDriver: cgroupfs") {
		t.Error("config missing expected cgroupDriver")
	}
}
