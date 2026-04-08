#!/usr/bin/env bash
# Test that tforge migration produces correct output for each CDKTF example.
#
# For each example in testdata/migration/:
#   1. Copy CDKTF source files to a temp dir
#   2. Run `tforge migrate` to process cdktf.json
#   3. Replace main.go with the hand-written tforge version
#   4. Set up go.mod with local tforge dependency
#   5. Run `go generate` + `go run` to produce .tf.json
#   6. Diff output against expected.tf.json
#
# Requires: tforge binary (built via `go install ./cmd/tforge`)

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
  total=$((total + 1))

  echo "=== Testing migration: ${name} ==="

  # Check required files exist
  for f in cdktf.json main.go.cdktf main.go.tforge go.mod.tforge expected.tf.json; do
    if [ ! -f "${example_dir}/${f}" ]; then
      echo "SKIP: ${name} (missing ${f})"
      continue 2
    fi
  done

  # Set up temp dir with CDKTF source
  tmpdir=$(mktemp -d)
  trap_cleanup() { rm -rf "${tmpdir}"; }
  trap trap_cleanup EXIT

  # Copy CDKTF files (the starting state before migration)
  cp "${example_dir}/cdktf.json" "${tmpdir}/"
  cp "${example_dir}/main.go.cdktf" "${tmpdir}/main.go"
  if [ -f "${example_dir}/package.json" ]; then
    cp "${example_dir}/package.json" "${tmpdir}/"
  fi
  if [ -f "${example_dir}/.gitignore" ]; then
    cp "${example_dir}/.gitignore" "${tmpdir}/"
  fi

  # Step 1: Run tforge migrate
  if ! "${TFORGE_BIN}" migrate -dir "${tmpdir}" > /dev/null 2>&1; then
    echo "FAIL: ${name} (tforge migrate failed)"
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

  # Verify generate.go was created
  if [ ! -f "${tmpdir}/generate.go" ]; then
    echo "FAIL: ${name} (generate.go not created)"
    failed=$((failed + 1))
    rm -rf "${tmpdir}"
    continue
  fi

  # Step 2: Replace main.go with tforge version
  cp "${example_dir}/main.go.tforge" "${tmpdir}/main.go"

  # Step 3: Set up go.mod with local replace
  cp "${example_dir}/go.mod.tforge" "${tmpdir}/go.mod"
  echo "replace tforge => ${TFORGE_ROOT}" >> "${tmpdir}/go.mod"

  # Step 4: Generate provider bindings and run
  if ! (cd "${tmpdir}" && go generate ./... 2>/dev/null); then
    echo "FAIL: ${name} (go generate failed)"
    failed=$((failed + 1))
    rm -rf "${tmpdir}"
    continue
  fi

  if ! (cd "${tmpdir}" && go run . 2>/dev/null); then
    echo "FAIL: ${name} (go run failed)"
    failed=$((failed + 1))
    rm -rf "${tmpdir}"
    continue
  fi

  # Step 5: Diff output against expected
  if ! diff -u "${example_dir}/expected.tf.json" "${tmpdir}/main.tf.json" > /dev/null 2>&1; then
    echo "FAIL: ${name} (output mismatch)"
    diff -u "${example_dir}/expected.tf.json" "${tmpdir}/main.tf.json" || true
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
