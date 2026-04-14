#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PATCH_DIR="${ROOT_DIR}/.sparkai/patches"
BASE_REF="${PATCH_BASE_REF:-upstream/main}"

mkdir -p "${PATCH_DIR}"
find "${PATCH_DIR}" -maxdepth 1 -type f -name '*.patch' -delete

declare -a overlay_subjects=(
  "feat(auth): keycloak-only login + group gating"
  "feat(storage): minio private presigned downloads"
  "chore(selfhost): env/compose/docs for keycloak+minio"
)

if ! git rev-parse --verify "${BASE_REF}" >/dev/null 2>&1; then
  echo "base ref not found: ${BASE_REF}" >&2
  exit 1
fi

exported=0
for i in "${!overlay_subjects[@]}"; do
  subject="${overlay_subjects[$i]}"
  count=0
  commit=""
  while IFS=$'\x1f' read -r candidate_commit candidate_subject; do
    [[ -z "${candidate_commit}" || -z "${candidate_subject}" ]] && continue
    if [[ "${candidate_subject}" == "${subject}" ]]; then
      count=$((count + 1))
      if (( count == 1 )); then
        commit="${candidate_commit}"
      fi
    fi
  done < <(git log --format='%H%x1f%s' "${BASE_REF}..HEAD")

  if (( count > 1 )); then
    echo "multiple commits found for overlay subject: ${subject}" >&2
    exit 1
  fi
  if (( count == 0 )); then
    echo "skip missing overlay commit: ${subject}"
    continue
  fi

  slug="$(echo "${subject}" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//')"
  index=$((i + 1))
  file="$(printf '%03d-%s.patch' "${index}" "${slug}")"
  git format-patch -1 "${commit}" --stdout > "${PATCH_DIR}/${file}"
  exported=$((exported + 1))
done

if (( exported == 0 )); then
  echo "no overlay commits found in ${BASE_REF}..HEAD; patch directory cleared"
  exit 0
fi

echo "exported ${exported} overlay patches to ${PATCH_DIR}"
