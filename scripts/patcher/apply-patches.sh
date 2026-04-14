#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PATCH_DIR="${ROOT_DIR}/.sparkai/patches"

if [[ ! -d "${PATCH_DIR}" ]]; then
  echo "patch directory not found: ${PATCH_DIR}" >&2
  exit 1
fi

found_patch=0
while IFS= read -r patch; do
  found_patch=1
  name="$(basename "${patch}")"
  if git apply --reverse --check "${patch}" >/dev/null 2>&1; then
    echo "skip ${name} (already applied)"
    continue
  fi

  echo "apply ${name}"
  if ! git apply --3way "${patch}"; then
    echo "failed to apply ${name}" >&2
    exit 1
  fi
done < <(find "${PATCH_DIR}" -maxdepth 1 -type f -name '*.patch' | sort)

if [[ ${found_patch} -eq 0 ]]; then
  echo "no patches found in ${PATCH_DIR}"
  exit 0
fi

echo "all patches applied successfully"
