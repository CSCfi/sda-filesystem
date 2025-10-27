#!/bin/bash -eu

env_file="/app/env/.env.static"

# Check if file exists
if [ -e "$env_file" ]; then
    exit 0
fi

## Use swift-project for findata
docker exec keystone-swift bash -c 'OS_PROJECT_NAME=swift-project && openstack container create ${FINDATA_BUCKET}'
credentials=$(docker exec keystone-swift bash -c 'OS_PROJECT_NAME=swift-project && openstack ec2 credentials create')
access=$(echo "${credentials}" | grep access | awk '{print $4}')
secret=$(echo "${credentials}" | grep secret | awk '{print $4}')

cat <<EOF > $env_file
FINDATA_ACCESS=${access}
FINDATA_SECRET=${secret}
EOF

## Create sdapply-project for SD Apply and FEGA
echo 'Creating SD Apply project'
docker exec keystone-swift bash -c '
openstack project create --domain default --description "SD Apply test project" sdapply-project
openstack role add --project sdapply-project --user swift admin
'

docker exec keystone-swift bash -c 'OS_PROJECT_NAME=sdapply-project && openstack container create ${FEGA_BUCKET}'

credentials=$(docker exec keystone-swift bash -c '
export OS_PROJECT_NAME=sdapply-project
openstack ec2 credentials create --project sdapply-project
')
access=$(echo "${credentials}" | grep access | awk '{print $4}')
secret=$(echo "${credentials}" | grep secret | awk '{print $4}')

cat <<EOF >> $env_file
SDAPPLY_ACCESS=${access}
SDAPPLY_SECRET=${secret}
EOF

## Create bp-project for Big Picture
echo 'Creating Big Picture project'
docker exec keystone-swift bash -c '
openstack project create --domain default --description "Big Picture test project" bp-project
openstack role add --project bp-project --user swift admin
'

docker exec keystone-swift bash -c 'OS_PROJECT_NAME=bp-project && openstack container create ${BIGPICTURE_BUCKET}'

credentials=$(docker exec keystone-swift bash -c '
export OS_PROJECT_NAME=bp-project
openstack ec2 credentials create --project bp-project
')
access=$(echo "${credentials}" | grep access | awk '{print $4}')
secret=$(echo "${credentials}" | grep secret | awk '{print $4}')

cat <<EOF >> $env_file
BIGPICTURE_ACCESS=${access}
BIGPICTURE_SECRET=${secret}

EOF
