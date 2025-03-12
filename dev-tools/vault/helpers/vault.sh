#!/bin/sh

set -e

# This script starts and configures a vault server in development mode with c4ghtransit plugin enabled

# Dependencies
# - wget
# - vault
# - vault c4gh-transit plugin

# path to the c4gh-transit plugin repository
C4GH_TRANSIT_DIR=${C4GH_TRANSIT_DIR:-/tmp/plugins}

VAULT_ROLE=${VAULT_ROLE:-sdsi}
VAULT_SECRET=${VAULT_SECRET:-sdsi-secret-token}

# The loaded plugin is built for linux so have to run in docker to be compatible with macOS
mkdir -p "${C4GH_TRANSIT_DIR}"
wget \
    -O "${C4GH_TRANSIT_DIR}/c4ghtransit" \
    "https://${ARTIFACTORY_USER}:${ARTIFACTORY_USER_PASSWORD}@REDACTED:443/artifactory/sds-generic-local/c4gh-transit/c4ghtransit"
chmod +x "${C4GH_TRANSIT_DIR}/c4ghtransit"

# start vault server in development mode
VAULT_LOG_LEVEL=DEBUG exec vault server -dev -dev-plugin-dir="${C4GH_TRANSIT_DIR}" -dev-root-token-id="${VAULT_DEV_ROOT_TOKEN_ID}"
