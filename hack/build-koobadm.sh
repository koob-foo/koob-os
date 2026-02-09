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

# This script compiles the koobadm CLI for cluster initialization and
# management. It produces a statically linked Go binary.
# Requires: go.

# ANSI Color codes
OK_COLOR='\033[1;32m'
INFO_COLOR='\033[1;36m'
NC='\033[0m' # No Color

# Resolve absolute paths and anchor to the repo root
SCRIPT_DIR=$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)
cd "$REPO_ROOT"

# Configuration
DEST_BIN="tmp/rootfs/bin"

# Idempotency check
if [ -f "$DEST_BIN/koobadm" ]; then
  # In CI, we trust the existence of the binary if the cache hit
  if [ -n "$GITHUB_ACTIONS" ]; then
    MSG="koobadm restored from cache."
    printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
    exit 0
  fi

  MSG="Skipping building koobadm; binary already present in rootfs."
  printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
  exit 0
fi

MSG="Compiling koobadm CLI..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

mkdir -p "$DEST_BIN"
BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_HASH=$(git rev-parse --short HEAD || echo "unknown")
PKG="github.com/koob-foo/koob-os/pkg/version"
LDFLAGS="-s -w -X '${PKG}.Version=${VERSION:-dev}'"
LDFLAGS="${LDFLAGS} -X '${PKG}.BuildTime=${BUILD_TIME}'"
LDFLAGS="${LDFLAGS} -X '${PKG}.GitCommit=${GIT_HASH}'"

CGO_ENABLED=0 go build -ldflags="$LDFLAGS" -o "$DEST_BIN/koobadm" ./cmd/koobadm

MSG="koobadm binary installed to $DEST_BIN"
printf "%b[ SUCCESS ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
