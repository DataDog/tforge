#!/usr/bin/env bash
# Installs pinned Terraform and OpenTofu CLIs into $1 (default: $(go env GOPATH)/bin).
# CI uses this because the org's Actions policy blocks the third-party setup actions.
set -euo pipefail

# renovate: datasource=github-releases depName=hashicorp/terraform
TERRAFORM_VERSION="${TERRAFORM_VERSION:-1.17.2}"
# renovate: datasource=github-releases depName=opentofu/opentofu
TOFU_VERSION="${TOFU_VERSION:-1.13.9}"
BIN_DIR="${1:-$(go env GOPATH)/bin}"
PLATFORM="$(go env GOOS)_$(go env GOARCH)"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$BIN_DIR"

# install <name> <zip url> <SHA256SUMS url>
install() {
  local name="$1" zip_url="$2" sums_url="$3" zip
  zip="$(basename "$zip_url")"
  curl -sSfL -o "$tmp/$zip" "$zip_url"
  curl -sSfL -o "$tmp/$name.sums" "$sums_url"
  (cd "$tmp" && grep " ${zip}\$" "$name.sums" | shasum -a 256 -c -)
  unzip -oq "$tmp/$zip" "$name" -d "$BIN_DIR"
  "$BIN_DIR/$name" version | head -1
}

install terraform \
  "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_${PLATFORM}.zip" \
  "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_SHA256SUMS"

install tofu \
  "https://github.com/opentofu/opentofu/releases/download/v${TOFU_VERSION}/tofu_${TOFU_VERSION}_${PLATFORM}.zip" \
  "https://github.com/opentofu/opentofu/releases/download/v${TOFU_VERSION}/tofu_${TOFU_VERSION}_SHA256SUMS"
