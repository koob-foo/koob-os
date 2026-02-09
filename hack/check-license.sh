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

# This script verifies that source files contain the mandatory Apache 2.0
# license header. It is primarily used as a pre-commit hook.
# Requires: grep.

# ANSI Color codes
INFO_COLOR='\033[1;36m'
FAIL_COLOR='\033[1;31m'
NC='\033[0m' # No Color

# Resolve absolute paths and anchor to the repo root
SCRIPT_DIR=$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)
cd "$REPO_ROOT"

# Configuration
EXIT_CODE=0

for file in "$@"; do
  SUCCESS=true

  # Check for Copyright string and opening fold marker
  if ! grep -q "Copyright 2026 Koob Foo {{{" "$file"; then
    SUCCESS=false
  fi

  # Check for closing fold marker
  if ! grep -q "limitations under the License. }}}" "$file"; then
    SUCCESS=false
  fi

  if [ "$SUCCESS" = false ]; then
    MSG="License header missing or malformed (Vim folds?): $file"
    printf "%b[ FAIL ]%b %s\n" "${FAIL_COLOR}" "${NC}" "$MSG"
    EXIT_CODE=1
  fi
done

if [ "$EXIT_CODE" -ne 0 ]; then
  echo ""
  MSG="Please add the standard Apache 2.0 header to these files."
  printf "%b[ INFO ]%b %s\n" "${INFO_COLOR}" "${NC}" "$MSG"
  echo "         See docs/ci-cd.md for the template."
fi

exit "$EXIT_CODE"
