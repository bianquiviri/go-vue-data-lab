#!/bin/bash
# Creates the two application databases used by the lab: lab_source and lab_target.
#
# This script is mounted into /docker-entrypoint-initdb.d/ and is executed by the
# official postgres image on first initialization (empty data directory). It is
# written to be idempotent so it can also be run manually without failing if the
# databases already exist:
#
#     docker compose exec postgres /docker-entrypoint-initdb.d/init-db.sh
#
# Requires POSTGRES_USER (and optionally POSTGRES_DB) from the container env.
set -euo pipefail

: "${POSTGRES_USER:=postgres}"
: "${POSTGRES_DB:=postgres}"

psql -v ON_ERROR_STOP=1 \
	--username "$POSTGRES_USER" \
	--dbname "$POSTGRES_DB" <<-EOSQL
	SELECT 'CREATE DATABASE lab_source'
	WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'lab_source')\gexec

	SELECT 'CREATE DATABASE lab_target'
	WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'lab_target')\gexec
EOSQL

echo "[init-db] ensured databases: lab_source, lab_target"
