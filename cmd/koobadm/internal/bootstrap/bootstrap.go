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

// Package bootstrap handles early cluster RBAC and discovery initialization
package bootstrap

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"text/template"
	"time"

	"github.com/koob-foo/koob-os/cmd/koobadm/internal/config"
	"github.com/koob-foo/koob-os/cmd/koobadm/internal/status"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// InitBootstrapRBAC configures RBAC for nodes to join via TLS bootstrap.
func InitBootstrapRBAC() error {
	slog.Info("Waiting for API Server (Bootstrap/Discovery setup)...")

	var clientSet *kubernetes.Clientset
	var err error

	// Check if API and kube-system are ready
	for i := 0; i < 60; i++ {
		clientSet, err = status.Clientset()
		if err == nil {
			_, err = clientSet.CoreV1().Namespaces().Get(
				context.Background(),
				"kube-system",
				metav1.GetOptions{},
			)
			if err == nil {
				break
			}
		}
		time.Sleep(2 * time.Second)
		if i == 59 {
			return fmt.Errorf("timed out waiting for API server: %w", err)
		}
	}

	if err := applyJoinRBAC(clientSet); err != nil {
		return err
	}
	return initDiscovery(clientSet)
}

// initDiscovery creates cluster-info ConfigMap and RBAC for unauthenticated access.
func initDiscovery(c *kubernetes.Clientset) error {
	slog.Info("Initializing Discovery (cluster-info ConfigMap)...")

	ctx := context.Background()

	// Ensure kube-public namespace exists
	nsPublic := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-public"},
	}
	_, err := c.CoreV1().Namespaces().Create(
		ctx,
		nsPublic,
		metav1.CreateOptions{},
	)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create kube-public namespace: %w", err)
	}

	// Read CA Cert
	caCertPath := config.ResolvePath("/etc/kubernetes/pki/ca.crt")
	caCertData, err := os.ReadFile(caCertPath) // #nosec G304
	if err != nil {
		return fmt.Errorf("failed to read CA cert for discovery: %w", err)
	}

	// Create cluster-info ConfigMap with KubeConfig (kubeadm style)
	tmpl, err := template.ParseFS(
		templateFS,
		"templates/discovery.yaml.tmpl",
	)
	if err != nil {
		return fmt.Errorf("failed to parse discovery template: %w", err)
	}

	var buf bytes.Buffer
	data := struct{ CACert string }{
		CACert: base64.StdEncoding.EncodeToString(caCertData),
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute discovery template: %w", err)
	}

	discoveryConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-info",
			Namespace: "kube-public",
		},
		Data: map[string]string{
			"kubeconfig": buf.String(),
		},
	}
	_, err = c.CoreV1().ConfigMaps("kube-public").Create(
		ctx,
		discoveryConfig,
		metav1.CreateOptions{},
	)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create discovery configmap: %w", err)
	}

	// Create Role/RoleBinding for unauthenticated access to this ConfigMap
	discoveryRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeadm:bootstrap-signer-clusterinfo",
			Namespace: "kube-public",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"configmaps"},
				ResourceNames: []string{"cluster-info"},
				Verbs:         []string{"get"},
			},
		},
	}
	_, err = c.RbacV1().Roles("kube-public").Create(
		ctx,
		discoveryRole,
		metav1.CreateOptions{},
	)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create discovery role: %w", err)
	}

	discoveryRB := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeadm:bootstrap-signer-clusterinfo",
			Namespace: "kube-public",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "kubeadm:bootstrap-signer-clusterinfo",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "Group",
				Name:     "system:unauthenticated",
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
	}
	_, err = c.RbacV1().RoleBindings("kube-public").Create(
		ctx,
		discoveryRB,
		metav1.CreateOptions{},
	)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create discovery rolebinding: %w", err)
	}

	return nil
}

// applyJoinRBAC allows node bootstrappers to submit and auto-approve CSRs.
func applyJoinRBAC(c *kubernetes.Clientset) error {
	slog.Info("Applying Join RBAC for TLS bootstrapping...")

	ctx := context.Background()

	// Allow bootstrappers to submit CSRs
	crbBootstrap := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubeadm:kubelet-bootstrap",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "system:node-bootstrapper",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "Group",
				Name:     "system:bootstrappers",
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
	}
	_, err := c.RbacV1().ClusterRoleBindings().Create(
		ctx,
		crbBootstrap,
		metav1.CreateOptions{},
	)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create kubelet-bootstrap CRB: %w", err)
	}

	// Allow auto-approval of node client CSRs
	crbAutoApprove := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubeadm:node-autoapprove-bootstrap",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name: "system:certificates.k8s.io:" +
				"certificatesigningrequests:nodeclient",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "Group",
				Name:     "system:bootstrappers",
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
	}
	_, err = c.RbacV1().ClusterRoleBindings().Create(
		ctx,
		crbAutoApprove,
		metav1.CreateOptions{},
	)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create node-autoapprove CRB: %w", err)
	}

	return nil
}

// MarkControlPlane labels and taints the node as a control plane.
func MarkControlPlane() error {
	clientSet, err := status.Clientset()
	if err != nil {
		return err
	}

	nodeName, _ := os.Hostname()
	if nodeName == "" {
		nodeName = "koob-control-plane"
	}

	slog.Info("Waiting for node registration...", slog.String("node", nodeName))

	var node *corev1.Node
	ctx := context.Background()

	for i := 0; i < 60; i++ {
		node, err = clientSet.CoreV1().Nodes().Get(
			ctx,
			nodeName,
			metav1.GetOptions{},
		)
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	if node == nil {
		return fmt.Errorf("timed out waiting for node registration: %w", err)
	}

	slog.Info("Labeling and tainting node...", slog.String("node", nodeName))

	// Apply Labels
	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}
	node.Labels[status.ControlPlaneLabel] = ""

	// Apply Taint
	hasTaint := false
	for _, t := range node.Spec.Taints {
		if t.Key == status.ControlPlaneLabel &&
			t.Effect == corev1.TaintEffectNoSchedule {
			hasTaint = true
			break
		}
	}

	if !hasTaint {
		node.Spec.Taints = append(node.Spec.Taints, corev1.Taint{
			Key:    status.ControlPlaneLabel,
			Effect: corev1.TaintEffectNoSchedule,
		})
	}

	_, err = clientSet.CoreV1().Nodes().Update(
		ctx,
		node,
		metav1.UpdateOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to update node labels/taints: %w", err)
	}

	slog.Info("Control-plane labels and taints applied successfully")
	return nil
}
