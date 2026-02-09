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

# This script builds the monolithic koob OS initramfs by incorporating the
# target SquashFS root filesystem into a u-root CPIO archive.
# Requires: go, u-root, and rootfs.squashfs.

# ANSI Color codes
OK_COLOR='\033[1;32m'
INFO_COLOR='\033[1;36m'
FAIL_COLOR='\033[1;31m'
NC='\033[0m' # No Color

# Resolve absolute paths and anchor to the repo root
SCRIPT_DIR=$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)
cd "$REPO_ROOT"

# Idempotency check
if [ -f "initramfs.cpio" ]; then
  MSG="Skipping building initramfs; initramfs.cpio already exists."
  printf "%b[ SKIP ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
  exit 0
fi

# Verify the SquashFS root image is present
if [ ! -f "rootfs.squashfs" ]; then
  MSG="rootfs.squashfs not found! Monolithic builds require the root image."
  printf "%b[ FAIL ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
  echo "         Please run 'make squashfs' first."
  exit 1
fi

# Build the monolithic initramfs
MSG="Incorporating rootfs.squashfs into initramfs..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

go tool github.com/u-root/u-root \
  -o initramfs.cpio \
  -uinitcmd="" \
  -defaultsh="" \
  -initcmd="squashfs-init" \
  -files rootfs.squashfs:rootfs.squashfs \
  ./cmd/squashfs-init \
  github.com/u-root/u-root/cmds/core/mount \
  github.com/u-root/u-root/cmds/core/switch_root \
  github.com/u-root/u-root/cmds/core/losetup \
  github.com/u-root/u-root/cmds/core/ls \
  github.com/u-root/u-root/cmds/core/dmesg \
  github.com/u-root/u-root/cmds/core/cat \
  github.com/u-root/u-root/cmds/core/ps \
  github.com/u-root/u-root/cmds/core/kill \
  github.com/u-root/u-root/cmds/core/sleep

MSG="Monolithic initramfs build complete."
printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
echo "   Artifact: initramfs.cpio"
