#!/usr/bin/env python3

import argparse
import os
import time
import uuid
from io import BytesIO
from urllib.parse import quote

import psycopg2
import psycopg2.extras
import requests
from nacl.public import PrivateKey

from generate_data import create_from_lorem, encrypt_data, get_vault_token, wait_for_port, get_public_key, get_keystone_token


keystone_base_url = os.environ.get("KEYSTONE_BASE_URL", "http://127.0.0.1:5001")
vault_base_url = os.environ.get("VAULT_ADDR", "http://127.0.0.1:8200")
aai_base_url = os.environ.get("AAI_BASE_URL")
email = os.environ.get("USER_EMAIL")

dataset_sql = """INSERT INTO sda.datasets(stable_id)
                 VALUES(%s) RETURNING id;"""

file_sql = """INSERT INTO sda.files (stable_id, submission_user, submission_file_path, archive_file_path, archive_file_size)
              VALUES (%s, %s, %s, %s, %s) RETURNING id;"""

file_dataset_sql = """INSERT INTO sda.file_dataset (file_id, dataset_id) \
                      VALUES (%s, %s)"""

file_event_sql = """INSERT INTO sda.file_event_log (file_id, event) \
                    VALUES (%s, 'ready')"""


def run(
    swift_url: str,
    swift_token: str,
    vault_token: str,
    project_pubkey: bytes,
    project: str,
    container: str,
    target: str,
    db_string: str,
    n_datasets_for_user: int,
    n_datasets: int,
    n_objects: int,
    timeout: int,
):
    data = create_from_lorem(n_datasets, n_objects, True)
    seckey = bytes(PrivateKey.generate())

    try:
        conn = psycopg2.connect(db_string)
        conn.autocommit = True
        psycopg2.extras.register_uuid()
    except (Exception) as error:
        print(f'Unable to connect to the database {db_string}: {error}')

        return

    file_no = 1

    for ds in data:
        dataset_name = ds["name"]
        if target == "sdapply":
            dataset_name = "EGA" + dataset_name
        else:
            dataset_name = "https://bp-" + dataset_name + ".org"

        if n_datasets_for_user > 0:
            n_datasets_for_user -= 1

            # add dataset visa
            with requests.post(
                f'{aai_base_url}/api/jwk/{target}/' + quote(dataset_name, safe=''),
                timeout=timeout,
            ) as response:
                if response.status_code >= 400:
                    print("Failed to send dataset to mockauth: " + response.text)
        else:
            dataset_name += "-invalid" # The user should not be able to access these

        dataset_id = None
        with conn.cursor() as cur:
            try:
                # create dataset in db
                cur.execute(dataset_sql, (dataset_name,))

                # get the generated id back
                rows = cur.fetchone()
                if rows:
                    dataset_id = rows[0]
            except (Exception, psycopg2.DatabaseError) as error:
                print(f"Failed to add dataset {dataset_name} to database: {error}")
                break

        print(f'Dataset:\t {dataset_name}')

        for file in ds["objects"]:
            object_file = BytesIO(file["content"].encode('utf-8'))

            archive_path = f'{uuid.uuid4()}'
            submission_path = email + "/" + file["name"]

            b64_encoded_header, encrypted_data = encrypt_data(object_file, seckey, project_pubkey, True)
            # send header to vault
            with requests.post(
                f'{vault_base_url}/v1/c4ghtransit/files/{project}/{container}/{archive_path}',
                headers={"X-Vault-Token": f"{vault_token}"},
                json={"header": b64_encoded_header},
                timeout=timeout,
            ) as response:
                if response.status_code >= 400:
                    print("Failed to send header to vault: " + response.text)

            url = (
                swift_url
                + f'/{container}'
                + f'/{archive_path}'
            )
            # create object
            upload_data_response = requests.put(
                url,
                headers={"X-Auth-Token": swift_token, "Content-Type": "text/plain"},
                data=encrypted_data,
                allow_redirects=True,
                timeout=5.0,
            )
            if upload_data_response.status_code in {201, 202}:
                print(f'Object:\t\t {upload_data_response.status_code} {file["name"]}')
            else:
                print(f'Object:\t\tError {upload_data_response.status_code} {file["name"]}')
                response.raise_for_status()

            file_id = None
            with conn.cursor() as cur:
                try:
                    # create file in db
                    cur.execute(file_sql, (f'file_stable_id_{file_no}', email, submission_path, archive_path, len(encrypted_data),))
                    file_no += 1

                    # get the generated id back
                    rows = cur.fetchone()
                    if rows:
                        file_id = rows[0]

                    cur.execute(file_dataset_sql, (file_id, dataset_id,))
                    cur.execute(file_event_sql, (file_id,))
                except (Exception, psycopg2.DatabaseError) as error:
                    print(f"Failed to add file {file["name"]} to database: {error}")
                    break

    conn.close()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="Generate objects in swift object storage under a single bucket and fill in a sda DB accordingly.",
    )
    parser.add_argument(
        "--project", default="sdapply-project", help="Keystone project. Defaults to (sdapply-project)"
    )
    parser.add_argument(
        "--target", default="sdapply", help="Value that mockauth can use to recognise issuer. Defaults to (sdapply)"
    )
    parser.add_argument(
        "--container", help="The container to which all the data is uploaded"
    )
    parser.add_argument(
        "--db-string", help="Connection string to the sda DB"
    )
    parser.add_argument(
        "--datasets", type=int, default=3, help="Number of datasets to create that user can access"
    )
    parser.add_argument(
        "--all-datasets", type=int, default=6, help="Number of datasets to create that will exist in the database"
    )
    parser.add_argument(
        "--files", type=int, default=15, help="Number of files per dataset to create"
    )

    parser.add_argument(
        "--timeout",
        type=int,
        default=60,
        help="Maximum time to wait before data is generated and metadata is updated, for each run",
    )

    args = parser.parse_args()
    total_start = time.perf_counter()

    username = os.environ.get("CSC_USERNAME", "swift")
    password = os.environ.get("CSC_PASSWORD", "veryfast")

    wait_for_port(keystone_base_url, timeout=args.timeout)
    swift_base_url, keystone_token = get_keystone_token(
        username, password, args.project
    )

    wait_for_port(vault_base_url, timeout=args.timeout)
    vault_token = get_vault_token()
    public_key = get_public_key(vault_token, args.project)

    print("Uploading...")

    run(
        swift_base_url,
        keystone_token,
        vault_token,
        public_key,
        args.project,
        args.container,
        args.target,
        args.db_string,
        args.datasets,
        args.all_datasets,
        args.files,
        args.timeout,
    )

    total_end = time.perf_counter() - total_start
    print()
    print(f"Completed in {total_end:.2f} seconds")
