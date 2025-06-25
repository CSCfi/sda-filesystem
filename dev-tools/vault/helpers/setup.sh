#!/bin/sh

set -e

# This script applies the pre-defined vault policy and role with pre-defined token

# Dependencies
# - wget
# - vault

# get the current scripts folder
SCRIPT=$(realpath "$0")
SCRIPTS=$(dirname "$SCRIPT")

export VAULT_ADDR="${VAULT_ADDR:-'http://127.0.0.1:8200'}"
VAULT_ROLE=${VAULT_ROLE:-sdsi}
VAULT_SECRET=${VAULT_SECRET:-sdsi-secret-token}

until wget -o /dev/null -q --spider "${VAULT_ADDR}/v1/sys/health?standbyok=true"; do
    sleep 1
done

vault login token="${VAULT_DEV_ROOT_TOKEN_ID}"

if vault read auth/approle/role/"$VAULT_ROLE" >/dev/null 2>&1; then
    exit 0
fi

vault auth enable approle
vault secrets enable c4ghtransit
vault policy write "$VAULT_ROLE" "$SCRIPTS"/vault_policy.hcl
vault write auth/approle/role/"$VAULT_ROLE" \
    secret_id_ttl=0 \
    secret_id_num_uses=0 \
    token_ttl=5m \
    token_max_ttl=5m \
    token_num_uses=0 \
    token_policies="$VAULT_ROLE" \
    role_id="$VAULT_ROLE"
vault write -format=json -f auth/approle/role/"$VAULT_ROLE"/custom-secret-id secret_id="$VAULT_SECRET"
