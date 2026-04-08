#!/usr/bin/env bash
# Validate golden .tf.json files against multiple Terraform and OpenTofu versions via Docker.
# Versions are read from env vars (set by Makefile) or fall back to defaults.
#
# Usage:
#   make validate-golden                              # uses versions from Makefile
#   ./scripts/validate-golden.sh                      # uses default versions
#   TERRAFORM_VERSIONS="1.9 latest" ./scripts/validate-golden.sh  # override
#
# Requires: docker

set -euo pipefail

if ! command -v docker &> /dev/null || ! docker info &> /dev/null; then
  echo "SKIP: Docker not available, skipping golden file validation"
  exit 0
fi

# Read versions from env (space-separated) or use defaults
IFS=' ' read -ra TF_VERSIONS <<< "${TERRAFORM_VERSIONS:-1.5 1.9 latest}"
IFS=' ' read -ra OT_VERSIONS <<< "${TOFU_VERSIONS:-1.8 latest}"

validate() {
  local image="$1"
  local version="$2"
  local cmd="$3"
  local golden_file="$4"
  local label="${image}:${version}"

  echo "--- ${cmd} validate (${label}) ---"

  # Use docker cp approach to avoid tmpdir mount issues
  local container_id
  container_id=$(docker create --entrypoint sh "${label}" -c "${cmd} init -backend=false 2>&1 && ${cmd} validate")
  docker cp "${golden_file}" "${container_id}:/main.tf.json"
  if ! docker start -a "${container_id}"; then
    echo "FAIL: ${golden_file} with ${label}"
    docker rm "${container_id}" > /dev/null 2>&1
    return 1
  fi
  docker rm "${container_id}" > /dev/null 2>&1
}

for dir in testdata/golden/*/; do
  golden="${dir}main.tf.golden.json"
  if [ ! -f "${golden}" ]; then
    echo "SKIP: ${dir} (no golden file)"
    continue
  fi

  for version in "${TF_VERSIONS[@]}"; do
    echo "=== Validating ${dir} with terraform:${version} ==="
    validate "hashicorp/terraform" "${version}" "terraform" "${golden}"
  done

  for version in "${OT_VERSIONS[@]}"; do
    echo "=== Validating ${dir} with opentofu:${version} ==="
    validate "ghcr.io/opentofu/opentofu" "${version}" "tofu" "${golden}"
  done
done

echo ""
echo "All golden files validated against ${#TF_VERSIONS[@]} Terraform + ${#OT_VERSIONS[@]} OpenTofu versions."
