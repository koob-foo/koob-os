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

// Package pki handles generation and management of cluster certificates
package pki

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/koob-foo/koob-os/cmd/koobadm/internal/config"
	"github.com/koob-foo/koob-os/pkg/sysutil"
)

// Dir returns the directory where Kubernetes PKI assets are stored.
func Dir() string {
	return config.ResolvePath("/etc/kubernetes/pki")
}

const (
	keySize = 2048
)

// InitPKI generates the cluster's root of trust (CA) and all component
// certificates required for a secure control plane.
func InitPKI(nodeName, nodeIP string) error {
	pkiDir := Dir()
	if err := sysutil.Mkdir(pkiDir); err != nil {
		return err
	}
	// Root CA
	caCert, caKey, err := generateCA()
	if err != nil {
		return fmt.Errorf("failed to generate CA: %w", err)
	}
	if err := writeCertAndKey(pkiDir, "ca", caCert, caKey); err != nil {
		return err
	}
	slog.Info("Generated Root CA certificate")

	// API Server Certificate
	// We include the node name, IP, and Kubernetes service aliases in the
	// SANs to ensure TLS verification succeeds from any internal path
	sans := []string{
		nodeName,
		fmt.Sprintf("%s.internal", nodeName),
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc.cluster.local",
		"localhost",
		"127.0.0.1",
		nodeIP,
		"10.96.0.1",
	}

	apiCert, apiKey, err := generateSignedCert(
		caCert, caKey, "kube-apiserver", []string{}, sans,
	)
	if err != nil {
		return fmt.Errorf("failed to generate API server cert: %w", err)
	}
	if err := writeCertAndKey(pkiDir, "apiserver",
		apiCert, apiKey); err != nil {
		return err
	}
	slog.Info("Generated API Server certificate")

	// Kubelet Client Cert (Authority: system:masters)
	// Used by the API server to perform authoritative actions on Kubelets
	// (e.g. exec, logs)
	clientCert, clientKey, err := generateSignedCert(
		caCert, caKey, "kube-apiserver-kubelet-client",
		[]string{"system:masters"}, []string{},
	)
	if err != nil {
		return fmt.Errorf("failed to generate kubelet client cert: %w", err)
	}
	if err := writeCertAndKey(pkiDir, "apiserver-kubelet-client",
		clientCert, clientKey); err != nil {
		return err
	}
	slog.Info("Generated API Server Kubelet Client certificate")

	// Admin Client Cert (system:masters)
	adminCert, adminKey, err := generateSignedCert(
		caCert, caKey, "kubernetes-admin", []string{"system:masters"},
		[]string{},
	)
	if err != nil {
		return fmt.Errorf("failed to generate admin cert: %w", err)
	}
	if err := writeCertAndKey(pkiDir, "admin",
		adminCert, adminKey); err != nil {
		return err
	}
	slog.Info("Generated Admin Client certificate")

	// Kubelet Client Cert (system:node:<nodeName>)
	kubeletCert, kubeletKey, err := generateSignedCert(
		caCert, caKey, fmt.Sprintf("system:node:%s", nodeName),
		[]string{"system:nodes"}, []string{},
	)
	if err != nil {
		return fmt.Errorf("failed to generate kubelet client cert: %w", err)
	}
	if err := writeCertAndKey(pkiDir, "kubelet", kubeletCert,
		kubeletKey); err != nil {
		return err
	}
	slog.Info("Generated local Kubelet certificate")

	// Controller Manager Client Cert (system:kube-controller-manager)
	cmCert, cmKey, err := generateSignedCert(
		caCert, caKey, "system:kube-controller-manager",
		[]string{"system:kube-controller-manager"}, []string{},
	)
	if err != nil {
		return fmt.Errorf("failed to generate controller-manager cert: %w", err)
	}
	if err := writeCertAndKey(pkiDir, "controller-manager",
		cmCert, cmKey); err != nil {
		return err
	}
	slog.Info("Generated Controller Manager certificate")

	// Scheduler Client Cert (system:kube-scheduler)
	schedCert, schedKey, err := generateSignedCert(
		caCert, caKey, "system:kube-scheduler",
		[]string{"system:kube-scheduler"}, []string{},
	)
	if err != nil {
		return fmt.Errorf("failed to generate scheduler cert: %w", err)
	}
	if err := writeCertAndKey(pkiDir, "scheduler",
		schedCert, schedKey); err != nil {
		return err
	}
	slog.Info("Generated Scheduler certificate")

	// Service Account Token Signing Key
	// Unlike component certs, Service Accounts use JWTs. The Controller
	// Manager signs tokens with the private key; the API Server verifies
	// them with the public key
	saKey, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return fmt.Errorf("failed to generate SA key: %w", err)
	}
	if err := writeKey(pkiDir, "sa", saKey); err != nil {
		return err
	}
	if err := writePublicKey(pkiDir, "sa", &saKey.PublicKey); err != nil {
		return err
	}
	slog.Info("Generated Service Account signing keys")

	return nil
}

// CalculateCAHash computes the SHA256 of the Subject Public Key Info (SPKI)
// for discovery
// It checks local PKI first, then falls back to ServiceAccount mount for
// Pod compatibility
func CalculateCAHash() (string, error) {
	paths := []string{
		config.ResolvePath("/etc/kubernetes/pki/ca.crt"),
		"/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
	}

	var caBytes []byte
	var err error
	for _, path := range paths {
		caBytes, err = os.ReadFile(path) // #nosec G304
		if err == nil {
			break
		}
	}

	if err != nil {
		return "", fmt.Errorf("failed to read CA certificate: %w", err)
	}

	block, _ := pem.Decode(caBytes)
	if block == nil {
		return "", fmt.Errorf("failed to decode CA certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", err
	}

	hasher := sha256.New()
	hasher.Write(cert.RawSubjectPublicKeyInfo)
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func generateCA() (*x509.Certificate, *rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return nil, nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "Koob OS Root CA",
		},
		NotBefore: time.Now(),
		// 10 years
		NotAfter: time.Now().Add(time.Hour * 24 * 365 * 10),
		KeyUsage: x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		// BasicConstraints mark this as the "Anchor of Trust."
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(
		rand.Reader, tmpl, tmpl, &key.PublicKey, key,
	)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, err
	}

	return cert, key, nil
}

func generateSignedCert(caCert *x509.Certificate, caKey *rsa.PrivateKey,
	cn string, orgs []string, sans []string) (*x509.Certificate,
	*rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return nil, nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: orgs,
		},
		NotBefore: time.Now(),
		// 1 year
		NotAfter: time.Now().Add(time.Hour * 24 * 365),
		KeyUsage: x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth,
		},
	}

	for _, s := range sans {
		if ip := net.ParseIP(s); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, s)
		}
	}

	certDER, err := x509.CreateCertificate(
		rand.Reader, tmpl, caCert, &key.PublicKey, caKey,
	)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, err
	}

	return cert, key, nil
}

func writeCertAndKey(dir, name string, cert *x509.Certificate,
	key *rsa.PrivateKey) error {
	// Write Cert
	certPath := filepath.Join(dir, name+".crt")
	certOut, err := os.Create(certPath) // #nosec G304
	if err != nil {
		return err
	}
	if err := pem.Encode(certOut, &pem.Block{
		Type: "CERTIFICATE", Bytes: cert.Raw,
	}); err != nil {
		_ = certOut.Close()
		return err
	}
	if err := certOut.Close(); err != nil {
		return err
	}

	return writeKey(dir, name, key)
}

func writeKey(dir, name string, key *rsa.PrivateKey) error {
	keyPath := filepath.Join(dir, name+".key")
	keyOut, err := os.Create(keyPath) // #nosec G304
	if err != nil {
		return err
	}
	privBytes := x509.MarshalPKCS1PrivateKey(key)
	if err := pem.Encode(keyOut, &pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: privBytes,
	}); err != nil {
		_ = keyOut.Close()
		return err
	}
	return keyOut.Close()
}

func writePublicKey(dir, name string, pub *rsa.PublicKey) error {
	pubPath := filepath.Join(dir, name+".pub")
	pubOut, err := os.Create(pubPath) // #nosec G304
	if err != nil {
		return err
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		_ = pubOut.Close()
		return err
	}
	if err := pem.Encode(pubOut, &pem.Block{
		Type: "PUBLIC KEY", Bytes: pubBytes,
	}); err != nil {
		_ = pubOut.Close()
		return err
	}
	return pubOut.Close()
}
