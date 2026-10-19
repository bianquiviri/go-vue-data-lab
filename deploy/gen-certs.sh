#!/usr/bin/env bash
# Generates locally-trusted TLS certificates with mkcert, writing them into
# deploy/certs/ (git-ignored).
#
# Hosts (mapped to 127.0.0.1 in /etc/hosts):
#   apidatago.josebianco.local   -> backend Go API
#   frontgodata.josebianco.local -> Vite dev server
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

DOMAINS=(
	"apidatago.josebianco.local"
	"frontgodata.josebianco.local"
)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERT_DIR="${SCRIPT_DIR}/certs"

if ! command -v mkcert >/dev/null 2>&1; then
	echo "error: mkcert not found in PATH. Install it with: brew install mkcert nss" >&2
	exit 1
fi

mkdir -p "$CERT_DIR"

echo "[gen-certs] installing local CA (mkcert -install)"
mkcert -install

for DOMAIN in "${DOMAINS[@]}"; do
	echo "[gen-certs] generating certificate for ${DOMAIN}"
	mkcert \
		-key-file "${CERT_DIR}/${DOMAIN}-key.pem" \
		-cert-file "${CERT_DIR}/${DOMAIN}.pem" \
		"${DOMAIN}" "localhost" "127.0.0.1" "::1"
done

echo "[gen-certs] done. Certificates written to ${CERT_DIR}"
ls -1 "$CERT_DIR"
