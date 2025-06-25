#!/bin/bash -eu

credentials=$(docker exec keystone-swift bash -c 'openstack container create ${FINDATA_BUCKET}')
credentials=$(docker exec keystone-swift bash -c 'openstack ec2 credentials create')

cat <<EOF > /app/env/.env.findata
FINDATA_ACCESS=$(echo "${credentials}" | grep access | awk '{print $4}')
FINDATA_SECRET=$(echo "${credentials}" | grep secret | awk '{print $4}')

EOF
