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

package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/koob-foo/koob-os/pkg/netutil"
)

// initHardwareNet brings up essential low-level networking components
// like the loopback interface and sets the initial dynamic hostname.
func initHardwareNet() {
	slog.Info("Initializing Hardware Networking...")

	// Bring up Loopback.
	if err := exec.Command("ip", "link", "set", "lo", "up").Run(); err != nil {
		slog.Warn("Failed to bring up lo interface", slog.Any("error", err))
	}

	setDynamicHostname()
}

// setDynamicHostname generates and sets a hostname derived from the primary
// interface's MAC address.
func setDynamicHostname() {
	iface, err := netutil.PrimaryInterface()
	if err != nil {
		slog.Warn("No primary interface found for hostname generation",
			slog.Any("error", err))
		fallbackHostname()
		return
	}

	// Generate from MAC address.
	macPath := fmt.Sprintf("/sys/class/net/%s/address", iface.Name)
	// #nosec G304 - Primary interface path is trusted in early-boot env.
	data, err := os.ReadFile(macPath)
	if err != nil {
		slog.Warn("Failed to read MAC address",
			slog.String("interface", iface.Name),
			slog.Any("error", err))
		fallbackHostname()
		return
	}

	// Clean MAC string: 52:54:00:12:34:56 -> 525400123456.
	macClean := ""
	for _, c := range string(data) {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') ||
			(c >= 'A' && c <= 'F') {
			macClean += string(c)
		}
	}

	suffix := macClean
	if len(macClean) > 6 {
		suffix = macClean[len(macClean)-6:]
	}
	if len(suffix) == 0 {
		suffix = "unknown"
	}

	hostname := "k-" + suffix
	slog.Info("Generated dynamic hostname",
		slog.String("hostname", hostname),
		slog.String("mac", macClean))

	if err := syscall.Sethostname([]byte(hostname)); err != nil {
		slog.Error("Failed to set hostname",
			slog.String("hostname", hostname),
			slog.Any("error", err))
	}
}

// fallbackHostname sets a random hostname when dynamic generation fails.
func fallbackHostname() {
	hostname := "k-" + fmt.Sprintf("%d", time.Now().UnixNano()%100000000)
	slog.Info("Using fallback random hostname",
		slog.String("hostname", hostname))

	if err := syscall.Sethostname([]byte(hostname)); err != nil {
		slog.Error("Failed to set fallback hostname",
			slog.String("hostname", hostname),
			slog.Any("error", err))
	}
}

// configureDynamicNet attempts to acquire an IP address via DHCP and
// populates /etc/hosts upon success.
func configureDynamicNet() {
	slog.Info("Configuring dynamic networking...")

	iface, err := netutil.PrimaryInterface()
	if err != nil {
		slog.Warn("No primary interface found for DHCP", slog.Any("error", err))
		slog.Info("Continuing boot sequence without network.")
		return
	}

	// Fire DHCP (Async).
	slog.Info("Requesting DHCP lease", slog.String("interface", iface.Name))

	dhcpStarted := true
	// #nosec G204 - Interface name comes from trusted net.Interfaces().
	if err := exec.Command("dhclient", "-v", iface.Name).Start(); err != nil {
		slog.Warn("DHCP launch failure",
			slog.String("interface", iface.Name),
			slog.Any("error", err))
		dhcpStarted = false
	}

	// Wait for valid IP.
	var ip string
	if dhcpStarted {
		slog.Info("Waiting for IP address (60s timeout)...")

		// Loop: check every 5s for total of 60s.
		for i := 0; i < 12; i++ {
			if detected, err := netutil.DefaultIP(); err == nil {
				ip = detected
				break
			}
			time.Sleep(5 * time.Second)
		}
	}

	if ip != "" {
		slog.Info("Network interface ready", slog.String("ip", ip))
		hostname, err := os.Hostname()
		if err != nil {
			slog.Warn("Could not determine hostname for /etc/hosts",
				slog.Any("error", err))
			hostname = "localhost"
		}
		hostsContent := fmt.Sprintf("127.0.0.1\tlocalhost\n"+
			"::1\t\tlocalhost\n%s\t%s.internal\t%s\n",
			ip, hostname, hostname)

		if err := os.WriteFile("/etc/hosts", []byte(hostsContent), 0600); err != nil {
			slog.Error("Failed to write /etc/hosts",
				slog.Any("error", err))
		} else {
			slog.Info("Synchronized /etc/hosts")
		}
	} else {
		slog.Warn("Failed to acquire IP address; continuing without network.")
	}
}
