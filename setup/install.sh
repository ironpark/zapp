#!/usr/bin/env bash
# Installs a zapp release for this runner, verified against the release
# checksums and cached in the tool cache, and puts it on PATH.
#
# ZAPP_SETUP_VERSION  a version (1.2.0 or v1.2.0), "latest", "local" to use the
#                     zapp already on PATH, or empty to follow the action's ref
# ZAPP_SETUP_REF      the ref the action was used at (github.action_ref)
# GH_TOKEN            token for the GitHub API and release downloads
set -euo pipefail

repo=ironpark/zapp
requested=${ZAPP_SETUP_VERSION:-}
ref=${ZAPP_SETUP_REF:-}

# Windows runners run this in Git Bash; zapp.exe and $GITHUB_PATH want Windows
# paths, which cygpath's mixed form (C:/dir) serves as well as bash.
native() {
  if command -v cygpath >/dev/null; then cygpath -m "$1"; else printf '%s\n' "$1"; fi
}

output() { echo "$1=$2" >>"$GITHUB_OUTPUT"; }

if [[ $requested == local ]]; then
  path=$(command -v zapp || command -v zapp.exe || true)
  if [[ -z $path ]]; then
    echo "::error::version: local needs zapp on PATH" >&2
    exit 1
  fi
  # Releases before 1.1 have no --version.
  output version "$(zapp --version 2>/dev/null | awk '{print $NF}' || true)"
  output path "$(native "$path")"
  exit 0
fi

# An empty version follows the action's own ref: @v1.2.0 installs 1.2.0 and
# @v1 the newest 1.x release. Any other ref, such as a branch, installs latest.
if [[ -z $requested ]]; then
  if [[ $ref =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
    requested=$ref
  elif [[ $ref =~ ^v[0-9]+$ ]]; then
    tag=$(gh api "repos/$repo/releases" --paginate \
      -q ".[] | select(.prerelease | not) | .tag_name | select(startswith(\"$ref.\"))" | head -n 1)
    if [[ -z $tag ]]; then
      echo "::error::no $ref.x release of $repo" >&2
      exit 1
    fi
    requested=$tag
  else
    requested=latest
  fi
fi
if [[ $requested == latest ]]; then
  requested=$(gh api "repos/$repo/releases/latest" -q .tag_name)
fi
tag=v${requested#v}
version=${tag#v}

case $RUNNER_OS in
  Linux) os=Linux ext=tar.gz bin=zapp ;;
  macOS) os=Darwin ext=tar.gz bin=zapp ;;
  Windows) os=Windows ext=zip bin=zapp.exe ;;
  *) echo "::error::zapp has no build for $RUNNER_OS" >&2; exit 1 ;;
esac
case $RUNNER_ARCH in
  X64) arch=x86_64 ;;
  ARM64) arch=arm64 ;;
  *) echo "::error::zapp has no build for $RUNNER_ARCH" >&2; exit 1 ;;
esac
asset=zapp_${os}_${arch}.${ext}

dir=$(native "${RUNNER_TOOL_CACHE:-$RUNNER_TEMP}")/zapp/$version/$arch
if [[ ! -x $dir/$bin ]]; then
  download=$(native "$RUNNER_TEMP")/zapp-download-$$
  rm -rf "$download"
  mkdir -p "$download"
  gh release download "$tag" --repo "$repo" --dir "$download" \
    --pattern "$asset" --pattern "zapp_${version}_checksums.txt"
  (
    cd "$download"
    grep " $asset\$" "zapp_${version}_checksums.txt" >expected
    if command -v sha256sum >/dev/null; then sha256sum -c expected; else shasum -a 256 -c expected; fi
  )
  rm -rf "$dir"
  mkdir -p "$dir"
  if [[ $ext == zip ]]; then
    unzip -q "$download/$asset" -d "$dir"
  else
    tar -xzf "$download/$asset" -C "$dir"
  fi
  rm -rf "$download"
fi

echo "$dir" >>"$GITHUB_PATH"
echo "zapp $version installed at $dir/$bin"
output version "$version"
output path "$dir/$bin"
