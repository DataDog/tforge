#!/usr/bin/env bash
# Validate golden .tf.json files against multiple Terraform and OpenTofu versions via Docker.
# Versions are read from env vars (set by Makefile) or fall back to defaults.
#
# Strategy:
#   1. Group golden files by their required_providers (different version constraints
#      for the same provider can't share an init cache).
#   2. For each unique provider set, create init caches (Terraform + OpenTofu).
#   3. Run all version containers in parallel, mounting all caches read-only.
#   4. Each container re-symlinks providers per group, then validates each golden file.
#
# Warm run is fast: no downloads, no unzipping, no init. Just validate.
#
# Usage:
#   make validate-golden                              # uses versions from Makefile
#   ./scripts/validate-golden.sh                      # uses default versions
#   TERRAFORM_VERSIONS="1.9 latest" ./scripts/validate-golden.sh  # override
#   ./scripts/validate-golden.sh --clear-cache          # wipe cached providers
#
# Requires: docker, jq

set -euo pipefail

if [ "${1:-}" = "--clear-cache" ]; then
  echo "Clearing provider cache volumes..."
  docker volume ls --format '{{.Name}}' | grep '^tforge-' | xargs -r docker volume rm 2>/dev/null || true
  echo "Done. Run again without --clear-cache to re-populate."
  exit 0
fi

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

# Tool definitions: tool_key|init_image|command
TOOLS=("tf|hashicorp/terraform:latest|terraform" "tofu|ghcr.io/opentofu/opentofu:latest|tofu")

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

TMPDIR_BASE=$(mktemp -d)
cleanup() { rm -rf "${TMPDIR_BASE}"; }
trap cleanup EXIT

# Shared host-side plugin cache so provider binaries are downloaded once
PLUGIN_CACHE_DIR="/tmp/tforge-cache"
mkdir -p "${PLUGIN_CACHE_DIR}"

# Resolve hash command once (sha256sum on Linux, shasum on macOS)
if command -v sha256sum &> /dev/null; then
  hash_cmd() { sha256sum | cut -d' ' -f1; }
else
  hash_cmd() { shasum -a 256 | cut -d' ' -f1; }
fi

cache_vol_name() { echo "tforge-init-${1}-${2}"; }

outfile_for_label() { echo "${TMPDIR_BASE}/out-${1//[:\/]/_}"; }

run_and_rm() {
  local container_id="$1"
  local rc=0
  docker start -a "${container_id}" 2>&1 || rc=$?
  docker rm "${container_id}" > /dev/null 2>&1 || true
  return "${rc}"
}

# --- Provider set grouping ---
# Group golden files by their required_providers so incompatible
# version constraints (e.g., datadog ~> 3.39 vs ~> 4.4) get separate init caches.

declare -A file_to_group   # file index -> group hash
declare -A group_config    # group hash -> config file path
group_hashes=()

for i in "${!golden_files[@]}"; do
  canonical=$(jq -cS '.terraform.required_providers // {}' "${golden_files[$i]}")
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

# Runs init in a container and saves providers + lock file + content hash to a volume.
init_providers() {
  local image="$1"
  local cmd="$2"
  local vol="$3"
  local config_file="$4"
  local config_hash="$5"

  local container_id
  container_id=$(docker create \
    --entrypoint sh \
    -v "${vol}:/cache" \
    -v "${PLUGIN_CACHE_DIR}:/plugin-cache" \
    -e TF_PLUGIN_CACHE_DIR=/plugin-cache \
    "${image}" \
    -c "mkdir /work && cd /work && cp /tmp/config.tf.json main.tf.json && ${cmd} init -backend=false && cp -a .terraform/providers /cache/providers && cp .terraform.lock.hcl /cache/lock.hcl && echo '${config_hash}' > /cache/config.hash")

  docker cp "${config_file}" "${container_id}:/tmp/config.tf.json"

  if ! run_and_rm "${container_id}"; then
    echo "FAIL: could not initialize ${cmd} provider cache"
    return 1
  fi
}

# Batch-check which volumes have valid caches. Prints the volume name of each
# valid cache (one per line). Runs a single Alpine container.
batch_check_caches() {
  local vol_hash_pairs=("$@")
  if [ ${#vol_hash_pairs[@]} -eq 0 ]; then
    return
  fi

  # Build mount args and check script
  local mount_args=()
  local check_script=""
  local idx=0
  for pair in "${vol_hash_pairs[@]}"; do
    local vol="${pair%%|*}"
    local expected="${pair#*|}"
    mount_args+=(-v "${vol}:/check_${idx}:ro")
    check_script="${check_script}
stored=\$(cat /check_${idx}/config.hash 2>/dev/null || echo '')
if [ \"\${stored}\" = '${expected}' ]; then echo '${vol}'; fi"
    idx=$((idx + 1))
  done

  docker run --rm "${mount_args[@]}" alpine sh -c "${check_script}" 2>/dev/null || true
}

reset_volume() {
  local vol="$1"
  docker volume rm "${vol}" > /dev/null 2>&1 || true
  docker volume create "${vol}" > /dev/null 2>&1 || true
}

# Ensure init caches exist for all provider groups.
ensure_init_caches() {
  # Collect all volume+hash pairs to check
  local pairs=()
  local vol_keys=()    # parallel array: tool_key|image|cmd|config_file|config_hash
  for ghash in "${group_hashes[@]}"; do
    local short="${ghash:0:12}"
    local config_file="${group_config[${ghash}]}"
    local config_hash
    config_hash=$(hash_cmd < "${config_file}")

    for tool_info in "${TOOLS[@]}"; do
      IFS='|' read -r tool_key image cmd <<< "${tool_info}"
      local vol
      vol=$(cache_vol_name "${tool_key}" "${short}")
      docker volume create "${vol}" > /dev/null 2>&1 || true
      pairs+=("${vol}|${config_hash}")
      vol_keys+=("${vol}|${image}|${cmd}|${config_file}|${config_hash}")
    done
  done

  # Single container checks all caches at once
  local valid_vols
  valid_vols=$(batch_check_caches "${pairs[@]}")

  # Init only the stale ones
  for entry in "${vol_keys[@]}"; do
    IFS='|' read -r vol image cmd config_file config_hash <<< "${entry}"
    if echo "${valid_vols}" | grep -qxF "${vol}"; then
      continue
    fi
    echo "Initializing ${cmd} provider cache for ${vol##*-}..."
    reset_volume "${vol}"
    init_providers "${image}" "${cmd}" "${vol}" "${config_file}" "${config_hash}"
  done
}

# --- Validation ---

# Mounts all group init caches, then validates each golden file with
# the correct providers for its group.
validate_version() {
  local label="$1"
  local cmd="$2"
  local tool_key="$3"
  local outfile
  outfile=$(outfile_for_label "${label}")

  # Build volume mount args (one per group)
  local vol_args=()
  for ghash in "${group_hashes[@]}"; do
    local short="${ghash:0:12}"
    local vol
    vol=$(cache_vol_name "${tool_key}" "${short}")
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

  local container_id
  container_id=$(docker create \
    --entrypoint sh \
    "${vol_args[@]}" \
    -v "${PLUGIN_CACHE_DIR}:/plugin-cache:ro" \
    -e TF_PLUGIN_CACHE_DIR=/plugin-cache \
    "${label}" \
    -c "${script}")

  for i in "${!golden_files[@]}"; do
    docker cp "${golden_files[$i]}" "${container_id}:/tmp/golden_${i}.tf.json"
  done

  {
    echo "=== Validating with ${label} ==="
    if ! run_and_rm "${container_id}"; then
      echo "FAIL: ${label}"
      return 1
    fi
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

for label in "${labels[@]}"; do
  outfile=$(outfile_for_label "${label}")
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
