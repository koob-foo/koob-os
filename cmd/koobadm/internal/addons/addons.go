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

// Package addons manages core cluster addons like CoreDNS
package addons

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"log/slog"
	"strings"
	"text/template"
	"time"

	"github.com/koob-foo/koob-os/cmd/koobadm/internal/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// ApplyCoreDNS deploys CoreDNS as a cluster addon. It waits for the API
// server to become ready before applying the manifests.
func ApplyCoreDNS() error {
	adminConf := config.ResolvePath("/etc/kubernetes/admin.conf")
	var clientSet *kubernetes.Clientset

	slog.Info("Waiting for API Server (kube-system namespace)...")

	// Retry loop to ensure API is responsive
	var err error
	for i := 0; i < 60; i++ {
		cfg, cerr := clientcmd.BuildConfigFromFlags("", adminConf)
		if cerr == nil {
			clientSet, cerr = kubernetes.NewForConfig(cfg)
			if cerr == nil {
				_, cerr = clientSet.CoreV1().Namespaces().Get(
					context.Background(),
					"kube-system",
					metav1.GetOptions{},
				)
				if cerr == nil {
					slog.Info("API Server and kube-system are ready")
					err = nil
					break
				}
			}
		}
		err = cerr

		if i%5 == 0 && i > 0 {
			slog.Info("Connection status",
				slog.Int("attempt", i),
				slog.Int("max_attempts", 60),
			)
		}
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		return fmt.Errorf("timeout waiting for kube-system namespace: %w", err)
	}

	slog.Info("Applying CoreDNS manifests...")

	tmpl, err := template.ParseFS(
		templateFS,
		"templates/coredns.yaml.tmpl",
	)
	if err != nil {
		return fmt.Errorf("failed to parse coredns template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if err := applyManifests(clientSet, buf.String()); err != nil {
		return fmt.Errorf("failed to apply coredns manifests: %w", err)
	}

	slog.Info("CoreDNS Addon applied successfully")
	return nil
}

// applyManifests splits a multi-resource YAML and applies each to the cluster.
func applyManifests(c *kubernetes.Clientset, manifests string) error {
	parts := strings.Split(manifests, "---")
	for _, p := range parts {
		raw := strings.TrimSpace(p)
		if raw == "" {
			continue
		}

		if err := applySingleResource(c, raw); err != nil {
			return err
		}
	}
	return nil
}

// applySingleResource decodes a single YAML resource and creates it in the
// cluster.
func applySingleResource(c *kubernetes.Clientset, raw string) error {
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(raw), 4096)
	var typeMeta metav1.TypeMeta
	if err := decoder.Decode(&typeMeta); err != nil {
		return err
	}

	// Reset decoder for full object decode.
	decoder = yaml.NewYAMLOrJSONDecoder(strings.NewReader(raw), 4096)

	ctx := context.Background()

	switch typeMeta.Kind {
	case "ServiceAccount":
		var sa corev1.ServiceAccount
		if err := decoder.Decode(&sa); err != nil {
			return err
		}
		_, err := c.CoreV1().ServiceAccounts(sa.Namespace).Create(
			ctx,
			&sa,
			metav1.CreateOptions{},
		)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	case "ConfigMap":
		var cm corev1.ConfigMap
		if err := decoder.Decode(&cm); err != nil {
			return err
		}
		_, err := c.CoreV1().ConfigMaps(cm.Namespace).Create(
			ctx,
			&cm,
			metav1.CreateOptions{},
		)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	case "ClusterRole":
		var cr rbacv1.ClusterRole
		if err := decoder.Decode(&cr); err != nil {
			return err
		}
		_, err := c.RbacV1().ClusterRoles().Create(
			ctx,
			&cr,
			metav1.CreateOptions{},
		)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	case "ClusterRoleBinding":
		var crb rbacv1.ClusterRoleBinding
		if err := decoder.Decode(&crb); err != nil {
			return err
		}
		_, err := c.RbacV1().ClusterRoleBindings().Create(
			ctx,
			&crb,
			metav1.CreateOptions{},
		)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	case "Deployment":
		var deploy appsv1.Deployment
		if err := decoder.Decode(&deploy); err != nil {
			return err
		}
		_, err := c.AppsV1().Deployments(deploy.Namespace).Create(
			ctx,
			&deploy,
			metav1.CreateOptions{},
		)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	case "Service":
		var svc corev1.Service
		if err := decoder.Decode(&svc); err != nil {
			return err
		}
		_, err := c.CoreV1().Services(svc.Namespace).Create(
			ctx,
			&svc,
			metav1.CreateOptions{},
		)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}
