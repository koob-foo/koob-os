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

// Package pki provides tests for the cluster PKI infrastructure.
package pki

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/koob-foo/koob-os/cmd/koobadm/internal/config"
)

func TestInitPKI(t *testing.T) {
	// Setup temporary root.
	tmpDir, err := os.MkdirTemp("", "koobadm-pki-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Set RootPath for the duration of the test.
	oldRoot := config.RootPath
	config.RootPath = tmpDir
	defer func() { config.RootPath = oldRoot }()

	// Run InitPKI.
	nodeName := "test-node"
	nodeIP := "192.168.1.100"
	err = InitPKI(nodeName, nodeIP)
	if err != nil {
		t.Fatalf("InitPKI failed: %v", err)
	}

	// Verify Files exist.
	pkiDir := Dir()
	expectedFiles := []string{
		"ca.crt", "ca.key",
		"apiserver.crt", "apiserver.key",
		"apiserver-kubelet-client.crt", "apiserver-kubelet-client.key",
		"admin.crt", "admin.key",
		"kubelet.crt", "kubelet.key",
		"controller-manager.crt", "controller-manager.key",
		"scheduler.crt", "scheduler.key",
		"sa.pub", "sa.key",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(pkiDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s does not exist", f)
		}
	}

	// Verify CA Certificate content.
	caCertPath := filepath.Join(pkiDir, "ca.crt")
	caCertData, err := os.ReadFile(caCertPath) // #nosec G304
	if err != nil {
		t.Fatalf("failed to read ca.crt: %v", err)
	}

	block, _ := pem.Decode(caCertData)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatal("failed to decode CA certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse CA certificate: %v", err)
	}

	if cert.Subject.CommonName != "Koob OS Root CA" {
		t.Errorf(
			"expected CA CN 'Koob OS Root CA', got '%s'",
			cert.Subject.CommonName,
		)
	}

	if !cert.IsCA {
		t.Error("expected CA certificate to have IsCA=true")
	}

	// Verify API Server Certificate SANs.
	apiCertPath := filepath.Join(pkiDir, "apiserver.crt")
	apiCertData, err := os.ReadFile(apiCertPath) // #nosec G304
	if err != nil {
		t.Fatalf("failed to read apiserver.crt: %v", err)
	}

	block, _ = pem.Decode(apiCertData)
	cert, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse API server certificate: %v", err)
	}

	expectedDNS := []string{
		nodeName,
		nodeName + ".internal",
		"kubernetes",
		"localhost",
	}
	foundDNS := make(map[string]bool)
	for _, dns := range cert.DNSNames {
		foundDNS[dns] = true
	}

	for _, dns := range expectedDNS {
		if !foundDNS[dns] {
			t.Errorf(
				"expected DNS SAN %s not found in API server certificate",
				dns,
			)
		}
	}

	expectedIP := []string{nodeIP, "127.0.0.1", "10.96.0.1"}
	foundIP := make(map[string]bool)
	for _, ip := range cert.IPAddresses {
		foundIP[ip.String()] = true
	}

	for _, ip := range expectedIP {
		if !foundIP[ip] {
			t.Errorf(
				"expected IP SAN %s not found in API server certificate",
				ip,
			)
		}
	}
}
