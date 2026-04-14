#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PATCH_DIR="${ROOT_DIR}/.sparkai/patches"
BASE_REF="${PATCH_BASE_REF:-upstream/main}"
PATCH_FILE="${PATCH_DIR}/001-sparkai-overlay.patch"

mkdir -p "${PATCH_DIR}"
find "${PATCH_DIR}" -maxdepth 1 -type f -name '*.patch' -delete

if ! git rev-parse --verify "${BASE_REF}" >/dev/null 2>&1; then
  echo "base ref not found: ${BASE_REF}" >&2
  exit 1
fi

if git diff --quiet "${BASE_REF}...HEAD" -- . ':(exclude).sparkai/patches/*'; then
  echo "no overlay diff found in ${BASE_REF}...HEAD; patch directory cleared"
  exit 0
fi

git diff --binary --full-index "${BASE_REF}...HEAD" -- . ':(exclude).sparkai/patches/*' > "${PATCH_FILE}"
echo "exported overlay patch to ${PATCH_FILE}"
