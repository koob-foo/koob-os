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

// Package token manages bootstrap token generation and registration.
package token

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/koob-foo/koob-os/cmd/koobadm/internal/config"
	"github.com/koob-foo/koob-os/cmd/koobadm/internal/pki"
	"github.com/koob-foo/koob-os/cmd/koobadm/internal/status"
	"github.com/koob-foo/koob-os/pkg/netutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// CreateToken generates a bootstrap token, registers it with the API, and
// prints the result.
func CreateToken(printJoin bool) error {
	// Pre-flight checks.
	isCP, err := status.IsControlPlane()
	if err != nil {
		return fmt.Errorf("failed to check control-plane status: %w", err)
	}
	if !isCP {
		return fmt.Errorf(
			"koobadm token must be run on a control-plane node "+
				"(label %s missing)",
			status.ControlPlaneLabel,
		)
	}

	// Generate Token ID and Secret.
	tokenID, err := generateRandomString(6)
	if err != nil {
		return err
	}
	tokenSecret, err := generateRandomString(16)
	if err != nil {
		return err
	}
	token := fmt.Sprintf("%s.%s", tokenID, tokenSecret)

	// Calculate Discovery Hash (CA Cert Hash).
	caHash, err := pki.CalculateCAHash()
	if err != nil {
		return fmt.Errorf("failed to calculate CA hash: %w", err)
	}

	// Register Secret with Kubernetes API.
	if err := registerBootstrapToken(tokenID, tokenSecret); err != nil {
		return fmt.Errorf("failed to register token with API: %w", err)
	}

	// Output.
	if printJoin {
		// Detect Node IP for the join command.
		nodeIP, err := netutil.DefaultIP()
		if err != nil {
			nodeIP = "<MASTER_IP>"
		}

		slog.Info("Bootstrap token generated", slog.String("token", token))

		fmt.Println("\nTo join a worker node, run the following command:")
		fmt.Println(strings.Repeat("-", 79))
		fmt.Printf(
			"koobadm join %s:6443 \\\n"+
				"  --token %s \\\n"+
				"  --discovery-token-ca-cert-hash sha256:%s\n",
			nodeIP,
			token,
			caHash,
		)
		fmt.Println(strings.Repeat("-", 79))
	} else {
		fmt.Println(token)
	}

	return nil
}

// generateRandomString produces a random string of length n for token
// components.
func generateRandomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	// Use lower-case alphanumeric for k8s compatibility.
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, n)
	for i, val := range b {
		result[i] = charset[int(val)%len(charset)]
	}
	return string(result), nil
}

// registerBootstrapToken creates a bootstrap.kubernetes.io/token type Secret
// in the API. It is environment-aware: uses admin.conf locally or
// InClusterConfig if in a Pod.
func registerBootstrapToken(id, secret string) error {
	var cfg *rest.Config
	var err error

	// Try local admin config (CLI mode).
	adminPath := config.ResolvePath("/etc/kubernetes/admin.conf")
	if _, err = os.Stat(adminPath); err == nil {
		cfg, err = clientcmd.BuildConfigFromFlags("", adminPath)
	} else {
		// Fallback to In-Cluster Config (Pod mode).
		cfg, err = rest.InClusterConfig()
	}

	if err != nil {
		return fmt.Errorf("failed to load kubernetes configuration: %w", err)
	}

	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	secretName := fmt.Sprintf("bootstrap-token-%s", id)
	expiration := time.Now().Add(24 * time.Hour).Format(time.RFC3339)

	const extraGroups = "system:bootstrappers:kubeadm:default-node-token"
	bootstrapSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "kube-system",
		},
		Type: corev1.SecretType("bootstrap.kubernetes.io/token"),
		Data: map[string][]byte{
			"token-id":                       []byte(id),
			"token-secret":                   []byte(secret),
			"usage-bootstrap-authentication": []byte("true"),
			"usage-bootstrap-signing":        []byte("true"),
			"expiration":                     []byte(expiration),
			"auth-extra-groups":              []byte(extraGroups),
		},
	}

	_, err = clientSet.CoreV1().Secrets("kube-system").Create(
		context.Background(),
		bootstrapSecret,
		metav1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to create bootstrap secret: %w", err)
	}
	return nil
}
