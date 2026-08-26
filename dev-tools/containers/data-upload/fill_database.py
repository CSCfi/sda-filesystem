#!/usr/bin/env python3
"""Generate lorem-ipsum datasets, upload them to swift and record them in the SDA database."""

import argparse
import os
import time
import uuid
from dataclasses import dataclass, field
from io import BytesIO

import psycopg2
import psycopg2.extras
from nacl.public import PrivateKey

from generate_data import (
    SwiftEndpoint,
    VaultAccess,
    authenticate,
    build_path,
    create_from_lorem,
    encrypt_data,
    register_visa,
    report_total_time,
    send_header_to_vault,
    upload_object,
)

email = os.environ.get("USER_EMAIL")

DATASET_SQL = """INSERT INTO sda.datasets(stable_id)
                 VALUES(%s) RETURNING id;"""

FILE_SQL = """INSERT INTO sda.files (stable_id, submission_user, submission_file_path, archive_file_path, archive_file_size)
              VALUES (%s, %s, %s, %s, %s) RETURNING id;"""

FILE_DATASET_SQL = """INSERT INTO sda.file_dataset (file_id, dataset_id) \
                      VALUES (%s, %s)"""

FILE_EVENT_SQL = """INSERT INTO sda.file_event_log (file_id, event) \
                    VALUES (%s, 'ready')"""


@dataclass
class UploadContext:
    """Shared connection details and credentials needed to upload one run of datasets."""

    swift: SwiftEndpoint
    vault: VaultAccess
    project: str
    container: str
    target: str
    timeout: int
    seckey: bytes = field(default_factory=lambda: bytes(PrivateKey.generate()))


@dataclass
class FileRecord:
    """Metadata about one uploaded file, to be recorded in the SDA database."""

    archive_path: str
    submission_path: str
    file_no: int
    size: int


def _create_dataset(conn, dataset_name: str):
    """Insert a dataset row and return its generated id, or None on failure."""
    with conn.cursor() as cur:
        try:
            cur.execute(DATASET_SQL, (dataset_name,))
            rows = cur.fetchone()
            return rows[0] if rows else None
        except psycopg2.DatabaseError as error:
            print(f"Failed to add dataset {dataset_name} to database: {error}")
            return None


def _insert_file(conn, dataset_id, record: FileRecord) -> bool:
    """Insert file + file_dataset + file_event rows for one uploaded file."""
    with conn.cursor() as cur:
        try:
            cur.execute(
                FILE_SQL,
                (
                    f"file_stable_id_{record.file_no}",
                    email,
                    record.submission_path,
                    record.archive_path,
                    record.size,
                ),
            )
            rows = cur.fetchone()
            file_id = rows[0] if rows else None

            cur.execute(FILE_DATASET_SQL, (file_id, dataset_id))
            cur.execute(FILE_EVENT_SQL, (file_id,))
            return True
        except psycopg2.DatabaseError as error:
            print(f"Failed to add file {record.archive_path} to database: {error}")
            return False


def _upload_dataset_files(ctx: UploadContext, conn, dataset_id, ds: dict, file_no: int) -> int:
    """Encrypt, upload and record every object belonging to one dataset. Returns the next file number."""
    for file in ds["objects"]:
        object_file = BytesIO(file["content"].encode("utf-8"))
        archive_path = f"{uuid.uuid4()}"
        submission_path = email + "/" + file["name"]

        b64_encoded_header, encrypted_data = encrypt_data(
            object_file, ctx.seckey, ctx.vault.project_pubkey, True
        )
        object_path = f"{ctx.project}/{ctx.container}/{archive_path}"
        send_header_to_vault(ctx.vault, object_path, ctx.timeout, b64_encoded_header)
        upload_object(ctx.swift, ctx.container, build_path(archive_path), encrypted_data, archive_path)

        record = FileRecord(archive_path, submission_path, file_no, len(encrypted_data))
        if not _insert_file(conn, dataset_id, record):
            break
        file_no += 1

    return file_no


def run(ctx: UploadContext, db_string: str, n_datasets_for_user: int, n_datasets: int, n_objects: int):
    """Generate lorem-ipsum datasets, upload them to swift and record them in the SDA database."""
    data = create_from_lorem(n_datasets, n_objects, True)

    try:
        conn = psycopg2.connect(db_string)
        conn.autocommit = True
        psycopg2.extras.register_uuid()
    except psycopg2.Error as error:
        print(f"Unable to connect to the database {db_string}: {error}")
        return

    file_no = 1
    for ds in data:
        dataset_name = ds["name"]
        if ctx.target == "sdapply":
            dataset_name = "EGA" + dataset_name
        else:
            dataset_name = "https://bp-" + dataset_name + ".org"

        if n_datasets_for_user > 0:
            n_datasets_for_user -= 1
            register_visa(ctx.target, dataset_name, ctx.timeout)
        else:
            dataset_name += "-invalid"  # The user should not be able to access these

        dataset_id = _create_dataset(conn, dataset_name)
        print(f"Dataset:\t {dataset_name}")

        file_no = _upload_dataset_files(ctx, conn, dataset_id, ds, file_no)

    conn.close()


def main():
    """Parse CLI arguments and upload the generated datasets."""
    parser = argparse.ArgumentParser(
        description="Generate objects in swift object storage under a single bucket and fill in a sda DB accordingly.",
    )
    parser.add_argument(
        "--project",
        default="sdapply-project",
        help="Keystone project. Defaults to (sdapply-project)",
    )
    parser.add_argument(
        "--target",
        default="sdapply",
        help="Value that mockauth can use to recognise issuer. Defaults to (sdapply)",
    )
    parser.add_argument("--container", help="The container to which all the data is uploaded")
    parser.add_argument("--db-string", help="Connection string to the sda DB")
    parser.add_argument(
        "--datasets", type=int, default=3, help="Number of datasets to create that user can access"
    )
    parser.add_argument(
        "--all-datasets",
        type=int,
        default=6,
        help="Number of datasets to create that will exist in the database",
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

    swift, vault = authenticate(args.project, args.timeout)
    print("Uploading...")

    ctx = UploadContext(
        swift=swift,
        vault=vault,
        project=args.project,
        container=args.container,
        target=args.target,
        timeout=args.timeout,
    )
    run(ctx, args.db_string, args.datasets, args.all_datasets, args.files)

    report_total_time(total_start)


if __name__ == "__main__":
    main()
