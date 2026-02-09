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

# This script builds a BusyBox-style multicall binary containing core
# utilities. It uses u-root to combine multiple Go-based tools into a
# single statically linked binary (bb).
# Requires: go, u-root.

# ANSI Color codes
OK_COLOR='\033[1;32m'
INFO_COLOR='\033[1;36m'
NC='\033[0m' # No Color

# Resolve absolute paths and anchor to the repo root
SCRIPT_DIR=$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)
cd "$REPO_ROOT"

# Ephemeral resource management
DEST_BIN="tmp/rootfs/bin"
BUILD_DIR=$(mktemp -d)
trap 'rm -rf "$BUILD_DIR"' EXIT

# Idempotency check
if [ -f "$DEST_BIN/bb" ]; then
  MSG="Skipping building bb; binary already present in rootfs."
  printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
  exit 0
fi

MSG="Compiling BusyBox-style core utilities (bb)..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

# Build the multicall binary using u-root
# Included tools: file utilities, networking, and process management
go tool github.com/u-root/u-root -build=bb -format=dir \
  -initcmd="" -defaultsh="" -o "$BUILD_DIR" \
  github.com/u-root/u-root/cmds/core/ls \
  github.com/u-root/u-root/cmds/core/cat \
  github.com/u-root/u-root/cmds/core/ps \
  github.com/u-root/u-root/cmds/core/cp \
  github.com/u-root/u-root/cmds/core/grep \
  github.com/u-root/u-root/cmds/core/ip \
  github.com/u-root/u-root/cmds/core/kill \
  github.com/u-root/u-root/cmds/core/mkdir \
  github.com/u-root/u-root/cmds/core/mount \
  github.com/u-root/u-root/cmds/core/mv \
  github.com/u-root/u-root/cmds/core/rm \
  github.com/u-root/u-root/cmds/core/umount \
  github.com/u-root/u-root/cmds/core/which \
  github.com/u-root/u-root/cmds/core/dmesg \
  github.com/u-root/u-root/cmds/core/dhclient \
  github.com/u-root/u-root/cmds/core/gosh

# Deploy the multicall binary and symlinks to the rootfs staging area
mkdir -p "$DEST_BIN"
cp -a "$BUILD_DIR/bbin/"* "$DEST_BIN/"

MSG="Core utilities installed to $DEST_BIN"
printf "%b[ SUCCESS ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
