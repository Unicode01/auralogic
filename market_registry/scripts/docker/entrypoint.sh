#!/bin/sh

set -eu

DATA_ROOT="${MARKET_REGISTRY_DOCKER_DATA_ROOT:-/app/data}"
: "${MARKET_REGISTRY_DATA_DIR:=${DATA_ROOT}/data}"
: "${MARKET_REGISTRY_KEY_DIR:=${DATA_ROOT}/keys}"
: "${MARKET_REGISTRY_ADDR:=:18080}"
: "${MARKET_REGISTRY_KEY_ID:=official-2026-01}"

mkdir -p "${MARKET_REGISTRY_DATA_DIR}" "${MARKET_REGISTRY_KEY_DIR}"

if [ "$#" -gt 0 ]; then
  exec "$@"
fi

PRIV_KEY_PATH="${MARKET_REGISTRY_KEY_DIR}/${MARKET_REGISTRY_KEY_ID}.key"
PUB_KEY_PATH="${MARKET_REGISTRY_KEY_DIR}/${MARKET_REGISTRY_KEY_ID}.pub"

if [ ! -f "${PRIV_KEY_PATH}" ] || [ ! -f "${PUB_KEY_PATH}" ]; then
  echo "[INFO] Signing key pair ${MARKET_REGISTRY_KEY_ID} is missing, generating it now..."
  market-registry-cli keygen --key-id "${MARKET_REGISTRY_KEY_ID}"
fi

exec market-registry-api
