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

# This script compiles a static multicall iptables binary for the koob OS
# root filesystem. It downloads the source, configures a hermetic
# static build, and deploys the binary to the staging area.
# Requires: curl, build-essential, tar.

# ANSI Color codes
OK_COLOR='\033[1;32m'
INFO_COLOR='\033[1;36m'
NC='\033[0m' # No Color

# Resolve absolute paths and anchor to the repo root
SCRIPT_DIR=$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)
cd "$REPO_ROOT"

# Ephemeral resource management
BUILD_DIR=$(mktemp -d)
trap 'rm -rf "$BUILD_DIR"' EXIT

# Configuration
VERSION="1.8.11"
DEST_BIN="tmp/rootfs/bin"
URL="https://www.netfilter.org/pub/iptables/iptables-${VERSION}.tar.xz"

# Idempotency check
if [ -f "$DEST_BIN/iptables" ]; then
  # In CI, we trust the existence of the binary if the cache hit
  if [ -n "$GITHUB_ACTIONS" ]; then
    MSG="Iptables restored from cache."
    printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
    exit 0
  fi

  MSG="Skipping iptables build; binary already present in rootfs."
  printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
  exit 0
fi

MSG="Compiling static iptables binary..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

MSG="Downloading iptables $VERSION..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"
curl -L --progress-bar -o "$BUILD_DIR/iptables.tar.xz" "$URL"

MSG="Extracting source..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"
tar -xf "$BUILD_DIR/iptables.tar.xz" -C "$BUILD_DIR" --strip-components=1

cd "$BUILD_DIR"

MSG="Configuring for static build..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

# Build with static linking and minimal features for the initramfs
LDFLAGS="-static"
CFLAGS="-O2"
./configure --enable-static --disable-shared --disable-nftables \
  --prefix="$(pwd)/install" LDFLAGS="$LDFLAGS" CFLAGS="$CFLAGS"

JOBS=$(nproc 2>/dev/null || echo 1)
MSG="Compiling source (Jobs: $JOBS)..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"
make -j"$JOBS"

# Standardize the destination path relative to the repo root
mkdir -p "$REPO_ROOT/$DEST_BIN"
cp iptables/xtables-legacy-multi "$REPO_ROOT/$DEST_BIN/iptables"
strip --strip-debug "$REPO_ROOT/$DEST_BIN/iptables" 2>/dev/null || true

MSG="Iptables installed to $DEST_BIN"
printf "%b[ SUCCESS ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
