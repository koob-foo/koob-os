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

# This script compresses the root filesystem into a read-only SquashFS
# image. It injects version metadata and performs basic sanity checks.
# Requires: squashfs-tools.

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
ROOTFS="tmp/rootfs"
IMAGE_NAME="rootfs.squashfs"

# Idempotency check
if [ -f "$IMAGE_NAME" ]; then
  # In CI, we trust the existence of the image if the cache hit
  if [ -n "$GITHUB_ACTIONS" ]; then
    MSG="Restoring SquashFS image from cache."
    printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
    exit 0
  fi

  MSG="Skipping SquashFS build; $IMAGE_NAME already exists."
  printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
  exit 0
fi

MSG="Building SquashFS Root Filesystem..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

# Sanity checks
if [ ! -d "$ROOTFS" ]; then
  MSG="Rootfs directory not found. Run 'make setup-environment' first."
  printf "%b[ FAIL ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
  exit 1
fi

if [ ! -f "$ROOTFS/bin/koobd" ]; then
  MSG="koobd not found in rootfs. Run 'make binaries' first."
  printf "%b[ FAIL ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
  exit 1
fi

# Inject version metadata
MSG="Injecting koob OS version metadata..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

mkdir -p "$ROOTFS/etc"
VERSION="${VERSION:-dev}"
echo "$VERSION" > "$ROOTFS/etc/koob-os-version"

{
  echo "NAME=\"koob OS\""
  echo "VERSION=\"$VERSION\""
  echo "ID=koob-os"
  echo "PRETTY_NAME=\"koob OS $VERSION\""
  echo "BUILD_ID=\"$(date +%Y%m%d%H%M%S)\""
} > "$ROOTFS/etc/os-release"

# Compress to SquashFS
MSG="Compressing rootfs to SquashFS (zstd)..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

# Use positional parameters to build the command safely
set -- \
  "$ROOTFS" \
  "$IMAGE_NAME" \
  -comp zstd \
  -Xcompression-level 19 \
  -noappend \
  -no-progress

mksquashfs "$@"

MSG="SquashFS build complete: $IMAGE_NAME"
printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"

# Size reporting
ROOT_SIZE=$(du -sh "$ROOTFS" | awk '{print $1}')
IMG_SIZE=$(find "./$IMAGE_NAME" -printf '%s\n' | \
  awk '{printf "%.2fMB\n", $1/1024/1024}')

echo ""
echo "Summary:"
echo "   Source:     $ROOTFS ($ROOT_SIZE)"
echo "   Artifact:   $REPO_ROOT/$IMAGE_NAME ($IMG_SIZE)"
