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

// Package join implements the worker node join flow.
package join

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/koob-foo/koob-os/cmd/koobadm/internal/config"
	"github.com/koob-foo/koob-os/cmd/koobadm/internal/kubelet"
	"github.com/koob-foo/koob-os/cmd/koobadm/internal/status"
	"github.com/koob-foo/koob-os/pkg/sysutil"
	certificatesv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// KubeletClientSignerName is the name of the signer used for kubelet client
// certificates.
const (
	KubeletClientSignerName = "kubernetes.io/kube-apiserver-client-kubelet"
)

// Cluster performs the TLS bootstrap flow to join a worker node to the cluster.
func Cluster(endpoint, token, caHash string) error {
	if !strings.HasPrefix(endpoint, "https://") &&
		!strings.HasPrefix(endpoint, "http://") {
		endpoint = "https://" + endpoint
	}
	// Pre-flight checks.
	isCP, err := status.IsControlPlane()
	if err == nil && isCP {
		return fmt.Errorf(
			"node is already a control plane (label %s present): "+
				"joining as worker is not allowed",
			status.ControlPlaneLabel,
		)
	}

	kubeletConf := config.ResolvePath("/etc/kubernetes/kubelet.conf")
	if _, err := os.Stat(kubeletConf); err == nil {
		return fmt.Errorf("node is already joined to a cluster "+
			"(kubelet.conf exists at %s)", kubeletConf)
	}

	slog.Info("Attempting to join cluster", slog.String("endpoint", endpoint))

	// Discovery Validation.
	slog.Info("Validating cluster identity", slog.String("hash", caHash))

	if err := validateDiscovery(endpoint, caHash); err != nil {
		return fmt.Errorf("discovery validation failed: %w", err)
	}

	slog.Info("Cluster identity verified")

	// Generate Kubelet Key & CSR.
	nodeName, err := os.Hostname()
	if err != nil {
		slog.Warn(
			"Could not determine hostname, using default",
			slog.Any("error", err),
		)
		nodeName = "koob-worker"
	}

	slog.Info("Generating Kubelet private key and CSR")

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}
	privBytes, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}
	block := &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}
	key := pem.EncodeToMemory(block)

	kubeletPKIDir := config.ResolvePath("/var/lib/kubelet/pki")
	if err := sysutil.Mkdir(kubeletPKIDir); err != nil {
		return fmt.Errorf("failed to ensure kubelet pki directory: %w", err)
	}
	keyPath := filepath.Join(kubeletPKIDir, "kubelet.key")
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return fmt.Errorf("failed to write kubelet key: %w", err)
	}

	// Create CSR.
	csrTemplate := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   fmt.Sprintf("system:node:%s", nodeName),
			Organization: []string{"system:nodes"},
		},
	}
	csrData, err := x509.CreateCertificateRequest(
		rand.Reader,
		csrTemplate,
		privKey,
	)
	if err != nil {
		return fmt.Errorf("failed to create CSR: %w", err)
	}

	// Create Bootstrap KubeConfig.
	slog.Info("Creating temporary bootstrap kubeconfig")
	bootstrapConfig := createBootstrapKubeConfig(endpoint, token)

	cfg, err := clientcmd.NewDefaultClientConfig(
		*bootstrapConfig,
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to load bootstrap config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to create bootstrap client: %w", err)
	}

	// Submit CSR.
	slog.Info("Submitting CSR to API Server")

	csr := &certificatesv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("node-csr-%s", nodeName),
		},
		Spec: certificatesv1.CertificateSigningRequestSpec{
			Request: pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE REQUEST",
				Bytes: csrData,
			}),
			SignerName: KubeletClientSignerName,
			Usages: []certificatesv1.KeyUsage{
				certificatesv1.UsageDigitalSignature,
				certificatesv1.UsageKeyEncipherment,
				certificatesv1.UsageClientAuth,
			},
		},
	}

	// Delete existing if any.
	err = clientset.CertificatesV1().CertificateSigningRequests().Delete(
		context.Background(),
		csr.Name,
		metav1.DeleteOptions{},
	)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		// We ignore 404s, but log other issues.
		slog.Debug("Cleanup of existing CSR skipped", slog.Any("error", err))
	}

	csrClient := clientset.CertificatesV1().CertificateSigningRequests()
	createdCSR, err := csrClient.Create(
		context.Background(),
		csr,
		metav1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to submit CSR: %w", err)
	}
	slog.Debug("CSR submitted", slog.String("uid", string(createdCSR.UID)))

	// Poll for approval.
	slog.Info("Waiting for CSR approval")
	fmt.Printf(
		" (manual approval: 'kubectl certificate approve %s')\n",
		csr.Name,
	)

	var certData []byte
	for i := 0; i < 60; i++ {
		c, err := clientset.CertificatesV1().CertificateSigningRequests().Get(
			context.Background(),
			csr.Name,
			metav1.GetOptions{},
		)
		if err == nil && len(c.Status.Certificate) > 0 {
			certData = c.Status.Certificate
			break
		}
		time.Sleep(5 * time.Second)
	}

	if len(certData) == 0 {
		return fmt.Errorf("timed out waiting for CSR approval")
	}

	kubeletCert := config.ResolvePath("/var/lib/kubelet/pki/kubelet.crt")
	if err := os.WriteFile(kubeletCert, certData, 0600); err != nil {
		return fmt.Errorf("failed to write kubelet certificate: %w", err)
	}

	// Generate final KubeConfig.
	slog.Info("Certificate issued. Generating final kubelet.conf")
	finalConfig := createFinalKubeConfig(endpoint, certData, key)
	kubeletConfPath := config.ResolvePath("/etc/kubernetes/kubelet.conf")
	err = clientcmd.WriteToFile(*finalConfig, kubeletConfPath)
	if err != nil {
		return fmt.Errorf("failed to write kubelet.conf: %w", err)
	}

	// Generate Kubelet Config.
	slog.Info("Generating Kubelet Configuration")
	if err := kubelet.InitKubeletConfig(nodeName); err != nil {
		return fmt.Errorf("failed to initialize kubelet config: %w", err)
	}

	slog.Info("Join complete. koobd should now start kubelet")
	return nil
}

func validateDiscovery(endpoint, expectedHash string) error {
	// Standardize hash format.
	expectedHash = strings.TrimPrefix(expectedHash, "sha256:")

	// Create temporary unauthenticated client.
	apiConfig := &api.Config{
		Clusters: map[string]*api.Cluster{
			"discovery": {
				Server:                endpoint,
				InsecureSkipTLSVerify: true,
			},
		},
		Contexts: map[string]*api.Context{
			"discovery": {Cluster: "discovery"},
		},
		CurrentContext: "discovery",
	}

	cfgD, err := clientcmd.NewDefaultClientConfig(
		*apiConfig,
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to create discovery client config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(cfgD)
	if err != nil {
		return fmt.Errorf("failed to create discovery clientset: %w", err)
	}

	// Fetch cluster-info ConfigMap.
	cm, err := clientset.CoreV1().ConfigMaps("kube-public").Get(
		context.Background(),
		"cluster-info",
		metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to fetch cluster-info: %w", err)
	}

	kubeconfig, ok := cm.Data["kubeconfig"]
	if !ok {
		return fmt.Errorf("cluster-info ConfigMap missing 'kubeconfig' data")
	}

	// Extract CA and hash it.
	cfg, err := clientcmd.Load([]byte(kubeconfig))
	if err != nil {
		return fmt.Errorf("failed to parse discovery kubeconfig: %w", err)
	}

	for _, cluster := range cfg.Clusters {
		if len(cluster.CertificateAuthorityData) == 0 {
			continue
		}

		block, _ := pem.Decode(cluster.CertificateAuthorityData)
		if block == nil {
			return fmt.Errorf("failed to decode discovery CA")
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse discovery certificate: %w", err)
		}

		hasher := sha256.New()
		hasher.Write(cert.RawSubjectPublicKeyInfo)
		actualHash := hex.EncodeToString(hasher.Sum(nil))

		if actualHash != expectedHash {
			return fmt.Errorf(
				"CA hash mismatch! Actual: %s, Expected: %s",
				actualHash,
				expectedHash,
			)
		}

		// Kubelet needs the CA cert to verify the API server.
		pkiDir := config.ResolvePath("/etc/kubernetes/pki")
		if err := sysutil.Mkdir(pkiDir); err != nil {
			return fmt.Errorf("failed to ensure pki directory: %w", err)
		}
		caPath := filepath.Join(pkiDir, "ca.crt")
		err = os.WriteFile(
			caPath,
			cluster.CertificateAuthorityData,
			0600,
		)
		if err != nil {
			return fmt.Errorf("failed to save CA cert: %w", err)
		}

		return nil
	}

	return fmt.Errorf("no CA data found in discovery config")
}

func createBootstrapKubeConfig(endpoint, token string) *api.Config {
	return &api.Config{
		Clusters: map[string]*api.Cluster{
			"kubernetes": {
				Server: endpoint,
				// we validated it manually in discovery
				InsecureSkipTLSVerify: true,
			},
		},
		Contexts: map[string]*api.Context{
			"bootstrap": {
				Cluster:  "kubernetes",
				AuthInfo: "bootstrap",
			},
		},
		CurrentContext: "bootstrap",
		AuthInfos: map[string]*api.AuthInfo{
			"bootstrap": {
				Token: token,
			},
		},
	}
}

func createFinalKubeConfig(
	endpoint string,
	certData,
	keyData []byte,
) *api.Config {
	return &api.Config{
		Clusters: map[string]*api.Cluster{
			"kubernetes": {
				Server:                endpoint,
				InsecureSkipTLSVerify: true,
			},
		},
		Contexts: map[string]*api.Context{
			"default": {
				Cluster:  "kubernetes",
				AuthInfo: "kubelet",
			},
		},
		CurrentContext: "default",
		AuthInfos: map[string]*api.AuthInfo{
			"kubelet": {
				ClientCertificateData: certData,
				ClientKeyData:         keyData,
			},
		},
	}
}
