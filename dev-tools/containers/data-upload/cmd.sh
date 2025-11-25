#!/bin/bash -eu

python generate_data.py
python generate_data.py --project "${SDAPPLY_PROJECT}" --headerless

python fill_database.py --project "${SDAPPLY_PROJECT}" \
    --target sdapply \
    --container "${FEGA_BUCKET}" \
    --db-string "${DB_STRING_SDA_FEGA}"
python fill_database.py --project "${BIGPICTURE_PROJECT}" \
    --target bigpicture \
    --container "${BIGPICTURE_BUCKET}" \
    --db-string "${DB_STRING_SDA_BP}"
