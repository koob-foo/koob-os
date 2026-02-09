#!/bin/sh
# Copyright 2026 Koob Foo {{{
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License. }}}

set -e

# This script initializes the root filesystem (rootfs) staging area by
# creating the necessary FHS-compliant directory structure.
# Requires: mkdir and ln.

# ANSI Color codes
OK_COLOR='\033[1;32m'
INFO_COLOR='\033[1;36m'
NC='\033[0m' # No Color

# Resolve absolute paths and anchor to the repo root
SCRIPT_DIR=$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)
cd "$REPO_ROOT"

# Configuration
ROOTFS="tmp/rootfs"

MSG="Ensuring target RootFS skeleton..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

# Base structure
mkdir -p "$ROOTFS/bin" "$ROOTFS/sbin" "$ROOTFS/etc" "$ROOTFS/var" \
  "$ROOTFS/run" "$ROOTFS/tmp" "$ROOTFS/dev" "$ROOTFS/proc" \
  "$ROOTFS/sys" "$ROOTFS/usr"

# Variable data
mkdir -p "$ROOTFS/var/log" "$ROOTFS/var/lib" "$ROOTFS/var/run"
mkdir -p "$ROOTFS/var/lib/kubelet" "$ROOTFS/var/lib/containerd" \
  "$ROOTFS/var/lib/dhcp"

# Component staging
mkdir -p "$ROOTFS/etc/kubernetes" "$ROOTFS/etc/ssl/certs"
mkdir -p "$ROOTFS/opt/cni/bin"

# /usr substructure
mkdir -p "$ROOTFS/usr/bin" "$ROOTFS/usr/sbin" "$ROOTFS/usr/lib"

# Compatibility symlinks
ln -sf /run "$ROOTFS/var/run" 2>/dev/null || true

MSG="RootFS skeleton successfully created at $ROOTFS"
printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
