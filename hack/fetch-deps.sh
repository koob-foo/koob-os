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

# This script downloads external binary dependencies (Kubelet, containerd,
# runc, CNI plugins, and CA certs) into the rootfs staging area.
# Requires: curl, tar, and hack/init-rootfs.sh.

# ANSI Color codes
OK_COLOR='\033[1;32m'
INFO_COLOR='\033[1;36m'
NC='\033[0m' # No Color

# Resolve absolute paths and anchor to the repo root
SCRIPT_DIR=$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)
cd "$REPO_ROOT"

# Configuration
K8S_VER="v1.36.1"
CONTAINERD_VER="2.3.0"
RUNC_VER="v1.4.2"
CNI_VER="v1.9.1"
ARCH="amd64"
ROOTFS="tmp/rootfs"
DEST_BIN="$ROOTFS/bin"
DEST_CNI="$ROOTFS/opt/cni/bin"

# Ensure core rootfs skeleton exists
sh hack/init-rootfs.sh

# Idempotency check
MSG="Fetching external dependencies..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

# Kubernetes (Kubelet)
if [ ! -f "$DEST_BIN/kubelet" ]; then
  MSG="Downloading Kubelet $K8S_VER..."
  printf "   %s\n" "$MSG"
  URL_BASE="https://dl.k8s.io/release/$K8S_VER/bin/linux/$ARCH"
  mkdir -p "$DEST_BIN"
  curl -L --progress-bar -o "$DEST_BIN/kubelet" "$URL_BASE/kubelet"
  chmod +x "$DEST_BIN/kubelet"
else
  MSG="Kubelet present"
  printf "   %b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
fi

# Containerd
if [ ! -f "$DEST_BIN/containerd" ]; then
  MSG="Downloading Containerd $CONTAINERD_VER..."
  printf "   %s\n" "$MSG"
  URL_BASE="https://github.com/containerd/containerd/releases/download"
  URL="${URL_BASE}/v${CONTAINERD_VER}/containerd-${CONTAINERD_VER}"
  URL="${URL}-linux-${ARCH}.tar.gz"
  curl -L --progress-bar -o containerd.tar.gz "$URL"
  tar -xzvf containerd.tar.gz -C "$DEST_BIN" \
    bin/containerd bin/containerd-shim-runc-v2 --strip-components=1
  rm containerd.tar.gz
else
  MSG="Containerd present"
  printf "   %b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
fi

# Runc
if [ ! -f "$DEST_BIN/runc" ]; then
  MSG="Downloading Runc $RUNC_VER..."
  printf "   %s\n" "$MSG"
  URL_BASE="https://github.com/opencontainers/runc/releases/download"
  URL="${URL_BASE}/$RUNC_VER/runc.$ARCH"
  curl -L --progress-bar -o "$DEST_BIN/runc" "$URL"
  chmod +x "$DEST_BIN/runc"
else
  MSG="Runc present"
  printf "   %b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
fi

# CNI plugins
if [ -z "$(ls -A "$DEST_CNI" 2>/dev/null)" ]; then
  MSG="Downloading CNI Plugins $CNI_VER..."
  printf "   %s\n" "$MSG"
  URL_BASE="https://github.com/containernetworking/plugins/releases/download"
  URL="${URL_BASE}/$CNI_VER/cni-plugins-linux-$ARCH-$CNI_VER.tgz"
  curl -L --progress-bar -o cni.tgz "$URL"
  mkdir -p "$DEST_CNI"
  tar -xzvf cni.tgz -C "$DEST_CNI"
  rm cni.tgz

  MSG="Optimizing CNI plugin footprint..."
  printf "   %b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"
  KEEP="bridge firewall host-local loopback portmap"
  # Directory traversal
  cd "$DEST_CNI"
  for plugin in *; do
    [ -d "$plugin" ] && continue
    case " $KEEP " in
      *" $plugin "*) continue ;;
    esac
    [ "$plugin" = "LICENSE" ] || [ "$plugin" = "README.md" ] && continue
    rm "$plugin"
  done
  cd "$REPO_ROOT"
else
  MSG="CNI Plugins present"
  printf "   %b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
fi

# CA certificates
if [ ! -f "$ROOTFS/etc/ssl/certs/ca-certificates.crt" ]; then
  MSG="Downloading CA Certificates..."
  printf "   %s\n" "$MSG"
  URL="https://curl.se/ca/cacert.pem"
  mkdir -p "$ROOTFS/etc/ssl/certs"
  curl -L --progress-bar -o "$ROOTFS/etc/ssl/certs/ca-certificates.crt" "$URL"
else
  MSG="CA Certificates present"
  printf "   %b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
fi

MSG="Dependency verification complete."
printf "%b[ SUCCESS ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
