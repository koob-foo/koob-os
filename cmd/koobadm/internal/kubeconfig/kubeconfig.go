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

// Package kubeconfig provides logic for generating KubeConfig files.
package kubeconfig

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/koob-foo/koob-os/cmd/koobadm/internal/config"
	"github.com/koob-foo/koob-os/cmd/koobadm/internal/pki"
	"github.com/koob-foo/koob-os/pkg/sysutil"
)

// InitKubeConfigs generates the standard set of kubeconfig files used by
// cluster components (admin, kubelet, controller-manager, scheduler).
func InitKubeConfigs(nodeName, _, controlPlaneEndpoint string) error {
	kubernetesDir := config.ResolvePath("/etc/kubernetes")

	// Ensure directory exists.
	if err := sysutil.Mkdir(kubernetesDir); err != nil {
		return fmt.Errorf("failed to ensure kubernetes directory: %w", err)
	}

	// admin.conf (Cluster Admin).
	if err := writeEmbeddedKubeConfig(
		kubernetesDir,
		"admin.conf",
		"kubernetes-admin",
		controlPlaneEndpoint,
		"ca",
		"admin",
	); err != nil {
		return err
	}
	slog.Info("KubeConfig generated", slog.String("file", "admin.conf"))

	// kubelet.conf (Node Identity).
	kubeletUser := fmt.Sprintf("system:node:%s", nodeName)
	if err := writeEmbeddedKubeConfig(
		kubernetesDir,
		"kubelet.conf",
		kubeletUser,
		controlPlaneEndpoint,
		"ca",
		"kubelet",
	); err != nil {
		return err
	}
	slog.Info("KubeConfig generated", slog.String("file", "kubelet.conf"))

	// controller-manager.conf.
	if err := writeEmbeddedKubeConfig(
		kubernetesDir,
		"controller-manager.conf",
		"system:kube-controller-manager",
		controlPlaneEndpoint,
		"ca",
		"controller-manager",
	); err != nil {
		return err
	}
	slog.Info(
		"KubeConfig generated",
		slog.String("file", "controller-manager.conf"),
	)

	// scheduler.conf.
	if err := writeEmbeddedKubeConfig(
		kubernetesDir,
		"scheduler.conf",
		"system:kube-scheduler",
		controlPlaneEndpoint,
		"ca",
		"scheduler",
	); err != nil {
		return err
	}
	slog.Info("KubeConfig generated", slog.String("file", "scheduler.conf"))

	return nil
}

// writeEmbeddedKubeConfig loads PEM-encoded PKI assets from the pki dir
// and bundles them into a single, self-contained KubeConfig file.
func writeEmbeddedKubeConfig(
	dir,
	filename,
	user,
	server,
	caName,
	clientName string,
) error {
	pkiDir := pki.Dir()
	caCertPath := filepath.Join(pkiDir, caName+".crt")
	caCert, err := os.ReadFile(caCertPath) // #nosec G304
	if err != nil {
		return fmt.Errorf("failed to read %s.crt: %w", caName, err)
	}
	clientCertPath := filepath.Join(pkiDir, clientName+".crt")
	clientCert, err := os.ReadFile(clientCertPath) // #nosec G304
	if err != nil {
		return fmt.Errorf("failed to read %s.crt: %w", clientName, err)
	}
	clientKeyPath := filepath.Join(pkiDir, clientName+".key")
	clientKey, err := os.ReadFile(clientKeyPath) // #nosec G304
	if err != nil {
		return fmt.Errorf("failed to read %s.key: %w", clientName, err)
	}

	config := generateEmbeddedKubeConfig(
		user,
		server,
		caCert,
		clientCert,
		clientKey,
	)
	outputPath := filepath.Join(dir, filename)
	if err := os.WriteFile(outputPath, []byte(config), 0600); err != nil {
		return fmt.Errorf("failed to write kubeconfig %s: %w", outputPath, err)
	}
	return nil
}

// generateEmbeddedKubeConfig returns a YAML string conforming to the K8s
// Config standard, with all certificates/keys embedded in base64.
func generateEmbeddedKubeConfig(
	user,
	server string,
	caCert,
	clientCert,
	clientKey []byte,
) string {
	caBase64 := base64.StdEncoding.EncodeToString(caCert)
	certBase64 := base64.StdEncoding.EncodeToString(clientCert)
	keyBase64 := base64.StdEncoding.EncodeToString(clientKey)

	return fmt.Sprintf(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: %s
    server: %s
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: %s
  name: %s@kubernetes
current-context: %s@kubernetes
kind: Config
preferences: {}
users:
- name: %s
  user:
    client-certificate-data: %s
    client-key-data: %s
`, caBase64, server, user, user, user, user, certBase64, keyBase64)
}
