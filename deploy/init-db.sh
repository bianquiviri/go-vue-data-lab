#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
    CREATE DATABASE lab_source;
    CREATE DATABASE lab_target;
EOSQL
