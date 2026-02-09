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

# This script packages the signed Unified Kernel Image (UKI) into a
# bootable UEFI ISO image. It prepares an EFI System Partition (FAT)
# and generates the final ISO using xorriso.
# Requires: xorriso, mtools, and build-uki.sh.

# ANSI Color codes
OK_COLOR='\033[1;32m'
INFO_COLOR='\033[1;36m'
NC='\033[0m' # No Color

# Resolve absolute paths and anchor to the repo root
SCRIPT_DIR=$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)
cd "$REPO_ROOT"

# Ephemeral resource management
ISO_STAGING=$(mktemp -d)
EFI_STAGING=$(mktemp -d)
trap 'rm -rf "$ISO_STAGING" "$EFI_STAGING"' EXIT

# Configuration
ISO_NAME="koob-os.iso"

# The ISO image is the final product and should always be generated
# to ensure it contains the latest UKI and enrollment artifacts.
rm -f "$ISO_NAME"
# (Skipping idempotency check as per user request to ensure fresh media)

EFI_IMG="$ISO_STAGING/efi.img"

# Build the UKI
sh hack/build-uki.sh

# Prepare the EFI System Partition image (FAT)
MSG="Generating EFI System Partition image..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

# Calculate size dynamically: UKI size + 6MB buffer
UKI_SIZE_BYTES=$(wc -c < "koob-os.efi")
EFI_SIZE_MB=$(( (UKI_SIZE_BYTES / 1024 / 1024) + 6 ))

MSG="Target efi.img size: ${EFI_SIZE_MB}MB"
printf "   %s\n" "$MSG"
dd if=/dev/zero of="$EFI_IMG" bs=1M count="$EFI_SIZE_MB" status=none
/usr/sbin/mkfs.vfat "$EFI_IMG" >/dev/null

# Embed the UKI into the FAT image
MSG="Embedding Unified Kernel Image into EFI partition..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

mkdir -p "$EFI_STAGING/EFI/BOOT"
cp koob-os.efi "$EFI_STAGING/EFI/BOOT/BOOTX64.EFI"
mcopy -s -i "$EFI_IMG" "$EFI_STAGING/EFI" ::/

# Finalize the bootable UEFI ISO
MSG="Finalizing bootable UEFI ISO..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

{
  echo "koob OS - Monolithic UKI Boot Media"
  echo "======================================"
  echo "This media contains a signed Unified Kernel Image (UKI)."
  echo ""
  echo "Project: https://koob.foo"
  echo "Source:  https://github.com/koob-foo/koob-os"
} > "$ISO_STAGING/README.txt"

# Generate the final ISO using xorriso.
# We use positional parameters to build the command safely with proper quoting.
set -- \
  -as mkisofs \
  -iso-level 3 \
  -o "$ISO_NAME" \
  -full-iso9660-filenames \
  -volid "koob OS" \
  -eltorito-alt-boot \
  -e efi.img \
  -no-emul-boot \
  -isohybrid-gpt-basdat \
  "$ISO_STAGING"

xorriso "$@"

MSG="ISO successfully generated: $ISO_NAME"
printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"

# Deployment Summary
ISO_SIZE_MB=$(find "./$ISO_NAME" -printf '%s\n' | \
  awk '{printf "%.2fMB\n", $1/1024/1024}')
echo ""
echo "ISO Information:"
echo "   Path: $REPO_ROOT/$ISO_NAME"
echo "   Size: $ISO_SIZE_MB"
