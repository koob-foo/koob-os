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

# This script builds the essential shared libraries (GLIBC) and kernel
# headers required for the koob OS root filesystem. It performs a
# hermetic build and deploys the loader and key libraries to staging.
# Requires: build-essential, curl, tar, bison, and gawk.

# ANSI Color codes
OK_COLOR='\033[1;32m'
INFO_COLOR='\033[1;36m'
NC='\033[0m' # No Color

# Resolve absolute paths and anchor to the repo root
SCRIPT_DIR=$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)
cd "$REPO_ROOT"

# Ephemeral resource management
TEMP_DIR="tmp/glibc-build"
mkdir -p "$TEMP_DIR"
# Note: We do NOT trap-delete TEMP_DIR here by default because GLIBC
# takes 10+ minutes to build; we want to preserve partial progress if
# the script is interrupted during development.

# Configuration
GLIBC_VERSION="2.40"
KERNEL_VERSION="6.18.32"
ROOTFS="tmp/rootfs"

GLIBC_URL="https://gnu.mirrorservice.org/glibc/glibc-${GLIBC_VERSION}.tar.xz"
KURL_BASE="https://cdn.kernel.org/pub/linux/kernel/v6.x"
KERNEL_URL="${KURL_BASE}/linux-${KERNEL_VERSION}.tar.xz"
JOBS=$(nproc 2>/dev/null || echo 1)

# Idempotency check
if [ -f "$ROOTFS/.libs_deployed" ]; then
  # In CI, we trust the existence of .libs_deployed if the cache hit
  if [ -n "$GITHUB_ACTIONS" ]; then
    MSG="Shared libraries restored from cache."
    printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
    exit 0
  fi

  MSG="Skipping GLIBC build; shared libraries already deployed."
  printf "%b[ OK ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
  exit 0
fi

MSG="Preparing hermetic build environment for GLIBC..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

# Download external sources
cd "$TEMP_DIR"
if [ ! -f "glibc.tar.xz" ]; then
  MSG="Downloading GLIBC ${GLIBC_VERSION}..."
  printf "   %s\n" "$MSG"
  curl -L --progress-bar -o glibc.tar.xz "$GLIBC_URL"
fi
if [ ! -f "linux.tar.xz" ]; then
  MSG="Downloading kernel headers (${KERNEL_VERSION})..."
  printf "   %s\n" "$MSG"
  curl -L --progress-bar -o linux.tar.xz "$KERNEL_URL"
fi

# Extract archives
if [ ! -d "glibc-${GLIBC_VERSION}" ]; then
  MSG="Extracting GLIBC..."
  printf "   %s\n" "$MSG"
  tar -xf glibc.tar.xz
fi
if [ ! -d "linux-${KERNEL_VERSION}" ]; then
  MSG="Extracting Linux..."
  printf "   %s\n" "$MSG"
  tar -xf linux.tar.xz
fi

# Install isolated kernel headers
KERNEL_HEADERS_DIR="$REPO_ROOT/$TEMP_DIR/kernel-headers"
if [ ! -d "$KERNEL_HEADERS_DIR" ]; then
  MSG="Installing sanitized kernel headers..."
  printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"
  mkdir -p "$KERNEL_HEADERS_DIR"
  cd "linux-${KERNEL_VERSION}"
  make headers_install ARCH=x86_64 INSTALL_HDR_PATH="$KERNEL_HEADERS_DIR"
  cd - >/dev/null
fi

# Build GLIBC
BUILD_DIR="$REPO_ROOT/$TEMP_DIR/build"
INSTALL_DIR="$REPO_ROOT/$TEMP_DIR/install"
mkdir -p "$BUILD_DIR" "$INSTALL_DIR"

cd "$BUILD_DIR"

if [ ! -f "Makefile" ]; then
  MSG="Configuring GLIBC (prefix=/usr)..."
  printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

  # Reset flags to avoid contamination from host environment
  unset CFLAGS CPPFLAGS LDFLAGS

  # -U_FORTIFY_SOURCE is required on Ubuntu 24.04 to fix syslog inlining
  export CFLAGS="-g -O2 -U_FORTIFY_SOURCE"

  # We use positional parameters to build the complex configuration safely.
  set -- \
    --prefix=/usr \
    --libdir=/usr/lib \
    --disable-profile \
    --enable-add-ons \
    --without-selinux \
    --disable-werror \
    --disable-nscd \
    --enable-stack-protector=strong \
    --with-headers="$KERNEL_HEADERS_DIR/include"

  CC="gcc" CXX=false ../glibc-${GLIBC_VERSION}/configure "$@"
fi

MSG="Commencing compilation (Jobs: $JOBS)..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"
make -j"$JOBS"

MSG="Installing to temporary staging area..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"
make install_root="$INSTALL_DIR" install

# Deploy to rootfs
MSG="Deploying core libraries to $ROOTFS..."
printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"

mkdir -p "$REPO_ROOT/$ROOTFS/lib" "$REPO_ROOT/$ROOTFS/lib64"
SRC_LIB="$INSTALL_DIR/lib64"

# Deploy the Dynamic Loader
cp "$SRC_LIB/ld-linux-x86-64.so.2" "$REPO_ROOT/$ROOTFS/lib64/"
strip --strip-debug "$REPO_ROOT/$ROOTFS/lib64/ld-linux-x86-64.so.2"

# Deploy Key Shared Libraries
LIBS="libc.so.6 libpthread.so.0 libresolv.so.2 libm.so.6 librt.so.1 libdl.so.2"
for lib in $LIBS; do
  cp -L "$SRC_LIB/$lib" "$REPO_ROOT/$ROOTFS/lib/"
  strip --strip-debug "$REPO_ROOT/$ROOTFS/lib/$lib"
done

MSG="GLIBC build and deployment complete"
printf "%b[ SUCCESS ]%b %s\n" "${OK_COLOR}" "${NC}" "$MSG"
touch "$REPO_ROOT/$ROOTFS/.libs_deployed"
