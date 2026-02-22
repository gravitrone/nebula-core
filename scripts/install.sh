#!/usr/bin/env bash
set -euo pipefail

NEBULA_HOME="${NEBULA_HOME:-$HOME/.nebula}"
NEBULA_CHANNEL="${NEBULA_CHANNEL:-stable}"
NEBULA_VERSION="${NEBULA_VERSION:-latest}"
MANIFEST_ROOT="${NEBULA_MANIFEST_ROOT:-https://nebula.gravitrone.com/channels}"
PRIMARY_MANIFEST_URL="${NEBULA_MANIFEST_URL:-${MANIFEST_ROOT}/${NEBULA_CHANNEL}/manifest.json}"
FALLBACK_MANIFEST_URL="${NEBULA_FALLBACK_MANIFEST_URL:-${MANIFEST_ROOT}/${NEBULA_CHANNEL}/manifest.previous.json}"

BIN_DIR="$NEBULA_HOME/bin"
RUNTIME_ROOT="$NEBULA_HOME/runtime"
RELEASES_DIR="$RUNTIME_ROOT/releases"
CURRENT_LINK="$RUNTIME_ROOT/current"
CACHE_DIR="$NEBULA_HOME/cache"
DATA_DIR="$NEBULA_HOME/data/postgres"
LOG_DIR="$NEBULA_HOME/logs"

RESOLVED_VERSION=""
CLI_URL=""
CLI_SHA=""
COMPOSE_URL=""
COMPOSE_SHA=""

print_box() {
  local title="$1"
  local body="$2"
  local width=92
  local rule
  rule=$(printf '%*s' "$width" '' | tr ' ' '─')
  printf '╭%s╮\n' "${rule}"
  printf '│ %-88s │\n' "[ ${title} ]"
  printf '├%s┤\n' "${rule}"
  while IFS= read -r line; do
    printf '│ %-88s │\n' "$line"
  done <<<"$body"
  printf '╰%s╯\n' "${rule}"
}

info() {
  print_box "$1" "$2"
}

warn() {
  print_box "warning" "$1"
}

fail() {
  print_box "error" "$1"
  exit 1
}

require_cmd() {
  local bin="$1"
  local hint="$2"
  if ! command -v "$bin" >/dev/null 2>&1; then
    fail "$bin is required.\n$hint"
  fi
}

require_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    fail "docker is required for nebula install.\ninstall docker desktop, start it, then rerun install.sh"
  fi

  if ! docker info >/dev/null 2>&1; then
    fail "docker daemon is not running.\nstart docker desktop and rerun install.sh"
  fi

  if ! docker compose version >/dev/null 2>&1; then
    fail "docker compose plugin is required.\nupdate docker desktop and rerun install.sh"
  fi
}

platform_key() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m | tr '[:upper:]' '[:lower:]')"

  case "$arch" in
    x86_64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
  esac

  printf '%s_%s' "$os" "$arch"
}

sha256_file() {
  local path="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$path" | awk '{print $1}'
    return
  fi

  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$path" | awk '{print $1}'
    return
  fi

  if command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$path" | awk '{print $2}'
    return
  fi

  fail "could not find sha256 tool.\ninstall coreutils (sha256sum) or openssl."
}

verify_checksum() {
  local path="$1"
  local expected="$2"
  local actual

  actual="$(sha256_file "$path")"
  if [[ "$actual" != "$expected" ]]; then
    fail "checksum mismatch for $(basename "$path").\nexpected: $expected\nactual:   $actual"
  fi
}

fetch_manifest() {
  local out="$1"
  if curl -fsSL "$PRIMARY_MANIFEST_URL" -o "$out"; then
    return
  fi

  warn "primary manifest fetch failed at:\n$PRIMARY_MANIFEST_URL\nfalling back to:\n$FALLBACK_MANIFEST_URL"

  if ! curl -fsSL "$FALLBACK_MANIFEST_URL" -o "$out"; then
    fail "could not fetch release manifest from primary or fallback urls.\nprimary: $PRIMARY_MANIFEST_URL\nfallback: $FALLBACK_MANIFEST_URL"
  fi
}

resolve_manifest_fields() {
  local manifest_path="$1"
  local platform="$2"
  local requested_version="$3"
  local tmp

  tmp="$(mktemp)"

  if ! python3 - "$manifest_path" "$platform" "$requested_version" >"$tmp" <<'PY'; then
import json
import sys

path, platform, requested = sys.argv[1:]

with open(path, "r", encoding="utf-8") as fh:
    manifest = json.load(fh)

if manifest.get("schema_version") != 1:
    raise SystemExit("manifest schema_version must be 1")

releases = manifest.get("releases")
if not isinstance(releases, dict):
    raise SystemExit("manifest.releases must be an object")

latest = releases.get("latest")
if not isinstance(latest, str) or not latest:
    raise SystemExit("manifest.releases.latest is required")

versions = releases.get("versions")
if not isinstance(versions, dict):
    raise SystemExit("manifest.releases.versions must be an object")

version = latest if requested in ("", "latest") else requested
entry = versions.get(version)
if not isinstance(entry, dict):
    raise SystemExit(f"release version not found: {version}")

cli = entry.get("cli", {})
platform_entry = cli.get(platform)
if not isinstance(platform_entry, dict):
    raise SystemExit(f"cli artifact missing for platform {platform}")

compose = entry.get("compose")
if not isinstance(compose, dict):
    raise SystemExit("compose artifact missing")

required = {
    "CLI_URL": platform_entry.get("url"),
    "CLI_SHA": platform_entry.get("sha256"),
    "COMPOSE_URL": compose.get("url"),
    "COMPOSE_SHA": compose.get("sha256"),
}

for key, value in required.items():
    if not isinstance(value, str) or not value.strip():
        raise SystemExit(f"{key} missing in manifest")

print(f"RESOLVED_VERSION={version}")
for key, value in required.items():
    print(f"{key}={value}")
PY
    rm -f "$tmp"
    fail "manifest validation failed.\ncheck manifest format and release artifact links."
  fi

  # shellcheck disable=SC1090
  source "$tmp"
  rm -f "$tmp"
}

prepare_dirs() {
  mkdir -p "$BIN_DIR" "$RELEASES_DIR" "$CACHE_DIR" "$DATA_DIR" "$LOG_DIR"
}

download_artifacts() {
  local cli_archive="$1"
  local compose_archive="$2"

  info "download" "fetching release artifacts for $RESOLVED_VERSION"
  curl -fsSL "$CLI_URL" -o "$cli_archive"
  curl -fsSL "$COMPOSE_URL" -o "$compose_archive"

  verify_checksum "$cli_archive" "$CLI_SHA"
  verify_checksum "$compose_archive" "$COMPOSE_SHA"
}

install_cli() {
  local cli_archive="$1"
  local extract_dir

  extract_dir="$(mktemp -d)"
  tar -xzf "$cli_archive" -C "$extract_dir"

  local bin_path
  bin_path="$(find "$extract_dir" -type f -name nebula | head -n 1 || true)"
  if [[ -z "$bin_path" ]]; then
    rm -rf "$extract_dir"
    fail "nebula binary not found in release archive"
  fi

  install -m 755 "$bin_path" "$BIN_DIR/nebula"
  rm -rf "$extract_dir"
}

install_runtime_bundle() {
  local compose_archive="$1"
  local release_dir="$RELEASES_DIR/$RESOLVED_VERSION"
  local env_path

  rm -rf "$release_dir"
  mkdir -p "$release_dir"
  tar -xzf "$compose_archive" -C "$release_dir"

  if [[ ! -f "$release_dir/compose.yaml" ]]; then
    fail "compose.yaml missing from runtime bundle"
  fi

  ln -sfn "$release_dir" "$CURRENT_LINK"
  env_path="$CURRENT_LINK/.env"

  if [[ ! -f "$env_path" ]]; then
    if [[ -f "$CURRENT_LINK/.env.example" ]]; then
      cp "$CURRENT_LINK/.env.example" "$env_path"
    else
      cat >"$env_path" <<ENV
POSTGRES_DB=nebula
POSTGRES_USER=nebula
POSTGRES_PASSWORD=nebula_local_change_me
POSTGRES_PORT=6432
ADMINER_PORT=8080
NEBULA_DATA_DIR=$DATA_DIR
ENV
    fi
  fi

  if ! grep -q '^NEBULA_DATA_DIR=' "$env_path"; then
    printf '\nNEBULA_DATA_DIR=%s\n' "$DATA_DIR" >>"$env_path"
  fi

  if [[ -n "${NEBULA_POSTGRES_PORT:-}" ]]; then
    if grep -q '^POSTGRES_PORT=' "$env_path"; then
      sed -i.bak "s/^POSTGRES_PORT=.*/POSTGRES_PORT=${NEBULA_POSTGRES_PORT}/" "$env_path"
    else
      printf '\nPOSTGRES_PORT=%s\n' "$NEBULA_POSTGRES_PORT" >>"$env_path"
    fi
  fi

  if [[ -n "${NEBULA_ADMINER_PORT:-}" ]]; then
    if grep -q '^ADMINER_PORT=' "$env_path"; then
      sed -i.bak "s/^ADMINER_PORT=.*/ADMINER_PORT=${NEBULA_ADMINER_PORT}/" "$env_path"
    else
      printf '\nADMINER_PORT=%s\n' "$NEBULA_ADMINER_PORT" >>"$env_path"
    fi
  fi

  rm -f "${env_path}.bak"
}

start_stack() {
  local compose_file="$CURRENT_LINK/compose.yaml"
  local env_file="$CURRENT_LINK/.env"
  local up_log

  info "runtime" "starting nebula data stack"
  up_log="$(mktemp)"
  if ! docker compose --project-name nebula -f "$compose_file" --env-file "$env_file" up -d >"$up_log" 2>&1; then
    if grep -qi "port is already allocated" "$up_log"; then
      rm -f "$up_log"
      fail "port conflict detected while starting nebula stack.\\nstop other postgres/adminer services on ports 6432/8080, or set POSTGRES_PORT/ADMINER_PORT in ~/.nebula/runtime/current/.env and rerun install."
    fi
    local output
    output="$(cat "$up_log")"
    rm -f "$up_log"
    fail "docker compose failed to start nebula stack.\\n$output"
  fi
  rm -f "$up_log"

  local tries=60
  local n
  for ((n=1; n<=tries; n++)); do
    local container
    container="$(docker compose --project-name nebula -f "$compose_file" --env-file "$env_file" ps -q postgres 2>/dev/null || true)"
    if [[ -n "$container" ]]; then
      local health
      health="$(docker inspect --format='{{.State.Health.Status}}' "$container" 2>/dev/null || true)"
      if [[ "$health" == "healthy" ]]; then
        return
      fi
    fi
    sleep 2
  done

  fail "postgres did not become healthy in time.\nrun: docker compose --project-name nebula -f \"$compose_file\" --env-file \"$env_file\" logs postgres"
}

print_success() {
  local path_hint
  path_hint="export PATH=\"$BIN_DIR:\$PATH\""

  info "success" "nebula $RESOLVED_VERSION installed.\ncli path: $BIN_DIR/nebula\nruntime: $CURRENT_LINK\nif nebula command is not found, run:\n$path_hint"
}

main() {
  require_cmd curl "install curl and rerun"
  require_cmd tar "install tar and rerun"
  require_cmd python3 "python3 is required to parse release manifests"
  require_docker

  prepare_dirs

  local platform manifest_file cli_archive compose_archive
  platform="$(platform_key)"
  manifest_file="$CACHE_DIR/manifest-${NEBULA_CHANNEL}.json"
  cli_archive="$CACHE_DIR/nebula-${NEBULA_CHANNEL}-${platform}.tar.gz"
  compose_archive="$CACHE_DIR/nebula-runtime-${NEBULA_CHANNEL}.tar.gz"

  info "nebula installer" "channel: $NEBULA_CHANNEL\nrequested version: $NEBULA_VERSION\nplatform: $platform"

  fetch_manifest "$manifest_file"
  resolve_manifest_fields "$manifest_file" "$platform" "$NEBULA_VERSION"

  download_artifacts "$cli_archive" "$compose_archive"
  install_cli "$cli_archive"
  install_runtime_bundle "$compose_archive"
  start_stack
  print_success
}

main "$@"
