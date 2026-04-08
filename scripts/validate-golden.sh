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

# Read versions from env (space-separated) or use defaults
IFS=' ' read -ra TF_VERSIONS <<< "${TERRAFORM_VERSIONS:-1.5 1.9 latest}"
IFS=' ' read -ra OT_VERSIONS <<< "${TOFU_VERSIONS:-1.8 latest}"

failed=0

validate() {
  local image="$1"
  local version="$2"
  local cmd="$3"
  local golden_dir="$4"

  tmpdir=$(mktemp -d)

  cp "${golden_dir}"main.tf.golden.json "${tmpdir}/main.tf.json"

  if ! docker run --rm -v "${tmpdir}:/work" -w /work "${image}:${version}" \
    sh -c "${cmd} init -backend=false > /dev/null 2>&1 && ${cmd} validate" 2>&1; then
    echo "FAIL: ${golden_dir} with ${image}:${version}"
    rm -rf "${tmpdir}"
    return 1
  fi

  rm -rf "${tmpdir}"
}

for dir in testdata/golden/*/; do
  if [ ! -f "${dir}main.tf.golden.json" ]; then
    echo "SKIP: ${dir} (no golden file)"
    continue
  fi

  for version in "${TF_VERSIONS[@]}"; do
    echo "Validating ${dir} with terraform:${version}..."
    if ! validate "hashicorp/terraform" "$version" "terraform" "$dir"; then
      failed=1
    fi
  done

  for version in "${OT_VERSIONS[@]}"; do
    echo "Validating ${dir} with opentofu:${version}..."
    if ! validate "ghcr.io/opentofu/opentofu" "$version" "tofu" "$dir"; then
      failed=1
    fi
  done
done

if [ $failed -ne 0 ]; then
  echo ""
  echo "Some validations failed."
  exit 1
fi

echo ""
echo "All golden files validated against ${#TF_VERSIONS[@]} Terraform + ${#OT_VERSIONS[@]} OpenTofu versions."
