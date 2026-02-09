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

# This script performs an end-to-end (E2E) boot test of koob OS using QEMU.
# It validates the UEFI boot sequence and core process supervision (koobd).
# Requires: qemu-system-x86_64 and ovmf.

# ANSI Color codes
OK_COLOR='\033[1;32m'
FAIL_COLOR='\033[1;31m'
INFO_COLOR='\033[1;36m'
NC='\033[0m' # No Color

# Resolve absolute paths and anchor to the repo root
SCRIPT_DIR=$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)
cd "$REPO_ROOT"

# Configuration
ISO_IMAGE="koob-os.iso"
SERIAL_LOG="tmp/serial.log"
TIMEOUT=120

if [ ! -f "$ISO_IMAGE" ]; then
  MSG="ISO image not found: $ISO_IMAGE. Run 'make iso' first."
  printf "%b[ FAIL ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
  exit 1
fi

mkdir -p tmp
rm -f "$SERIAL_LOG"
touch "$SERIAL_LOG"

MSG="Launching E2E Boot Test (Timeout: ${TIMEOUT}s)..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

# Locate UEFI firmware (OVMF)
OVMF_PATH=""
for path in \
  "/usr/share/OVMF/OVMF_CODE.fd" \
  "/usr/share/ovmf/OVMF.fd" \
  "/usr/share/qemu/OVMF.fd" \
  "/usr/share/OVMF/OVMF_CODE_4M.fd"; do
  if [ -f "$path" ]; then
    OVMF_PATH="$path"
    break
  fi
done

if [ -z "$OVMF_PATH" ]; then
  MSG="OVMF firmware not found. Please install the 'ovmf' package."
  printf "%b[ FAIL ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
  exit 1
fi

MSG="Using UEFI Firmware: $OVMF_PATH"
printf "   %s\n" "$MSG"

# Orchestrate QEMU execution
# -nographic: No VGA output
# -serial file: Redirect serial output to log for parsing
# -m 2G: Minimum RAM for Kubernetes components
set -- \
  -nographic \
  -m 2G \
  -machine q35 \
  -bios "$OVMF_PATH" \
  -netdev user,id=net0 -device virtio-net-pci,netdev=net0 \
  -cdrom "$ISO_IMAGE" \
  -serial file:"$SERIAL_LOG" \
  -boot d

qemu-system-x86_64 "$@" &
QEMU_PID=$!

# Ensure QEMU is terminated on script exit
trap 'kill $QEMU_PID >/dev/null 2>&1 || true' EXIT

MSG="Waiting for kernel boot sequence..."
printf "   %s\n" "$MSG"

# Log parsing and success verification
START_TIME=$(date +%s)

# Stage A: Kernel/Initramfs Hand-off
while true; do
  if grep -q "boot sequence starting" "$SERIAL_LOG" 2>/dev/null; then
    MSG="Boot sequence detected successfully."
    printf "  %b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
    break
  fi

  ELAPSED=$(($(date +%s) - START_TIME))
  if [ "$ELAPSED" -ge "$TIMEOUT" ]; then
    MSG="Boot timeout reached after ${TIMEOUT}s."
    printf "  %b[ FAIL ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
    printf "   Last logs:\n"
    tail -n 15 "$SERIAL_LOG"
    exit 1
  fi
  sleep 1
done

# Stage B: Process Supervision (koobd -> containerd)
MSG="Waiting for process supervision (containerd)..."
printf "   %s\n" "$MSG"

while true; do
  if grep -q "\[containerd\] .* Starting immediately" "$SERIAL_LOG" 2>/dev/null
  then
    MSG="Supervision active: containerd launched successfully!"
    printf "  %b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
    break
  fi

  ELAPSED=$(($(date +%s) - START_TIME))
  if [ "$ELAPSED" -ge "$TIMEOUT" ]; then
    MSG="Supervision timeout reached after ${TIMEOUT}s."
    printf "  %b[ FAIL ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
    printf "   Last logs:\n"
    tail -n 15 "$SERIAL_LOG"
    exit 1
  fi
  sleep 1
done

MSG="E2E Boot & Supervision Test passed"
printf "\n%b[ SUCCESS ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
exit 0
