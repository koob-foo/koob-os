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

// Package main provides the koobadm utility for managing koob OS nodes
package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/koob-foo/koob-os/cmd/koobadm/internal/addons"
	"github.com/koob-foo/koob-os/cmd/koobadm/internal/bootstrap"
	"github.com/koob-foo/koob-os/cmd/koobadm/internal/join"
	"github.com/koob-foo/koob-os/cmd/koobadm/internal/kubeconfig"
	"github.com/koob-foo/koob-os/cmd/koobadm/internal/kubelet"
	"github.com/koob-foo/koob-os/cmd/koobadm/internal/manifests"
	"github.com/koob-foo/koob-os/cmd/koobadm/internal/pki"
	"github.com/koob-foo/koob-os/cmd/koobadm/internal/token"
	"github.com/koob-foo/koob-os/pkg/netutil"
	"github.com/koob-foo/koob-os/pkg/sysutil"
	"github.com/koob-foo/koob-os/pkg/version"
)

func main() {
	if err := run(); err != nil {
		// Final exit logic is centralized here.
		os.Exit(1)
	}
}

// run orchestrates the koobadm command dispatching.
func run() error {
	// Initialize structured logging with colored console handler.
	slog.SetDefault(slog.New(sysutil.NewConsoleHandler(slog.LevelInfo)))

	cmd := "help"
	if len(os.Args) >= 2 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "init":
		return runInit()
	case "token":
		return runToken()
	case "config":
		return runConfig()
	case "join":
		return runJoin()
	case "version":
		runVersion()
		return nil
	case "help", "--help":
		runHelp()
		return nil
	default:
		slog.Error("Unknown command",
			slog.String("command", cmd),
			slog.String("usage", "koobadm [init|token|join|config|version]"),
		)
		return errors.New("unknown command")
	}
}

// runHelp displays the interactive usage guide and version info
func runHelp() {
	runVersion()
	fmt.Println("\nUsage: koobadm <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  init      Initialize a new control plane node")
	fmt.Println("  join      Join an existing cluster as a worker node")
	fmt.Println("  token     Manage bootstrap tokens (create, list, delete)")
	fmt.Println("  config    Display admin kubeconfig (base64 encoded)")
	fmt.Println("  version   Display koob OS version information")
	fmt.Println("  help      Display this help message")
}

func runVersion() {
	fmt.Printf("koob OS Version: %s\n", version.Version)
	fmt.Printf("Build Time:      %s\n", version.BuildTime)
}

// runToken handles bootstrap token lifecycle (creation/deletion)
func runToken() error {
	if len(os.Args) < 3 || os.Args[2] != "create" {
		fmt.Println("Usage: koobadm token create [--print-join-command]")
		return nil
	}

	printJoin := false
	for _, arg := range os.Args {
		if arg == "--print-join-command" {
			printJoin = true
			break
		}
	}

	if err := token.CreateToken(printJoin); err != nil {
		slog.Error("Error creating token",
			slog.Any("error", err),
		)
		return err
	}
	return nil
}

// runConfig exports the admin kubeconfig in a copy-pasteable base64 format
func runConfig() error {
	path := "/etc/kubernetes/admin.conf"
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Error("Error reading admin.conf",
			slog.String("path", path),
			slog.Any("error", err),
		)
		return err
	}

	encoded := base64.StdEncoding.EncodeToString(data)

	fmt.Println("\n" + strings.Repeat("-", 79))

	// Chunk output to avoid truncation issues
	chunkSize := 76
	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		fmt.Println(encoded[i:end])
	}

	fmt.Println(strings.Repeat("-", 79))

	slog.Info("Copy the block above into your local machine and run:")
	fmt.Println("\ncat > admin.b64 <<EOF")
	fmt.Println("<PASTE BLOCK HERE>")
	fmt.Println("EOF")
	fmt.Println("\nbase64 -d admin.b64 > admin.conf")
	fmt.Println("export KUBECONFIG=$(pwd)/admin.conf")
	fmt.Println("kubectl get nodes")
	return nil
}

// runInit orchestrates the local control plane setup
func runInit() error {
	// Guard against accidental re-initialization
	adminConf := "/etc/kubernetes/admin.conf"
	if _, err := os.Stat(adminConf); err == nil {
		slog.Error("Control plane already initialized",
			slog.String("hint", "Delete /etc/kubernetes to re-init"),
		)
		return errors.New("already initialized")
	}

	slog.Info("Initializing koob OS node...")

	dirs := []string{
		"/etc/kubernetes",
		"/var/lib/kubelet",
	}

	for _, d := range dirs {
		if err := sysutil.Mkdir(d); err != nil {
			slog.Error("Failed to create directory",
				slog.String("path", d),
				slog.Any("error", err),
			)
			return err
		}
	}
	// Detect identity (hostname + IP).
	// koobd ensures networking is ready before this shell/tool is usable
	nodeName, err := os.Hostname()
	if err != nil {
		slog.Warn("Could not determine hostname",
			slog.Any("error", err),
		)
		nodeName = "koob-control-plane"
	}

	nodeIP, err := netutil.DefaultIP()
	if err != nil {
		slog.Warn("Could not detect default IP; falling back to 127.0.0.1",
			slog.String("warning", "External access will be broken"),
			slog.Any("error", err),
		)
		nodeIP = "127.0.0.1"
	}
	slog.Info("Node identity configured",
		slog.String("name", nodeName),
		slog.String("ip", nodeIP),
	)
	slog.Info("Generating PKI artifacts...")
	if err := pki.InitPKI(nodeName, nodeIP); err != nil {
		slog.Error("Failed to generate PKI",
			slog.Any("error", err),
		)
		return err
	}

	// Generate KubeConfigs, Manifests, and Kubelet Config
	cpEndpoint := fmt.Sprintf("https://%s:6443", nodeIP)
	slog.Info("Generating KubeConfigs...",
		slog.String("endpoint", cpEndpoint),
	)
	if err := kubeconfig.InitKubeConfigs(nodeName, nodeIP, cpEndpoint); err != nil {
		slog.Error("Failed to generate KubeConfigs",
			slog.Any("error", err),
		)
		return err
	}

	slog.Info("Generating Static Pod Manifests...")
	if err := manifests.InitManifests(nodeIP); err != nil {
		slog.Error("Failed to generate Manifests",
			slog.Any("error", err),
		)
		return err
	}

	slog.Info("Generating Kubelet Configuration...")
	if err := kubelet.InitKubeletConfig(nodeName); err != nil {
		slog.Error("Failed to generate Kubelet Config",
			slog.Any("error", err),
		)
		return err
	}

	slog.Info("Post-bootstrap: Configuring cluster resources...")

	// Configure RBAC for node discovery and join
	if err := bootstrap.InitBootstrapRBAC(); err != nil {
		slog.Warn("Failed to initialize Bootstrap RBAC",
			slog.Any("error", err),
		)
	}

	if err := addons.ApplyCoreDNS(); err != nil {
		slog.Warn("Failed to apply CoreDNS addon",
			slog.String("hint", "Try re-running init or manual fix"),
			slog.Any("error", err),
		)
	}

	if err := bootstrap.MarkControlPlane(); err != nil {
		slog.Warn("Failed to mark control plane",
			slog.Any("error", err),
		)
	}

	slog.Info("Initialization complete; koobd will now start services")
	return nil
}

// runJoin orchestrates the node handshake with the control plane
func runJoin() error {
	if len(os.Args) < 3 {
		fmt.Println("Usage: koobadm join <endpoint> --token <token> " +
			"--discovery-token-ca-cert-hash <hash>")
		return nil
	}

	endpoint := os.Args[2]
	var tokenStr, caHash string

	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--token" && i+1 < len(os.Args) {
			tokenStr = os.Args[i+1]
			i++
		}
		if os.Args[i] == "--discovery-token-ca-cert-hash" &&
			i+1 < len(os.Args) {
			caHash = os.Args[i+1]
			i++
		}
	}

	if tokenStr == "" || caHash == "" {
		slog.Error("Missing required join parameters",
			slog.String("token", tokenStr),
			slog.String("ca_hash", caHash),
		)
		return errors.New("missing parameters")
	}

	if err := join.Cluster(endpoint, tokenStr, caHash); err != nil {
		slog.Error("Join failed",
			slog.Any("error", err),
		)
		return err
	}
	return nil
}
