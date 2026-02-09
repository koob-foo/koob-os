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

// Package netutil provides common networking utilities for Koob OS components.
package netutil

import (
	"fmt"
	"net"
	"strings"
)

// DefaultIP detects the first non-loopback IPv4 address on the system.
// It prioritizes interfaces starting with "eth" or "en".
func DefaultIP() (string, error) {
	iface, err := PrimaryInterface()
	if err != nil {
		return "", err
	}
	return firstIPv4(iface)
}

// PrimaryInterface returns the first non-loopback interface starting
// with "eth" or "en".
func PrimaryInterface() (*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces: %w", err)
	}

	for i := range ifaces {
		iface := &ifaces[i]
		if (iface.Flags & net.FlagLoopback) != 0 {
			continue
		}
		if strings.HasPrefix(iface.Name, "eth") ||
			strings.HasPrefix(iface.Name, "en") {
			return iface, nil
		}
	}

	return nil, fmt.Errorf("no primary network interface (eth/en) found")
}

// firstIPv4 returns the first non-loopback IPv4 address for a given interface.
func firstIPv4(iface *net.Interface) (string, error) {
	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		if ip4 := ipnet.IP.To4(); ip4 != nil {
			return ip4.String(), nil
		}
	}

	return "", fmt.Errorf("no IPv4 address found for %s", iface.Name)
}
