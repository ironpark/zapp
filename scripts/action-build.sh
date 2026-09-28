#!/usr/bin/env bash
# Runs `zapp build` for the zapp GitHub Action from its IN_* inputs.
#
# Inputs reach zapp through the ZAPP_* variables its flags read, and only when
# they are set: an empty variable would still override the project file.
# Secrets therefore never appear on a command line, and file-shaped secrets
# are written to a private directory removed when the step ends.
set -euo pipefail

# Every input is optional.
: "${IN_CONFIG:=}"
: "${IN_STEPS:=}"
: "${IN_APP:=}"
: "${IN_SIGN:=}"
: "${IN_NOTARIZE:=}"
: "${IN_STAPLE:=}"
: "${IN_IDENTITY:=}"
: "${IN_ENTITLEMENTS:=}"
: "${IN_CERTIFICATE:=}"
: "${IN_CERTIFICATE_PASSWORD:=}"
: "${IN_PEM:=}"
: "${IN_API_KEY:=}"
: "${IN_API_KEY_ID:=}"
: "${IN_API_ISSUER_ID:=}"
: "${IN_API_PRIVATE_KEY:=}"
: "${IN_APPLE_ID:=}"
: "${IN_APP_PASSWORD:=}"
: "${IN_TEAM_ID:=}"
: "${IN_ARGS:=}"
: "${IN_ZIP:=}"
: "${IN_CHECKSUMS:=}"
: "${IN_UPLOAD:=}"
: "${IN_UPLOAD_URL:=}"
: "${IN_UPLOAD_METHOD:=}"
: "${IN_UPLOAD_FIELD:=}"
: "${IN_UPLOAD_HEADERS:=}"
: "${IN_UPLOAD_ARTIFACTS:=}"
: "${IN_GITHUB_RELEASE:=}"
: "${IN_GITHUB_TOKEN:=}"

native() {
  if command -v cygpath >/dev/null; then cygpath -m "$1"; else printf '%s\n' "$1"; fi
}

set_env() {
  if [[ -n $2 ]]; then export "$1=$2"; fi
}

private=$(native "$RUNNER_TEMP")/zapp-action-$$
rm -rf "$private"
mkdir -p "$private"
chmod 700 "$private"
trap 'rm -rf "$private"' EXIT

# secret_file writes a secret input to a private file and names it in var.
secret_file() {
  local file=$private/$2
  (umask 077 && printf '%s\n' "$3" >"$file")
  export "$1=$file"
}

# Bash 3.2, still /bin/bash on macOS, has no ${var,,}.
lower() { printf '%s' "$1" | tr '[:upper:]' '[:lower:]'; }

bool() {
  case $(lower "$2") in
    '') ;;
    true) export "ZAPP_$1=true" ;;
    false) export "ZAPP_NO_$1=true" ;;
    *) echo "::error::$3 must be true or false, not $2" >&2; exit 1 ;;
  esac
}

set_env ZAPP_CONFIG "$IN_CONFIG"
set_env ZAPP_APP "$IN_APP"
bool SIGN "$IN_SIGN" sign
bool NOTARIZE "$IN_NOTARIZE" notarize
# setting passes true or false through as ZAPP_<name>, overriding the project.
setting() {
  case $(lower "$2") in
    '') ;;
    true | false) export "ZAPP_$1=$(lower "$2")" ;;
    *) echo "::error::$3 must be true or false, not $2" >&2; exit 1 ;;
  esac
}
setting STAPLE "$IN_STAPLE" staple

set_env ZAPP_IDENTITY "$IN_IDENTITY"
set_env ZAPP_ENTITLEMENTS "$IN_ENTITLEMENTS"
set_env ZAPP_P12_BASE64 "$(printf '%s' "$IN_CERTIFICATE" | tr -d '[:space:]')"
set_env ZAPP_P12_PASSWORD "$IN_CERTIFICATE_PASSWORD"
if [[ -n $IN_PEM ]]; then
  secret_file ZAPP_PEM_FILE signing.pem "$IN_PEM"
fi

if [[ -n $IN_API_KEY ]]; then
  secret_file ZAPP_API_KEY_FILE api-key.json "$IN_API_KEY"
elif [[ -n $IN_API_KEY_ID || -n $IN_API_ISSUER_ID || -n $IN_API_PRIVATE_KEY ]]; then
  if [[ -z $IN_API_KEY_ID || -z $IN_API_ISSUER_ID || -z $IN_API_PRIVATE_KEY ]]; then
    echo "::error::api-key-id, api-issuer-id and api-private-key go together" >&2
    exit 1
  fi
  # The same JSON rcodesign encode-app-store-connect-api-key writes. A PEM
  # key holds nothing that needs escaping in JSON except its line breaks.
  key=${IN_API_PRIVATE_KEY//$'\r'/}
  key=${key//$'\n'/\\n}
  secret_file ZAPP_API_KEY_FILE api-key.json \
    "{\"issuer_id\":\"$IN_API_ISSUER_ID\",\"key_id\":\"$IN_API_KEY_ID\",\"private_key\":\"$key\"}"
fi
setting ZIP "$IN_ZIP" zip
setting CHECKSUMS "$IN_CHECKSUMS" checksums
bool UPLOAD "$IN_UPLOAD" upload
set_env ZAPP_UPLOAD_URL "$IN_UPLOAD_URL"
set_env ZAPP_UPLOAD_METHOD "$IN_UPLOAD_METHOD"
set_env ZAPP_UPLOAD_FIELD "$IN_UPLOAD_FIELD"
set_env ZAPP_UPLOAD_HEADER "${IN_UPLOAD_HEADERS//$'\r'/}"
set_env ZAPP_UPLOAD_ARTIFACTS "$IN_UPLOAD_ARTIFACTS"
set_env ZAPP_GITHUB_RELEASE "$IN_GITHUB_RELEASE"
# For GitHub release uploads, the project's included, unless the job set
# its own.
set_env GITHUB_TOKEN "${GITHUB_TOKEN:-$IN_GITHUB_TOKEN}"

set_env ZAPP_APPLE_ID "$IN_APPLE_ID"
set_env ZAPP_PASSWORD "$IN_APP_PASSWORD"
set_env ZAPP_TEAM_ID "$IN_TEAM_ID"

# --artifacts arrived with --version, in 1.1.0.
if ! zapp --version >/dev/null 2>&1; then
  echo "::error::the zapp action needs zapp 1.1.0 or later; use ironpark/zapp/setup for older releases" >&2
  exit 1
fi

# An older zapp would ignore the variables of a newer feature and quietly
# build without it.
help=$(zapp build --help 2>/dev/null || true)
if [[ -n $IN_ZIP$IN_UPLOAD$IN_UPLOAD_URL && $help != *--upload-url* ]]; then
  echo "::error::zip and upload need zapp 1.2.0 or later" >&2
  exit 1
fi
if [[ -n $IN_ENTITLEMENTS$IN_CHECKSUMS$IN_GITHUB_RELEASE && $help != *--github-release* ]]; then
  echo "::error::entitlements, checksums and github-release need zapp 1.3.0 or later" >&2
  exit 1
fi

args=(build --artifacts "$private/artifacts")
extra=()
while IFS= read -r line; do
  line=${line%$'\r'}
  [[ -n ${line//[[:space:]]/} ]] && extra+=("$line")
done <<<"$IN_ARGS"
steps=()
read -r -a steps <<<"$IN_STEPS"
# Bash 3.2 treats an empty array as unset under set -u.
zapp "${args[@]}" ${extra[@]+"${extra[@]}"} ${steps[@]+"${steps[@]}"}

# On Windows zapp reports D:\dir\file; the mixed D:/dir/file form works in
# bash steps as well as PowerShell and the artifact actions.
summary=$'### zapp\n\n| Artifact | Location |\n| --- | --- |\n'
while IFS='=' read -r name value; do
  if [[ -n $value && $name != *-url ]]; then
    value=$(native "$value")
  fi
  if [[ -n $value ]]; then
    summary+="| $name | \`$value\` |"$'\n'
  fi
  echo "$name=$value" >>"$GITHUB_OUTPUT"
done <"$private/artifacts"
printf '%s' "$summary" >>"$GITHUB_STEP_SUMMARY"
