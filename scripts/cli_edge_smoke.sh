#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLI_DIR="${ROOT_DIR}/cli/src"
BINARY="${CLI_DIR}/build/nebula"
CONFIG_PATH="${HOME}/.nebula/config"
API_BASE="${NEBULA_SMOKE_API_BASE:-http://127.0.0.1:8765}"
INVALID_KEY="nbl_invalid_cli_edge_smoke"

BASIC_EXPECT="${ROOT_DIR}/scripts/cli_basic_smoke.expect"
RELOGIN_EXPECT="${ROOT_DIR}/scripts/cli_relogin_smoke.expect"

CYCLES="${1:-25}"
TIMEOUT_SECONDS="${NEBULA_SMOKE_TIMEOUT_SECONDS:-20}"
ALLOW_PORT_8000="${NEBULA_SMOKE_ALLOW_PORT_8000:-0}"

LOG_DIR="${ROOT_DIR}/.tmp/cli-edge-smoke"
RUN_LOG="${LOG_DIR}/run-$(date +%Y%m%d-%H%M%S)"
BACKUP_CONFIG="${RUN_LOG}/config.backup"

mkdir -p "${RUN_LOG}"

if ! [[ "${CYCLES}" =~ ^[0-9]+$ ]] || [[ "${CYCLES}" -le 0 ]]; then
  echo "invalid cycles value: ${CYCLES}" >&2
  echo "usage: ./scripts/cli_edge_smoke.sh [cycles]" >&2
  exit 2
fi

require_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "required command not found: ${cmd}" >&2
    exit 3
  fi
}

require_cmd expect
require_cmd curl
require_cmd awk
require_cmd sed
require_cmd lsof

for script in "${BASIC_EXPECT}" "${RELOGIN_EXPECT}"; do
  if [[ ! -x "${script}" ]]; then
    chmod +x "${script}"
  fi
done

if [[ ! -x "${BINARY}" ]]; then
  echo "missing binary: ${BINARY}" >&2
  echo "build it with: cd cli/src && go build -o build/nebula ./cmd/nebula" >&2
  exit 4
fi

if [[ ! -f "${CONFIG_PATH}" ]]; then
  echo "missing config: ${CONFIG_PATH}" >&2
  echo "run: ${BINARY} login" >&2
  exit 5
fi

cp "${CONFIG_PATH}" "${BACKUP_CONFIG}"
chmod 600 "${BACKUP_CONFIG}"

cleanup() {
  local rc="$1"
  if [[ "${rc}" -ne 0 && -f "${BACKUP_CONFIG}" ]]; then
    cp "${BACKUP_CONFIG}" "${CONFIG_PATH}"
    chmod 600 "${CONFIG_PATH}"
    echo "restored original config after failure" >&2
  fi
}

trap 'cleanup $?' EXIT

read_config_value() {
  local key="$1"
  awk -F': ' -v k="${key}" '$1 == k {print $2; exit}' "${CONFIG_PATH}" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//'
}

write_invalid_key() {
  local tmp
  tmp="$(mktemp)"
  awk -v v="${INVALID_KEY}" '
    BEGIN { done = 0 }
    $1 == "api_key:" {
      print "api_key: " v
      done = 1
      next
    }
    { print }
    END {
      if (done == 0) {
        print "api_key: " v
      }
    }
  ' "${CONFIG_PATH}" >"${tmp}"
  mv "${tmp}" "${CONFIG_PATH}"
  chmod 600 "${CONFIG_PATH}"
}

assert_health() {
  curl -fsS "${API_BASE}/api/health" >/dev/null
}

assert_no_rogue_port_8000() {
  if [[ "${ALLOW_PORT_8000}" == "1" ]]; then
    return
  fi

  if lsof -nP -iTCP:8000 -sTCP:LISTEN >/dev/null 2>&1; then
    echo "rogue listener detected on :8000 (set NEBULA_SMOKE_ALLOW_PORT_8000=1 to ignore)" >&2
    lsof -nP -iTCP:8000 -sTCP:LISTEN >&2 || true
    exit 6
  fi
}

assert_key_works() {
  local key="$1"
  if [[ -z "${key}" ]]; then
    echo "config api_key is empty" >&2
    exit 7
  fi
  if ! [[ "${key}" == nbl_* ]]; then
    echo "config api_key has unexpected format" >&2
    exit 8
  fi
  curl -fsS -H "Authorization: Bearer ${key}" "${API_BASE}/api/keys/" >/dev/null
}

echo "cli edge smoke: cycles=${CYCLES} api=${API_BASE}"
echo "logs: ${RUN_LOG}"

assert_health
assert_no_rogue_port_8000

starting_key="$(read_config_value "api_key")"
assert_key_works "${starting_key}"

passes=0
for ((i = 1; i <= CYCLES; i++)); do
  assert_health
  assert_no_rogue_port_8000

  basic_log="${RUN_LOG}/cycle-${i}-basic.log"
  if ! "${BASIC_EXPECT}" "${BINARY}" "${TIMEOUT_SECONDS}" "${basic_log}"; then
    echo "cycle ${i}: FAIL (basic flow) -> ${basic_log}" >&2
    exit 9
  fi

  write_invalid_key
  relogin_log="${RUN_LOG}/cycle-${i}-relogin.log"
  if ! "${RELOGIN_EXPECT}" "${BINARY}" "${TIMEOUT_SECONDS}" "${relogin_log}"; then
    echo "cycle ${i}: FAIL (relogin recovery) -> ${relogin_log}" >&2
    exit 10
  fi

  recovered_key="$(read_config_value "api_key")"
  if [[ "${recovered_key}" == "${INVALID_KEY}" ]]; then
    echo "cycle ${i}: FAIL (api_key did not rotate after relogin)" >&2
    exit 11
  fi
  assert_key_works "${recovered_key}"

  passes=$((passes + 1))
  echo "cycle ${i}/${CYCLES}: pass"
done

echo "cli edge smoke complete: ${passes}/${CYCLES} passed"
