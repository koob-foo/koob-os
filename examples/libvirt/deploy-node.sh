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

# This script deploys a koob OS node to libvirt. It handles ISO storage,
# Secure Boot key enrollment disks, and VM lifecycle management.
# Requires: libvirt, virt-install, mtools, and ovmf.

# ANSI Color codes
OK_COLOR='\033[1;32m'
INFO_COLOR='\033[1;36m'
WARN_COLOR='\033[1;33m'
FAIL_COLOR='\033[1;31m'
NC='\033[0m' # No Color

# Resolve absolute paths and anchor to the repo root
SCRIPT_DIR=$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/../.." >/dev/null 2>&1 && pwd)
cd "$REPO_ROOT"

# Configuration
ISO_PATH="/var/lib/libvirt/images/koob-os.iso"
LOCAL_ISO="$REPO_ROOT/koob-os.iso"

usage() {
  echo "Usage: $0 [NODE_NAME] [--insecure]"
  echo ""
  echo "Arguments:"
  echo "  NODE_NAME   Name of the VM (default: koob-node)"
  echo "  --insecure  Boot in legacy BIOS mode (disables Secure Boot)"
  echo ""
  echo "Note: This script requires sudo privileges to interact with libvirt and create disk images."
  echo "You may be prompted for your password."
  exit 1
}

# Defaults
VM_NAME="koob-node"
INSECURE_MODE=false

# Positional and flag parsing
for arg in "$@"; do
  case "$arg" in
    --insecure) INSECURE_MODE=true ;;
    -h|--help)  usage ;;
    *)          VM_NAME="$arg" ;;
  esac
done

if [ "$INSECURE_MODE" = true ]; then
  MSG="Running in INSECURE mode (legacy BIOS/no Secure Boot)"
  printf "%b[ WARN ]%b %s\n" "${WARN_COLOR}" "${NC}" "$MSG"
fi

if [ ! -f "$LOCAL_ISO" ]; then
  MSG="Local ISO not found at $LOCAL_ISO. Run 'make iso' first."
  printf "%b[ FAIL ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
  exit 1
fi

# Detect libvirt environment (priority for Debian/Ubuntu)
LIBVIRT_USER="libvirt-qemu"
LIBVIRT_GROUP="kvm"
SHOULD_CHOWN=false

if getent passwd "$LIBVIRT_USER" >/dev/null && \
   getent group "$LIBVIRT_GROUP" >/dev/null; then
  SHOULD_CHOWN=true
fi

# Updating Libvirt image store
sudo cp "$LOCAL_ISO" "$ISO_PATH"
sudo chmod 644 "$ISO_PATH"
if [ "$SHOULD_CHOWN" = true ]; then
  sudo chown "$LIBVIRT_USER:$LIBVIRT_GROUP" "$ISO_PATH" 2>/dev/null || true
fi

MSG="Deploying $VM_NAME to Libvirt..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

# Cleanup old VM
if sudo virsh dominfo "$VM_NAME" >/dev/null 2>&1; then
  MSG="Destroying existing domain $VM_NAME..."
  printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"
  sudo virsh destroy "$VM_NAME" >/dev/null 2>&1 || true
  sudo virsh undefine "$VM_NAME" --nvram >/dev/null 2>&1 || true
fi

# Image Provisioning
KEY_DISK_PATH="/var/lib/libvirt/images/$VM_NAME-keys.iso"
KEY_EXTRA_DISK=""

# Resolve authoritative enrollment artifacts
# Priority 1: Persistent Sovereign Keys (~/.koob/keys)
# Priority 2: Local Workspace (tmp/secure-boot/enroll)
ENROLL_DIR="$HOME/.koob/keys"
if [ ! -f "$ENROLL_DIR/PK.auth" ]; then
  ENROLL_DIR="tmp/secure-boot/enroll"
fi

if [ "$INSECURE_MODE" = false ] && [ -d "$ENROLL_DIR" ]; then
  # Check if we have any files to enroll
  HAS_KEYS=false
  for f in "$ENROLL_DIR"/*.auth "$ENROLL_DIR"/*.crt "$ENROLL_DIR"/*.cer; do
    [ -f "$f" ] && HAS_KEYS=true && break
  done

  if [ "$HAS_KEYS" = true ]; then
    MSG="Preparing Secure Boot enrollment disk (via $ENROLL_DIR)..."
    printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"
    TMP_KEY_IMG="$REPO_ROOT/tmp/$VM_NAME-keys.img"

    # Generate a 2MB FAT image containing keys and authenticated variables
    rm -f "$TMP_KEY_IMG"
    sudo mkfs.vfat -C "$TMP_KEY_IMG" 2048 -n "KOOB_KEYS" >/dev/null
    sudo chown "$(id -u):$(id -g)" "$TMP_KEY_IMG"

    # Copy keys using mcopy (mtools) - only files that exist
    for f in "$ENROLL_DIR"/*.auth "$ENROLL_DIR"/*.crt "$ENROLL_DIR"/*.cer; do
      if [ -f "$f" ]; then
        mcopy -i "$TMP_KEY_IMG" "$f" ::/ >/dev/null 2>&1
      fi
    done

    sudo cp "$TMP_KEY_IMG" "$KEY_DISK_PATH"
    sudo chmod 644 "$KEY_DISK_PATH"
    if [ "$SHOULD_CHOWN" = true ]; then
      sudo chown "$LIBVIRT_USER:$LIBVIRT_GROUP" "$KEY_DISK_PATH" \
        2>/dev/null || true
    fi
    KEY_OPTS="device=disk,bus=sata,readonly=off,format=raw"
    KEY_EXTRA_DISK="--disk path=$KEY_DISK_PATH,$KEY_OPTS"
    rm "$TMP_KEY_IMG"
  fi
fi

# Select firmware based on mode
if [ "$INSECURE_MODE" = true ]; then
  if [ -f "/usr/share/OVMF/OVMF_CODE_4M.fd" ]; then
    FIRMWARE_LOADER="/usr/share/OVMF/OVMF_CODE_4M.fd"
  else
    FIRMWARE_LOADER="/usr/share/OVMF/OVMF_CODE.fd"
  fi
else
  if [ -f "/usr/share/OVMF/OVMF_CODE_4M.secboot.fd" ]; then
    FIRMWARE_LOADER="/usr/share/OVMF/OVMF_CODE_4M.secboot.fd"
  else
    FIRMWARE_LOADER="/usr/share/OVMF/OVMF_CODE.secboot.fd"
  fi
fi

# Construct virt-install parameters
# We use positional parameters "$@" to build the command safely with proper
# quoting.
BOOT_OPTS="loader=$FIRMWARE_LOADER,loader_ro=yes,loader_type=pflash"
if [ -f "/usr/share/OVMF/OVMF_VARS_4M.fd" ]; then
  VARS_TEMP="/usr/share/OVMF/OVMF_VARS_4M.fd"
else
  VARS_TEMP="/usr/share/OVMF/OVMF_VARS.fd"
fi
set -- \
  --name "$VM_NAME" \
  --memory 8192 \
  --vcpus 2 \
  --disk path="$ISO_PATH",device=disk,bus=virtio,readonly=on,format=raw \
  --os-variant linux2020 \
  --network network=default,model=virtio \
  --graphics none \
  --console pty,target_type=serial \
  --boot "$BOOT_OPTS,nvram_template=$VARS_TEMP" \
  --noautoconsole \
  --import

if [ -n "$KEY_EXTRA_DISK" ]; then
  set -- "$@" --disk "path=$KEY_DISK_PATH,$KEY_OPTS"
fi

# Install New VM
sudo virt-install "$@"

MSG="Virtual machine $VM_NAME is active."
printf "%b[ SUCCESS ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
echo ""
echo "Console connection:"
echo "   sudo virsh console $VM_NAME"

# Optional: Auto-connect to console in interactive terminals
if [ -t 0 ]; then
  printf "\nConnect to console now? [Y/n] "
  read -r opt
  case "$opt" in
    ""|[yY]*) sudo virsh console "$VM_NAME" ;;
    *)        echo "Skipping console connection." ;;
  esac
fi
