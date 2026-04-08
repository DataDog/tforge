#!/usr/bin/env bash
# Validate golden .tf.json files against multiple Terraform and OpenTofu versions via Docker.
# Versions are read from env vars (set by Makefile) or fall back to defaults.
#
# Strategy:
#   1. On first run, init providers for both Terraform and OpenTofu, caching the
#      .terraform directories in Docker volumes (one per tool).
#   2. Run all version containers in parallel, mounting the cached providers read-only.
#   3. Each container skips init entirely — just runs validate per golden file.
#
# Warm run is fast: no downloads, no unzipping, no init. Just validate.
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

# Docker volumes for caching (survive between runs).
# Delete with: docker volume rm tforge-tf-init tforge-tofu-init
TF_INIT_VOL="tforge-tf-init"
TOFU_INIT_VOL="tforge-tofu-init"
docker volume create "${TF_INIT_VOL}" > /dev/null 2>&1 || true
docker volume create "${TOFU_INIT_VOL}" > /dev/null 2>&1 || true

# Collect golden files
golden_files=()
for dir in testdata/golden/*/; do
  golden="${dir}main.tf.golden.json"
  if [ -f "${golden}" ]; then
    golden_files+=("${golden}")
  fi
done

if [ ${#golden_files[@]} -eq 0 ]; then
  echo "No golden files found"
  exit 0
fi

# Temp dir for helper files (cleaned up on exit)
TMPDIR_BASE=$(mktemp -d)
cleanup() { rm -rf "${TMPDIR_BASE}"; }
trap cleanup EXIT

# Superset config: union of all providers across all golden files.
SUPERSET="${TMPDIR_BASE}/superset.tf.json"
cat > "${SUPERSET}" << 'EOF'
{
  "terraform": {
    "required_providers": {
      "aws": { "source": "hashicorp/aws", "version": "~> 5.0" },
      "random": { "source": "hashicorp/random", "version": "~> 3.5" },
      "datadog": { "source": "datadog/datadog", "version": "~> 3.39" }
    }
  }
}
EOF

# init_providers <image> <cmd> <volume>
# Runs init in a container and saves .terraform/providers + lock file to a volume.
init_providers() {
  local image="$1"
  local cmd="$2"
  local vol="$3"

  local container_id
  container_id=$(docker create \
    --entrypoint sh \
    -v "${vol}:/cache" \
    "${image}" \
    -c "mkdir /work && cd /work && cp /tmp/superset.tf.json main.tf.json && ${cmd} init -backend=false && cp -a .terraform/providers /cache/providers && cp .terraform.lock.hcl /cache/lock.hcl")

  docker cp "${SUPERSET}" "${container_id}:/tmp/superset.tf.json"

  if ! docker start -a "${container_id}" 2>&1; then
    docker rm "${container_id}" > /dev/null 2>&1 || true
    echo "FAIL: could not initialize ${cmd} provider cache"
    return 1
  fi
  docker rm "${container_id}" > /dev/null 2>&1 || true
}

# Ensure both init caches are populated.
# On warm runs, both volumes already have providers — fast no-op check.
ensure_init_caches() {
  local need_tf=true need_tofu=true

  if docker run --rm -v "${TF_INIT_VOL}:/c" alpine \
    sh -c 'test -f /c/lock.hcl' 2>/dev/null; then
    need_tf=false
  fi

  if docker run --rm -v "${TOFU_INIT_VOL}:/c" alpine \
    sh -c 'test -f /c/lock.hcl' 2>/dev/null; then
    need_tofu=false
  fi

  if [ "${need_tf}" = true ]; then
    echo "Initializing Terraform provider cache (first run)..."
    init_providers "hashicorp/terraform:latest" "terraform" "${TF_INIT_VOL}"
  fi

  if [ "${need_tofu}" = true ]; then
    echo "Initializing OpenTofu provider cache (first run)..."
    init_providers "ghcr.io/opentofu/opentofu:latest" "tofu" "${TOFU_INIT_VOL}"
  fi
}

# validate_version <image:tag> <cmd> <init-vol>
# Mounts pre-initialized providers, then validates each golden file without init.
# Captures output to a temp file for clean parallel display.
validate_version() {
  local label="$1"
  local cmd="$2"
  local init_vol="$3"
  local outfile="${TMPDIR_BASE}/out-${label//[:\/]/_}"

  # Build in-container script:
  # 1. Set up .terraform with pre-cached providers (symlink, no copy)
  # 2. Copy lock file
  # 3. For each golden file: swap config and validate
  local script
  script="set -e
mkdir -p /work/.terraform && cd /work
ln -s /init-cache/providers /work/.terraform/providers
cp /init-cache/lock.hcl /work/.terraform.lock.hcl"

  for i in "${!golden_files[@]}"; do
    script="${script}
echo '--- ${golden_files[$i]} ---'
cp /tmp/golden_${i}.tf.json main.tf.json
${cmd} validate"
  done

  # Create container with init cache mounted read-only
  local container_id
  container_id=$(docker create \
    --entrypoint sh \
    -v "${init_vol}:/init-cache:ro" \
    "${label}" \
    -c "${script}")

  # Copy golden files into the container
  for i in "${!golden_files[@]}"; do
    docker cp "${golden_files[$i]}" "${container_id}:/tmp/golden_${i}.tf.json"
  done

  # Run and capture output
  {
    echo "=== Validating with ${label} ==="
    if ! docker start -a "${container_id}" 2>&1; then
      docker rm "${container_id}" > /dev/null 2>&1 || true
      echo "FAIL: ${label}"
      return 1
    fi
    docker rm "${container_id}" > /dev/null 2>&1 || true
  } > "${outfile}" 2>&1
}

ensure_init_caches

# Launch all version containers in parallel
pids=()
labels=()

for version in "${TF_VERSIONS[@]}"; do
  label="hashicorp/terraform:${version}"
  validate_version "${label}" "terraform" "${TF_INIT_VOL}" &
  pids+=($!)
  labels+=("${label}")
done

for version in "${OT_VERSIONS[@]}"; do
  label="ghcr.io/opentofu/opentofu:${version}"
  validate_version "${label}" "tofu" "${TOFU_INIT_VOL}" &
  pids+=($!)
  labels+=("${label}")
done

# Wait for all containers and collect results
failed=0
for i in "${!pids[@]}"; do
  if ! wait "${pids[$i]}"; then
    failed=1
  fi
done

# Display output from all containers (ordered, not interleaved)
for label in "${labels[@]}"; do
  outfile="${TMPDIR_BASE}/out-${label//[:\/]/_}"
  if [ -f "${outfile}" ]; then
    cat "${outfile}"
  fi
done

if [ "${failed}" -ne 0 ]; then
  echo ""
  echo "FAILED: Some validations failed (see above)."
  exit 1
fi

echo ""
echo "All ${#golden_files[@]} golden files validated against ${#TF_VERSIONS[@]} Terraform + ${#OT_VERSIONS[@]} OpenTofu versions."
