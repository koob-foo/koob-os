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

# This script combines the Kernel, Initramfs, Command Line, and EFI Stub
# into a single signed .efi binary (Unified Kernel Image).
# Requires: systemd-boot, binutils (objcopy), and sbsigntool.

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

# Configuration
KERNEL="bzImage"
INITRD="initramfs.cpio"
OUTPUT="koob-os.efi"
CMDLINE_FILE="config/cmdline.txt"
OS_RELEASE="config/os-release"

# Sanity checks
if [ ! -f "$KERNEL" ] || [ ! -f "$INITRD" ]; then
  MSG="Missing kernel ($KERNEL) or initramfs ($INITRD)."
  printf "%b[ FAIL ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
  exit 1
fi

# Find the UEFI stub
STUB=""
STUB_PATHS="
  /usr/lib/systemd/boot/efi/linuxx64.efi.stub
  /usr/lib/systemd/boot/efi/linuxx64.elf.stub
  /lib/systemd/boot/efi/linuxx64.efi.stub
"

for path in $STUB_PATHS; do
  if [ -f "$path" ]; then
    STUB="$path"
    break
  fi
done

if [ -z "$STUB" ]; then
  # Last resort: search ONLY for .stub files
  STUB=$(find /usr/lib/systemd/boot/efi -name "*.stub" 2>/dev/null | head -n 1)
fi

if [ -z "$STUB" ] || [ ! -f "$STUB" ]; then
  MSG="UEFI Stub (.stub) NOT found in systemd boot directories."
  printf "%b[ FAIL ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
  exit 1
fi

# Kernel command line
if [ ! -f "$CMDLINE_FILE" ]; then
  MSG="$CMDLINE_FILE not found, using default."
  printf "%b[ WARN ]%b %s\n" "${WARN_COLOR}" "${NC}" "$MSG"
  # Default command line
  CMDLINE="console=tty0 console=ttyS0 \
    net.ifnames=0 random.trust_cpu=on panic=10"
  printf "   %s\n" "$CMDLINE"
  printf "%s" "$CMDLINE" > "$CMDLINE_FILE"
fi

# Clean up cmdline: strip comments and newlines, then trim
CLEAN_CMDLINE=$(grep -v '^#' "$CMDLINE_FILE" | tr '\n' ' ' | xargs)

# Build UKI
if command -v ukify >/dev/null 2>&1; then
  MSG="Building Unified Kernel Image (UKI) using ukify..."
  printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

  set -- build \
    --linux="$KERNEL" \
    --initrd="$INITRD" \
    --cmdline="$CLEAN_CMDLINE" \
    --os-release="@$OS_RELEASE" \
    --stub="$STUB" \
    --output="$OUTPUT"

  ukify "$@"
else
  MSG="ukify not found. Falling back to simple objcopy..."
  printf "%b[ WARN ]%b %s\n" "${WARN_COLOR}" "${NC}" "$MSG"

  # For objcopy, we need a temporary file with the clean cmdline
  mkdir -p tmp
  printf "%s" "$CLEAN_CMDLINE" > tmp/cmdline.clean

  set -- \
    --add-section .osrel="$OS_RELEASE" --change-section-vma .osrel=0x20000 \
    --add-section .cmdline=tmp/cmdline.clean \
    --change-section-vma .cmdline=0x30000 \
    --add-section .linux="$KERNEL" --change-section-vma .linux=0x2000000 \
    --add-section .initrd="$INITRD" --change-section-vma .initrd=0x4000000 \
    "$STUB" "$OUTPUT"

  objcopy "$@"
fi

MSG="Generated UKI: $OUTPUT"
printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"

# Secure Boot signing
# Priority 1: Persistent Sovereign Keys (~/.koob/keys)
# Priority 2: Local Workspace Keys (tmp/secure-boot/keys)
DB_KEY="$HOME/.koob/keys/db.key"
DB_CERT="$HOME/.koob/keys/db.crt"

if [ ! -f "$DB_KEY" ]; then
  DB_KEY="tmp/secure-boot/keys/db.key"
  DB_CERT="tmp/secure-boot/keys/db.crt"
fi

if [ -f "$DB_KEY" ]; then
  MSG="Signing UKI using sovereign keys..."
  printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

  sbsign --key "$DB_KEY" --cert "$DB_CERT" --output "$OUTPUT" "$OUTPUT"

  MSG="UKI successfully signed: $OUTPUT"
  printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
else
  MSG="No Secure Boot keys found. UKI remains unsigned."
  printf "%b[ WARN ]%b %s\n" "${WARN_COLOR}" "${NC}" "$MSG"
fi
