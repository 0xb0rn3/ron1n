#!/usr/bin/env bash
set -Eeuo pipefail

RON1N_VERSION="${RON1N_VERSION:-0.0.1zoro}"
RON1N_REPOSITORY="${RON1N_REPOSITORY:-0xb0rn3/ron1n}"
install_scope="${RON1N_INSTALL_SCOPE:-user}"

usage() {
    cat <<'EOF'
Usage: install.sh [--user|--system]

Downloads native ron1n and ron1n-relay release binaries, verifies both against
the release SHA256SUMS file, and installs them. No exploit content is downloaded
until the operator later runs `ron1n install`.
EOF
}

while (($#)); do
    case "$1" in
        --user) install_scope="user" ;;
        --system) install_scope="system" ;;
        -h|--help) usage; exit 0 ;;
        *) printf 'install.sh: unknown argument: %s\n' "$1" >&2; usage >&2; exit 2 ;;
    esac
    shift
done

case "$(uname -s)" in
    Linux) target_os="linux" ;;
    Darwin) target_os="darwin" ;;
    *) printf 'install.sh: only Linux and macOS are supported by this installer\n' >&2; exit 1 ;;
esac

case "$(uname -m)" in
    x86_64|amd64) target_arch="amd64" ;;
    arm64|aarch64) target_arch="arm64" ;;
    *) printf 'install.sh: unsupported architecture: %s\n' "$(uname -m)" >&2; exit 1 ;;
esac

command -v curl >/dev/null 2>&1 || {
    printf 'install.sh: curl is required\n' >&2
    exit 1
}

release_base="https://github.com/${RON1N_REPOSITORY}/releases/download/${RON1N_VERSION}"
host_asset="ron1n-${target_os}-${target_arch}"
relay_asset="ron1n-relay-${target_os}-${target_arch}"
temp_dir="$(mktemp -d "${TMPDIR:-/tmp}/ron1n-install.XXXXXXXX")"
trap 'rm -rf -- "$temp_dir"' EXIT

curl --fail --silent --show-error --location "${release_base}/SHA256SUMS" --output "${temp_dir}/SHA256SUMS"
curl --fail --silent --show-error --location "${release_base}/${host_asset}" --output "${temp_dir}/${host_asset}"
curl --fail --silent --show-error --location "${release_base}/${relay_asset}" --output "${temp_dir}/${relay_asset}"

sha256_file() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{print $1}'
    else
        shasum -a 256 "$1" | awk '{print $1}'
    fi
}

verify_asset() {
    local asset="$1" expected actual
    expected="$(awk -v wanted="$asset" '$2 == wanted {print $1; exit}' "${temp_dir}/SHA256SUMS")"
    [[ "$expected" =~ ^[0-9a-fA-F]{64}$ ]] || {
        printf 'install.sh: checksum for %s is absent or invalid\n' "$asset" >&2
        exit 1
    }
    actual="$(sha256_file "${temp_dir}/${asset}")"
    actual="$(printf '%s' "$actual" | tr '[:upper:]' '[:lower:]')"
    expected="$(printf '%s' "$expected" | tr '[:upper:]' '[:lower:]')"
    [[ "$actual" == "$expected" ]] || {
        printf 'install.sh: SHA-256 mismatch for %s\n' "$asset" >&2
        exit 1
    }
}

verify_asset "$host_asset"
verify_asset "$relay_asset"

if [[ "$install_scope" == "system" ]]; then
    destination="/usr/local/bin"
    if [[ "${EUID}" -eq 0 ]]; then
        install -d -m 0755 "$destination"
        install -m 0755 "${temp_dir}/${host_asset}" "${destination}/ron1n"
        install -m 0755 "${temp_dir}/${relay_asset}" "${destination}/ron1n-relay"
    else
        command -v sudo >/dev/null 2>&1 || {
            printf 'install.sh: --system requires root or sudo\n' >&2
            exit 1
        }
        sudo install -d -m 0755 "$destination"
        sudo install -m 0755 "${temp_dir}/${host_asset}" "${destination}/ron1n"
        sudo install -m 0755 "${temp_dir}/${relay_asset}" "${destination}/ron1n-relay"
    fi
else
    destination="${RON1N_INSTALL_DIR:-${HOME}/.local/bin}"
    install -d -m 0755 "$destination"
    install -m 0755 "${temp_dir}/${host_asset}" "${destination}/ron1n"
    install -m 0755 "${temp_dir}/${relay_asset}" "${destination}/ron1n-relay"
fi

printf 'Installed ron1n %s and ron1n-relay to %s\n' "$RON1N_VERSION" "$destination"
case ":${PATH}:" in
    *":${destination}:"*) ;;
    *) printf 'Add %s to PATH, then run: ron1n install\n' "$destination" ;;
esac
