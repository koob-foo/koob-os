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

# This script verifies the presence and integrity of koob OS build
# deliverables (ISO, UKI, kernel artifacts).
# Requires: sbverify.

# ANSI Color codes
OK_COLOR='\033[1;32m'
FAIL_COLOR='\033[1;31m'
INFO_COLOR='\033[1;36m'
NC='\033[0m' # No Color

# Resolve absolute paths and anchor to the repo root
SCRIPT_DIR=$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)
cd "$REPO_ROOT"

MSG="Starting artifact validation pipeline..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

# Configuration
FAILURES=0

# Helper function to validate file existence
check_file() {
  file="$1"
  desc="$2"
  if [ -f "$file" ]; then
    MSG="$desc found: $file"
    printf "  %b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
  else
    MSG="$desc NOT found: $file"
    printf "  %b[ FAIL ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
    FAILURES=$((FAILURES + 1))
  fi
}

# Core building blocks (Required everywhere)
check_file "bzImage" "Linux Kernel"
check_file "initramfs.cpio" "Initramfs"

# Final deliverables (Required locally, optional in CI)
if [ -n "$GITHUB_ACTIONS" ]; then
  MSG="Running in CI; UKI and ISO are optional building blocks."
  printf "  %b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

  [ -f "koob-os.efi" ] && check_file "koob-os.efi" "Unified Kernel Image (UKI)"
  [ -f "koob-os.iso" ] && check_file "koob-os.iso" "Bootable ISO"
else
  check_file "koob-os.efi" "Unified Kernel Image (UKI)"
  check_file "koob-os.iso" "Bootable ISO"
fi

# Secure Boot infrastructure (Required locally, optional in CI)
# Priority 1: Persistent Sovereign Keys (~/.koob/keys)
# Priority 2: Local Workspace Keys (tmp/secure-boot/keys)
DB_CERT="$HOME/.koob/keys/db.crt"
if [ ! -f "$DB_CERT" ]; then
  DB_CERT="tmp/secure-boot/keys/db.crt"
fi
if [ -n "$GITHUB_ACTIONS" ]; then
  [ -f "$DB_CERT" ] && check_file "$DB_CERT" "Secure Boot DB Certificate"
else
  check_file "$DB_CERT" "Secure Boot DB Certificate"
fi

# Cryptographic signature verification
if [ -f "koob-os.efi" ] && [ -f "$DB_CERT" ]; then
  MSG="Verifying Secure Boot signature on UKI..."
  printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

  if sbverify --cert "$DB_CERT" koob-os.efi >/dev/null 2>&1; then
    MSG="UKI signature verified successfully"
    printf "  %b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
  else
    MSG="UKI signature verification FAILED!"
    printf "  %b[ FAIL ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
    FAILURES=$((FAILURES + 1))
  fi
else
  MSG="Skipping signature verification (missing UKI or DB cert)."
  printf "  %b[ WARN ]%b %s\n" "${NC}" "${NC}" "$MSG"
fi

# Final assessment
if [ "$FAILURES" -eq 0 ]; then
  MSG="All build artifacts validated successfully"
  printf "\n%b[ SUCCESS ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
  exit 0
else
  MSG="Artifact validation failed with $FAILURES error(s)."
  printf "\n%b[ FAILURE ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
  exit 1
fi
