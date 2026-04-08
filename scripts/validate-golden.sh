#!/usr/bin/env bash
# Validate golden .tf.json files against multiple Terraform and OpenTofu versions via Docker.
#
# Usage:
#   ./scripts/validate-golden.sh                    # validate all golden dirs
#   ./scripts/validate-golden.sh testdata/golden/datadog  # validate one dir
#
# Requires: docker

set -euo pipefail

TERRAFORM_VERSIONS=("1.5" "1.9" "latest")
TOFU_VERSIONS=("1.8" "latest")

GOLDEN_DIRS=("${@:-testdata/golden/*/}")

# Expand globs if no args
if [ $# -eq 0 ]; then
  GOLDEN_DIRS=()
  for dir in testdata/golden/*/; do
    GOLDEN_DIRS+=("$dir")
  done
fi

validate_with_docker() {
  local image="$1"
  local tag="$2"
  local golden_dir="$3"
  local label="${image}:${tag}"

  tmpdir=$(mktemp -d)
  trap "rm -rf $tmpdir" RETURN

  cp "${golden_dir}"main.tf.golden.json "${tmpdir}/main.tf.json"

  if ! docker run --rm -v "${tmpdir}:/work" -w /work "${image}:${tag}" \
    sh -c "terraform init -backend=false > /dev/null 2>&1 && terraform validate" 2>&1; then
    echo "FAIL: ${golden_dir} with ${label}"
    return 1
  fi
}

validate_tofu_with_docker() {
  local tag="$1"
  local golden_dir="$2"
  local label="tofu:${tag}"

  tmpdir=$(mktemp -d)
  trap "rm -rf $tmpdir" RETURN

  cp "${golden_dir}"main.tf.golden.json "${tmpdir}/main.tf.json"

  if ! docker run --rm -v "${tmpdir}:/work" -w /work "ghcr.io/opentofu/opentofu:${tag}" \
    sh -c "tofu init -backend=false > /dev/null 2>&1 && tofu validate" 2>&1; then
    echo "FAIL: ${golden_dir} with ${label}"
    return 1
  fi
}

failed=0

for dir in "${GOLDEN_DIRS[@]}"; do
  if [ ! -f "${dir}main.tf.golden.json" ]; then
    echo "SKIP: ${dir} (no golden file)"
    continue
  fi

  for version in "${TERRAFORM_VERSIONS[@]}"; do
    echo "Validating ${dir} with hashicorp/terraform:${version}..."
    if ! validate_with_docker "hashicorp/terraform" "$version" "$dir"; then
      failed=1
    fi
  done

  for version in "${TOFU_VERSIONS[@]}"; do
    echo "Validating ${dir} with opentofu:${version}..."
    if ! validate_tofu_with_docker "$version" "$dir"; then
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
echo "All golden files validated against ${#TERRAFORM_VERSIONS[@]} Terraform + ${#TOFU_VERSIONS[@]} OpenTofu versions."
