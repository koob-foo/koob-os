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

# This script prepares the development environment for building koob OS.
# It installs dependencies and configures script permissions.
# Currently supports: Debian and Ubuntu.

# ANSI Color codes
OK_COLOR='\033[1;32m'
INFO_COLOR='\033[1;36m'
WARN_COLOR='\033[1;33m'
FAIL_COLOR='\033[1;31m'
NC='\033[0m' # No Color

# Resolve absolute paths and anchor to the repo root
SCRIPT_DIR=$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)
cd "$REPO_ROOT"

MSG="Setting up koob OS build environment..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

# Script permissions and RootFS skeleton
MSG="Configuring script permissions and RootFS..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"
chmod +x hack/*.sh examples/libvirt/*.sh
sh hack/init-rootfs.sh

# OS detection
if [ -f /etc/os-release ]; then
  # shellcheck disable=SC1091
  . /etc/os-release
  OS=$ID
else
  MSG="Cannot detect OS; /etc/os-release not found."
  printf "%b[ FAIL ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
  exit 1
fi

# Dependency installation
MSG="Verifying build dependencies..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

if [ "$OS" = "debian" ] || [ "$OS" = "ubuntu" ]; then
  MSG="Detected $OS $VERSION_ID"
  printf "   %s\n" "$MSG"

  # Update package indices
  MSG="Updating package indices..."
  printf "   %s\n" "$MSG"
  sudo apt-get update -qq

  # Core build packages
  PKGS="build-essential git bison flex libelf-dev libssl-dev bc gawk wget"
  PKGS="$PKGS curl squashfs-tools xorriso qemu-kvm libvirt-daemon-system"
  PKGS="$PKGS libvirt-clients bridge-utils sbsigntool efitools uuid-runtime"
  PKGS="$PKGS dwarves mtools dosfstools systemd-boot ovmf"

  # Add version-specific packages for modern UKI builds
  MAJOR_VER=$(echo "$VERSION_ID" | cut -d. -f1)
  if [ "$OS" = "debian" ] && [ "$MAJOR_VER" -ge 13 ]; then
    PKGS="$PKGS systemd-boot-efi systemd-ukify"
  elif [ "$OS" = "ubuntu" ] && [ "$MAJOR_VER" -ge 24 ]; then
    PKGS="$PKGS systemd-boot-efi systemd-ukify"
  fi

  MSG="Installing required packages..."
  printf "   %s\n" "$MSG"
  # shellcheck disable=SC2086
  sudo apt-get install -y $PKGS

  MSG="Dependencies successfully installed."
  printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
else
  MSG="Unsupported OS: $OS. This script supports Debian/Ubuntu only."
  printf "%b[ FAIL ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
  exit 1
fi

# Compiler verification (Go)
if command -v go >/dev/null 2>&1; then
  GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
  MSG="Go version detected: $GO_VERSION"
  printf "   %b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"

  # Check minimum version requirement (1.21+)
  GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
  GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)
  if [ "$GO_MAJOR" -lt 1 ] || \
     { [ "$GO_MAJOR" -eq 1 ] && [ "$GO_MINOR" -lt 21 ]; }; then
    MSG="Go 1.21+ recommended for reliable builds."
    printf "   %b[ WARN ]%b %s\n" "${WARN_COLOR}" "${NC}" "$MSG"
  fi
else
  MSG="Go compiler not found. Please install Go 1.24 or later."
  printf "%b[ FAIL ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
  exit 1
fi

# Virtualization services
MSG="Configuring virtualization services..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"
sudo systemctl enable --now libvirtd
MSG="libvirt service enabled and active."
printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"

MSG="koob OS environment setup complete"
printf "\n%b[ SUCCESS ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
