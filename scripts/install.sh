#!/bin/sh
# Go source installation: go install github.com/ironpark/zapp/cmd/zapp@latest
# After installation: zapp init --app dist/MyApp.app; zapp config show; zapp build
# Install a published zapp release without requiring root.
# Optional: ZAPP_VERSION=v1.0.0-beta ZAPP_INSTALL_DIR=/custom/bin
set -eu

install_zapp() (
    fail() { printf 'zapp: %s\n' "$*" >&2; exit 1; }
    for tool in curl tar awk mktemp; do
        command -v "$tool" >/dev/null 2>&1 || fail "Required command not found: $tool"
    done
    case "$(uname -s)" in
        Darwin) platform=Darwin ;;
        Linux) platform=Linux ;;
        *) fail 'Supported systems: macOS and Linux. On Windows, use install.ps1.' ;;
    esac
    case "$(uname -m)" in
        x86_64|amd64) arch=x86_64 ;;
        arm64|aarch64) arch=arm64 ;;
        *) fail 'Supported architectures: x86_64 and arm64.' ;;
    esac
    if command -v sha256sum >/dev/null 2>&1; then
        hash_tool=sha256sum
    elif command -v shasum >/dev/null 2>&1; then
        hash_tool=shasum
    else
        fail 'SHA-256 verification requires sha256sum or shasum.'
    fi
    repo=https://github.com/ironpark/zapp
    version=${ZAPP_VERSION:-}
    if [ -z "$version" ]; then
        release_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "$repo/releases/latest")
        case "$release_url" in
            "$repo"/releases/tag/*) version=${release_url##*/} ;;
            *) fail 'Could not resolve the latest published release.' ;;
        esac
    fi
    case "$version" in
        ''|*[!A-Za-z0-9._-]*) fail 'Invalid ZAPP_VERSION; use a release tag such as v1.0.0-beta.' ;;
    esac
    install_dir=${ZAPP_INSTALL_DIR:-"$HOME/.local/bin"}
    case "$install_dir" in /*) ;; *) fail 'ZAPP_INSTALL_DIR must be an absolute path.' ;; esac
    archive="zapp_${platform}_${arch}.tar.gz"
    checksums="zapp_${version#v}_checksums.txt"
    work=$(mktemp -d)
    staged=
    trap 'rm -rf "$work"; if [ -n "$staged" ]; then rm -f "$staged"; fi' EXIT
    trap 'exit 1' HUP INT TERM
    printf 'Downloading zapp %s (%s/%s)...\n' "$version" "$platform" "$arch"
    base="$repo/releases/download/$version"
    curl -fsSL "$base/$archive" -o "$work/$archive"
    curl -fsSL "$base/$checksums" -o "$work/checksums.txt"
    expected=$(awk -v name="$archive" '$2 == name {print $1}' "$work/checksums.txt")
    [ "${#expected}" -eq 64 ] || fail 'Missing or invalid release checksum.'
    if [ "$hash_tool" = sha256sum ]; then
        actual=$(sha256sum "$work/$archive" | awk '{print $1}')
    else
        actual=$(shasum -a 256 "$work/$archive" | awk '{print $1}')
    fi
    [ "$actual" = "$expected" ] || fail 'Checksum mismatch; installation aborted.'
    tar -xzf "$work/$archive" -C "$work" zapp
    [ -f "$work/zapp" ] && [ ! -L "$work/zapp" ] || fail 'Archive does not contain a regular zapp binary.'
    mkdir -p "$install_dir"
    staged=$(mktemp "$install_dir/.zapp-install.XXXXXX")
    cp "$work/zapp" "$staged"
    chmod 755 "$staged"
    mv -f "$staged" "$install_dir/zapp"
    staged=
    printf 'Installed zapp %s to %s/zapp\n' "$version" "$install_dir"
    case ":$PATH:" in
        *":$install_dir:"*) ;;
        *) printf 'Add this directory to your shell PATH: %s\n' "$install_dir" ;;
    esac
)

install_zapp
