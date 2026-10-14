#!/usr/bin/env bash
# Generates locally-trusted TLS certificates for https://godata.josebianco.local
# using mkcert, writing them into deploy/certs/ (git-ignored).
#
# Usage:
#     ./deploy/gen-certs.sh      # or: make certs
#
# Prerequisites:
#     brew install mkcert nss    # nss only needed for Firefox
#
# `mkcert -install` creates a local CA and installs it into the system trust
# store (may prompt for your password / Touch ID on macOS). It is idempotent.
set -euo pipefail

DOMAIN="godata.josebianco.local"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERT_DIR="${SCRIPT_DIR}/certs"

if ! command -v mkcert >/dev/null 2>&1; then
	echo "error: mkcert not found in PATH. Install it with: brew install mkcert nss" >&2
	exit 1
fi

mkdir -p "$CERT_DIR"

echo "[gen-certs] installing local CA (mkcert -install)"
mkcert -install

echo "[gen-certs] generating certificate for ${DOMAIN}"
mkcert \
	-key-file "${CERT_DIR}/${DOMAIN}-key.pem" \
	-cert-file "${CERT_DIR}/${DOMAIN}.pem" \
	"${DOMAIN}" localhost 127.0.0.1 ::1

echo "[gen-certs] done. Certificates written to ${CERT_DIR}"
ls -1 "$CERT_DIR"
