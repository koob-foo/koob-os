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

# This script generates custom Secure Boot keys (PK, KEK, db) and prepares
# enrollment artifacts (.auth, .esl, .cer) for UEFI firmware.
# Requires: openssl, efitools, sbsigntool, and uuidgen.

# ANSI Color codes
OK_COLOR='\033[1;32m'
INFO_COLOR='\033[1;36m'
NC='\033[0m' # No Color

# Resolve absolute paths and anchor to the repo root
SCRIPT_DIR=$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)
cd "$REPO_ROOT"

# Configuration
KEY_DIR="tmp/secure-boot/keys"
ENROLL_DIR="tmp/secure-boot/enroll"
PERSIST_DIR="$HOME/.koob/keys"
NAME="KoobOS"

# CI Optimization: Skip key generation in GitHub Actions
if [ "$GITHUB_ACTIONS" = "true" ]; then
  MSG="Running in CI (GitHub Actions). Skipping Secure Boot key generation."
  printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"
  exit 0
fi

mkdir -p "$ENROLL_DIR"

# Resolve authoritative key directory
# Priority 1: Persistent Sovereign Keys (~/.koob/keys)
# Priority 2: Local Workspace Keys (tmp/secure-boot/keys)
if [ -f "$PERSIST_DIR/PK.key" ]; then
  MSG="Using sovereign keys from $PERSIST_DIR"
  printf "   %b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"
  KEY_DIR="$PERSIST_DIR"
else
  mkdir -p "$KEY_DIR"
fi

# Generate keys function
gen_efi_key() {
  key_label="$1"
  target_key="$KEY_DIR/${key_label}.key"
  target_crt="$KEY_DIR/${key_label}.crt"

  if [ -f "$target_key" ]; then
    MSG="Pre-existing ${key_label} detected."
    printf "   %b[ SKIP ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
  else
    MSG="Generating ${key_label}..."
    printf "   %s\n" "$MSG"
    set -- req -new -x509 -newkey rsa:2048 -subj "/CN=$NAME $key_label/" \
      -keyout "$target_key" -out "$target_crt" \
      -days 3650 -nodes -sha256
    openssl "$@" 2>/dev/null
    chmod 600 "$target_key"
  fi
}

for label in PK KEK db; do
  gen_efi_key "$label"
done

# Enrollment artifact preparation
MSG="Preparing EFI enrollment artifacts..."
printf "   %s\n" "$MSG"

# Convert certificates to EFI signature lists (ESL)
for label in PK KEK db; do
  cert-to-efi-sig-list -g "$(uuidgen)" \
    "$KEY_DIR/${label}.crt" "$ENROLL_DIR/${label}.esl" >/dev/null
done

# Sign EFI signature lists to create Authenticated Variables (.auth)
sign-efi-sig-list -k "$KEY_DIR/PK.key" -c "$KEY_DIR/PK.crt" \
  PK "$ENROLL_DIR/PK.esl" "$ENROLL_DIR/PK.auth" >/dev/null

sign-efi-sig-list -k "$KEY_DIR/PK.key" -c "$KEY_DIR/PK.crt" \
  KEK "$ENROLL_DIR/KEK.esl" "$ENROLL_DIR/KEK.auth" >/dev/null

sign-efi-sig-list -k "$KEY_DIR/KEK.key" -c "$KEY_DIR/KEK.crt" \
  db "$ENROLL_DIR/db.esl" "$ENROLL_DIR/db.auth" >/dev/null

# Generate binary DER certificates for specific BIOS compatibility
for label in PK KEK db; do
  openssl x509 -in "$KEY_DIR/${label}.crt" -outform DER \
    -out "$ENROLL_DIR/${label}.cer"
done

MSG="Secure Boot enrollment files ready."
printf "%b[ SUCCESS ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"

MSG="Enrollment data path: $REPO_ROOT/$ENROLL_DIR"
printf "   %s\n" "$MSG"

if [ "$KEY_DIR" != "$PERSIST_DIR" ]; then
  MSG="Tip: To persist your keys across clean builds, run:"
  printf "\n%b[ TIP ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"
  printf "   mkdir -p %s && cp %s/*.key %s/*.crt %s/\n" \
    "$PERSIST_DIR" "$KEY_DIR" "$KEY_DIR" "$PERSIST_DIR"
fi
