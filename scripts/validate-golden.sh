#!/usr/bin/env bash
# Validate golden .tf.json files against multiple Terraform and OpenTofu versions via Docker.
# Versions are read from env vars (set by Makefile) or fall back to defaults.
#
# Strategy:
#   1. Group golden files by their required_providers (different version constraints
#      for the same provider can't share an init cache).
#   2. For each unique provider set, create init caches (Terraform + OpenTofu).
#   3. Run all version containers in parallel, mounting all caches read-only.
#   4. Each container iterates over groups: swap providers, validate files, repeat.
#
# Warm run is fast: no downloads, no unzipping, no init. Just validate.
#
# Usage:
#   make validate-golden                              # uses versions from Makefile
#   ./scripts/validate-golden.sh                      # uses default versions
#   TERRAFORM_VERSIONS="1.9 latest" ./scripts/validate-golden.sh  # override
#
# Requires: docker, jq

set -euo pipefail

if ! command -v docker &> /dev/null || ! docker info &> /dev/null; then
  echo "SKIP: Docker not available, skipping golden file validation"
  exit 0
fi

if ! command -v jq &> /dev/null; then
  echo "ERROR: jq is required but not found"
  exit 1
fi

# Read versions from env (space-separated) or use defaults
IFS=' ' read -ra TF_VERSIONS <<< "${TERRAFORM_VERSIONS:-1.5 1.9 latest}"
IFS=' ' read -ra OT_VERSIONS <<< "${TOFU_VERSIONS:-1.8 latest}"

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

# --- Provider set grouping ---
# Golden files with different version constraints for the same provider
# (e.g., datadog ~> 3.39 vs ~> 4.4) can't share a single init cache.
# Group files by the hash of their required_providers block.

hash_cmd() {
  if command -v sha256sum &> /dev/null; then
    sha256sum | cut -d' ' -f1
  else
    shasum -a 256 | cut -d' ' -f1
  fi
}

# Compute per-file provider hash and build groups.
# group_hashes: unique hashes (sorted)
# group_files_<hash>: space-separated file indices in that group
# group_config_<hash>: path to the tf.json config for that group
declare -A file_to_group   # file index -> group hash
declare -A group_config    # group hash -> config file path
group_hashes=()

for i in "${!golden_files[@]}"; do
  providers=$(jq -c '.terraform.required_providers // {}' "${golden_files[$i]}")
  # Canonical sort for stable hashing
  canonical=$(echo "${providers}" | jq -cS '.')
  ghash=$(echo -n "${canonical}" | hash_cmd)

  file_to_group[$i]="${ghash}"

  if [ -z "${group_config[${ghash}]+x}" ]; then
    group_hashes+=("${ghash}")
    config_file="${TMPDIR_BASE}/group-${ghash}.tf.json"
    jq -n --argjson p "${canonical}" '{"terraform":{"required_providers":$p}}' > "${config_file}"
    group_config[${ghash}]="${config_file}"
  fi
done

echo "Found ${#golden_files[@]} golden files in ${#group_hashes[@]} provider group(s)"

# --- Init cache management ---

# init_providers <image> <cmd> <volume> <config_file>
# Runs init in a container and saves .terraform/providers + lock file to a volume.
init_providers() {
  local image="$1"
  local cmd="$2"
  local vol="$3"
  local config_file="$4"

  local container_id
  container_id=$(docker create \
    --entrypoint sh \
    -v "${vol}:/cache" \
    "${image}" \
    -c "mkdir /work && cd /work && cp /tmp/config.tf.json main.tf.json && ${cmd} init -backend=false && cp -a .terraform/providers /cache/providers && cp .terraform.lock.hcl /cache/lock.hcl")

  docker cp "${config_file}" "${container_id}:/tmp/config.tf.json"

  if ! docker start -a "${container_id}" 2>&1; then
    docker rm "${container_id}" > /dev/null 2>&1 || true
    echo "FAIL: could not initialize ${cmd} provider cache"
    return 1
  fi
  docker rm "${container_id}" > /dev/null 2>&1 || true
}

# Check if a volume's cache matches the expected hash.
cache_is_valid() {
  local vol="$1"
  local expected_hash="$2"
  local stored_hash
  stored_hash=$(docker run --rm -v "${vol}:/c" alpine \
    cat /c/config.hash 2>/dev/null) || return 1
  [ "${stored_hash}" = "${expected_hash}" ]
}

# Wipe and recreate a volume when cache is stale.
reset_volume() {
  local vol="$1"
  docker volume rm "${vol}" > /dev/null 2>&1 || true
  docker volume create "${vol}" > /dev/null 2>&1 || true
}

# Store a hash in a volume to track cache validity.
store_hash() {
  local vol="$1"
  local hash_val="$2"
  docker run --rm -v "${vol}:/cache" alpine \
    sh -c "echo '${hash_val}' > /cache/config.hash"
}

# Ensure init caches exist for all provider groups.
# Volume naming: tforge-init-{tf|tofu}-{hash_prefix}
ensure_init_caches() {
  for ghash in "${group_hashes[@]}"; do
    local short="${ghash:0:12}"
    local config_file="${group_config[${ghash}]}"

    # Compute hash of the config file content for cache invalidation
    local config_hash
    config_hash=$(hash_cmd < "${config_file}")

    for tool_info in "tf|hashicorp/terraform:latest|terraform" "tofu|ghcr.io/opentofu/opentofu:latest|tofu"; do
      IFS='|' read -r tool_key image cmd <<< "${tool_info}"
      local vol="tforge-init-${tool_key}-${short}"
      docker volume create "${vol}" > /dev/null 2>&1 || true

      if cache_is_valid "${vol}" "${config_hash}"; then
        continue
      fi

      echo "Initializing ${cmd} provider cache for group ${short}..."
      reset_volume "${vol}"
      init_providers "${image}" "${cmd}" "${vol}" "${config_file}"
      store_hash "${vol}" "${config_hash}"
    done
  done
}

# --- Validation ---

# validate_version <image:tag> <cmd> <tool_key>
# Mounts all group init caches, then validates each golden file with
# the correct providers for its group.
validate_version() {
  local label="$1"
  local cmd="$2"
  local tool_key="$3"
  local outfile="${TMPDIR_BASE}/out-${label//[:\/]/_}"

  # Build volume mount args (one per group)
  local vol_args=()
  for ghash in "${group_hashes[@]}"; do
    local short="${ghash:0:12}"
    local vol="tforge-init-${tool_key}-${short}"
    vol_args+=(-v "${vol}:/init-cache-${short}:ro")
  done

  # Build in-container script:
  # For each golden file, symlink the correct group's providers and validate.
  local script="set -e
mkdir -p /work && cd /work"

  local prev_group=""
  for i in "${!golden_files[@]}"; do
    local ghash="${file_to_group[$i]}"
    local short="${ghash:0:12}"

    # Only re-link providers when switching groups
    if [ "${ghash}" != "${prev_group}" ]; then
      script="${script}
rm -f /work/.terraform/providers /work/.terraform.lock.hcl
mkdir -p /work/.terraform
ln -s /init-cache-${short}/providers /work/.terraform/providers
cp /init-cache-${short}/lock.hcl /work/.terraform.lock.hcl"
      prev_group="${ghash}"
    fi

    script="${script}
echo '--- ${golden_files[$i]} ---'
cp /tmp/golden_${i}.tf.json main.tf.json
${cmd} validate"
  done

  # Create container with all group caches mounted
  local container_id
  container_id=$(docker create \
    --entrypoint sh \
    "${vol_args[@]}" \
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
  validate_version "${label}" "terraform" "tf" &
  pids+=($!)
  labels+=("${label}")
done

for version in "${OT_VERSIONS[@]}"; do
  label="ghcr.io/opentofu/opentofu:${version}"
  validate_version "${label}" "tofu" "tofu" &
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
