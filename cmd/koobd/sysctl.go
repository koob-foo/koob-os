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

	"github.com/koob-foo/koob-os/pkg/netutil"
)

// configureSysctls sets kernel parameters required for Kubernetes networking
// and container operation.
func configureSysctls() {
	sysctls := map[string]string{
		"/proc/sys/net/ipv4/ip_forward":         "1",
		"/proc/sys/net/ipv4/conf/all/rp_filter": "0",
	}

	// Add primary interface specific rp_filter.
	if iface, err := netutil.PrimaryInterface(); err == nil {
		path := fmt.Sprintf("/proc/sys/net/ipv4/conf/%s/rp_filter", iface.Name)
		sysctls[path] = "0"
	}

	slog.Info("Configuring Kernel Sysctls...")

	for path, value := range sysctls {
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			slog.Error("Failed to set kernel parameter",
				slog.String("path", path),
				slog.String("value", value),
				slog.Any("error", err))
		}
	}
}
