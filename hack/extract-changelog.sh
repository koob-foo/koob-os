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

# This script extracts the version-specific section from CHANGELOG.md
# for inclusion in GitHub release notes.
# Usage: ./hack/extract-changelog.sh <version>
# Requires: grep, sed, tail, and cut.

# Resolve absolute paths and anchor to the repo root
SCRIPT_DIR=$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)
cd "$REPO_ROOT"

# Configuration
VERSION="$1"

if [ -z "$VERSION" ]; then
  echo "Usage: $0 [version]"
  exit 1
fi

# Remove 'v' prefix if present for matching
VERSION=$(echo "$VERSION" | sed 's/^v//')

# Find the start line for the version section
START_LINE=$(grep -n "## \[$VERSION\]" CHANGELOG.md | cut -d: -f1)

if [ -z "$START_LINE" ]; then
  echo "Error: Version $VERSION not found in CHANGELOG.md"
  exit 1
fi

# Find the end line (the next version header or end of file)
# tail -n +$((START_LINE + 1)) skips the current header line
NEXT_VERSION_LINE=$(tail -n +$((START_LINE + 1)) CHANGELOG.md | \
  grep -n "## \[" | head -n 1 | cut -d: -f1)

if [ -z "$NEXT_VERSION_LINE" ]; then
  # No next version found, extract until the end of the file
  sed -n "$((START_LINE + 1)),\$p" CHANGELOG.md
else
  # Extract until the line before the next version header
  # NEXT_VERSION_LINE is relative to START_LINE+1
  END_LINE=$((START_LINE + NEXT_VERSION_LINE - 1))
  sed -n "$((START_LINE + 1)),${END_LINE}p" CHANGELOG.md
fi
