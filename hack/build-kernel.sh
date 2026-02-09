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

# This script clones and builds the Linux kernel from source using a
# specific configuration. It produces a compressed kernel image (bzImage).
# Requires: build-essential, git, bison, flex, libssl-dev, and bc.

# ANSI Color codes
OK_COLOR='\033[1;32m'
INFO_COLOR='\033[1;36m'
FAIL_COLOR='\033[1;31m'
NC='\033[0m' # No Color

# Resolve absolute paths and anchor to the repo root
SCRIPT_DIR=$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)
cd "$REPO_ROOT"

# Configuration
VERSION="linux-6.6.y"
KERNEL_SRC="tmp/kernel-src"
KERNEL_URL="https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git"
CONFIG_FILE="config/kernel-config-amd64"

# Verify the kernel configuration is present
if [ ! -f "$CONFIG_FILE" ]; then
  MSG="Kernel configuration not found: $CONFIG_FILE"
  printf "%b[ FAIL ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
  exit 1
fi

# Idempotency check
if [ -f "bzImage" ]; then
  # In CI, we trust the existence of the binary if the cache hit
  if [ -n "$GITHUB_ACTIONS" ]; then
    MSG="Kernel image restored from cache."
    printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
    exit 0
  fi

  MSG="Skipping kernel build; bzImage already exists."
  printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
  exit 0
fi

MSG="Starting koob OS kernel build..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

# Fetch or verify the kernel source tree
if [ ! -d "$KERNEL_SRC" ]; then
  MSG="Fetching Linux $VERSION source..."
  printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"
  mkdir -p "$KERNEL_SRC"
  git clone --depth 1 -b "$VERSION" "$KERNEL_URL" "$KERNEL_SRC"
else
  MSG="Existing kernel source found in $KERNEL_SRC"
  printf "   %b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
fi

cd "$KERNEL_SRC"

# Apply the pinned configuration
MSG="Applying kernel configuration..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"
cp "$REPO_ROOT/$CONFIG_FILE" .config
make olddefconfig

# Build the kernel image
JOBS=$(nproc 2>/dev/null || echo 1)
MSG="Commencing compilation (Jobs: $JOBS)..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"
make -j"$JOBS" bzImage

# Promote the build artifact to the repository root
if [ -f "arch/x86/boot/bzImage" ]; then
  cp arch/x86/boot/bzImage "$REPO_ROOT/bzImage"
  MSG="Kernel successfully promoted to repo root"
  printf "%b[ SUCCESS ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
else
  MSG="Kernel build finished but bzImage not found!"
  printf "%b[ FAIL ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
  exit 1
fi
