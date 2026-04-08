#!/usr/bin/env bash
# Test that tforge migration produces correct output for each CDKTF example.
#
# Each example lives in testdata/migration/<name>/ with two subdirs:
#   cdktf/  — original CDKTF project (exact copy from terraform-cdk),
#             including cdktf.out/ with the expected output from cdktf synth
#   tforge/ — hand-written tforge conversion (main.go, go.mod)
#
# The test:
#   1. Copies cdktf/ to a temp dir
#   2. Runs `tforge migrate` (verifies cdktf.json deleted, generate.go created)
#   3. Swaps in main.go and go.mod from tforge/
#   4. Runs `go generate` + `go run` to produce main.tf.json
#   5. Diffs output against cdktf/cdktf.out/.../cdk.tf.json

set -euo pipefail

TFORGE_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MIGRATION_DIR="${TFORGE_ROOT}/testdata/migration"
TFORGE_BIN="${TFORGE_ROOT}/tforge"

# Always rebuild to ensure binary is current
echo "Building tforge..."
go build -o "${TFORGE_BIN}" ./cmd/tforge

failed=0
total=0

for example_dir in "${MIGRATION_DIR}"/*/; do
  name=$(basename "${example_dir}")
  cdktf_dir="${example_dir}/cdktf"
  tforge_dir="${example_dir}/tforge"
  total=$((total + 1))

  echo "=== Testing migration: ${name} ==="

  # Check required dirs and files
  if [ ! -d "${cdktf_dir}" ] || [ ! -d "${tforge_dir}" ]; then
    echo "SKIP: ${name} (missing cdktf/ or tforge/ subdir)"
    continue
  fi
  if [ ! -f "${cdktf_dir}/cdktf.json" ]; then
    echo "SKIP: ${name} (missing cdktf/cdktf.json)"
    continue
  fi
  if [ ! -f "${tforge_dir}/main.go" ]; then
    echo "SKIP: ${name} (missing tforge/main.go)"
    continue
  fi

  # Find expected output (cdktf.out/stacks/*/cdk.tf.json)
  expected_file=$(find "${cdktf_dir}/cdktf.out" -name "cdk.tf.json" 2>/dev/null | head -1)
  if [ -z "${expected_file}" ]; then
    echo "SKIP: ${name} (no cdktf.out/.../cdk.tf.json found)"
    continue
  fi

  # Set up temp dir with CDKTF source (exact copy, excluding cdktf.out)
  tmpdir=$(mktemp -d)
  for f in "${cdktf_dir}"/*; do
    fname=$(basename "$f")
    [ "${fname}" = "cdktf.out" ] && continue
    cp -a "$f" "${tmpdir}/"
  done
  # Copy hidden files
  for f in "${cdktf_dir}"/.*; do
    fname=$(basename "$f")
    [ "${fname}" = "." ] || [ "${fname}" = ".." ] && continue
    cp -a "$f" "${tmpdir}/"
  done

  # Step 1: Run tforge migrate
  if ! "${TFORGE_BIN}" migrate -dir "${tmpdir}" > /dev/null 2>&1; then
    echo "FAIL: ${name} (tforge migrate failed)"
    "${TFORGE_BIN}" migrate -dir "${tmpdir}" 2>&1 || true
    failed=$((failed + 1))
    rm -rf "${tmpdir}"
    continue
  fi

  # Verify cdktf.json was deleted
  if [ -f "${tmpdir}/cdktf.json" ]; then
    echo "FAIL: ${name} (cdktf.json not deleted)"
    failed=$((failed + 1))
    rm -rf "${tmpdir}"
    continue
  fi

  # Verify at least one provider generate.go was created
  gen_count=$(find "${tmpdir}" -name "generate.go" -not -path "*/cdktf.out/*" | wc -l)
  if [ "${gen_count}" -eq 0 ]; then
    echo "FAIL: ${name} (no generate.go created)"
    failed=$((failed + 1))
    rm -rf "${tmpdir}"
    continue
  fi

  # Step 2: Swap in tforge main.go and go.mod (with local replace)
  cp "${tforge_dir}/main.go" "${tmpdir}/main.go"
  cp "${tforge_dir}/go.mod" "${tmpdir}/go.mod"
  echo "replace tforge => ${TFORGE_ROOT}" >> "${tmpdir}/go.mod"

  # Step 3: Generate provider bindings and run
  if ! (cd "${tmpdir}" && go generate ./... 2>/dev/null); then
    echo "FAIL: ${name} (go generate failed)"
    (cd "${tmpdir}" && go generate ./... 2>&1) || true
    failed=$((failed + 1))
    rm -rf "${tmpdir}"
    continue
  fi

  if ! (cd "${tmpdir}" && go run . 2>/dev/null); then
    echo "FAIL: ${name} (go run failed)"
    (cd "${tmpdir}" && go run . 2>&1) || true
    failed=$((failed + 1))
    rm -rf "${tmpdir}"
    continue
  fi

  # Step 4: Diff tforge output against CDKTF expected output
  if ! diff -u "${expected_file}" "${tmpdir}/main.tf.json" > /dev/null 2>&1; then
    echo "FAIL: ${name} (output mismatch)"
    diff -u "${expected_file}" "${tmpdir}/main.tf.json" || true
    failed=$((failed + 1))
    rm -rf "${tmpdir}"
    continue
  fi

  echo "PASS: ${name}"
  rm -rf "${tmpdir}"
done

echo ""
if [ "${failed}" -ne 0 ]; then
  echo "FAILED: ${failed}/${total} migration tests failed."
  exit 1
fi

echo "All ${total} migration tests passed."
