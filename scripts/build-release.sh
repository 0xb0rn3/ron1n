#!/usr/bin/env bash
set -Eeuo pipefail

RON1N_VERSION="${RON1N_VERSION:-0.0.1zoro}"
output_dir="${1:-dist/${RON1N_VERSION}}"
mkdir -p "$output_dir"

binary_version="$(go run ./cmd/ron1n version)"
if [[ "$binary_version" != "$RON1N_VERSION" ]]; then
    printf 'build-release.sh: requested version %s does not match binary version %s\n' "$RON1N_VERSION" "$binary_version" >&2
    exit 1
fi

assets=()

build_one() {
    local target_os="$1" target_arch="$2" suffix=""
    [[ "$target_os" == "windows" ]] && suffix=".exe"
    local host_asset="ron1n-${target_os}-${target_arch}${suffix}"
    local relay_asset="ron1n-relay-${target_os}-${target_arch}${suffix}"
    CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
        go build -buildvcs=false -trimpath -ldflags="-s -w" -o "${output_dir}/${host_asset}" ./cmd/ron1n
    CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
        go build -buildvcs=false -trimpath -ldflags="-s -w" -o "${output_dir}/${relay_asset}" ./cmd/ron1n-relay
    assets+=("$host_asset" "$relay_asset")
}

for target_os in linux windows darwin; do
    for target_arch in amd64 arm64; do
        build_one "$target_os" "$target_arch"
    done
done

(
    cd "$output_dir"
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "${assets[@]}" > SHA256SUMS
    else
        shasum -a 256 "${assets[@]}" > SHA256SUMS
    fi
)

printf 'Built ron1n %s release assets in %s\n' "$RON1N_VERSION" "$output_dir"
