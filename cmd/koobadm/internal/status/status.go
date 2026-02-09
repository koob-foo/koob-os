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

// Package status provides logic for checking node control-plane status
package status

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/koob-foo/koob-os/cmd/koobadm/internal/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ControlPlaneLabel is the label applied to control plane nodes.
const ControlPlaneLabel = "node-role.kubernetes.io/control-plane"

// Clientset returns a Kubernetes clientset by automatically detecting
// the best available kubeconfig (admin.conf or kubelet.conf).
func Clientset() (*kubernetes.Clientset, error) {
	adminConf := config.ResolvePath("/etc/kubernetes/admin.conf")
	kubeletConf := config.ResolvePath("/etc/kubernetes/kubelet.conf")

	var configPath string
	if _, err := os.Stat(adminConf); err == nil {
		configPath = adminConf
	} else if _, err := os.Stat(kubeletConf); err == nil {
		configPath = kubeletConf
	}

	if configPath == "" {
		return nil, fmt.Errorf("no kubernetes configuration found")
	}

	slog.Debug(
		"Kubernetes configuration detected",
		slog.String("path", configPath),
	)

	cfg, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to load kubeconfig %s: %w",
			configPath,
			err,
		)
	}

	return kubernetes.NewForConfig(cfg)
}

// IsControlPlane checks if the current node has the control-plane label.
// It tries to use admin.conf or kubelet.conf to contact the API server.
func IsControlPlane() (bool, error) {
	clientSet, err := Clientset()
	if err != nil {
		// If no config exists, it isn't a control plane.
		return false, nil
	}

	// Get local node labels.
	nodeName, err := os.Hostname()
	if err != nil {
		return false, fmt.Errorf("failed to get hostname: %w", err)
	}

	node, err := clientSet.CoreV1().Nodes().Get(
		context.Background(),
		nodeName,
		metav1.GetOptions{},
	)
	if err != nil {
		// Fallback for bootstrap phase: if admin.conf exists, we are likely a
		// control plane.
		adminConf := config.ResolvePath("/etc/kubernetes/admin.conf")
		if _, serr := os.Stat(adminConf); serr == nil {
			slog.Debug(
				"API Server unreachable; assuming control plane due to local "+
					"admin.conf",
				slog.Any("error", err),
			)
			return true, nil
		}
		return false, fmt.Errorf("failed to get node info from API: %w", err)
	}

	// Check for label.
	_, exists := node.Labels[ControlPlaneLabel]
	return exists, nil
}
