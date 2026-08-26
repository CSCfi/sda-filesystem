#!/usr/bin/env python3
"""Generate lorem-ipsum containers and objects and upload them to swift object storage."""

import argparse
import os
import random
import time
import typing
from base64 import b64decode, b64encode
from dataclasses import dataclass, field
from io import BytesIO
from typing import Tuple
from urllib.parse import quote

import lorem
import requests
from crypt4gh.lib import encrypt, header
from nacl.public import PrivateKey

keystone_base_url = os.environ.get("KEYSTONE_BASE_URL", "http://127.0.0.1:5001")
vault_base_url = os.environ.get("VAULT_ADDR", "http://127.0.0.1:8200")
aai_base_url = os.environ.get("AAI_BASE_URL")

fixed_key = b64decode(os.environ.get("FIXED_PUBLIC_KEY"))


def build_path(*parts: str) -> str:
    """Join path components into a single URL-quoted, slash-separated path."""
    return "/" + "/".join(quote(part, safe="") for part in parts)


def wait_for_port(url, timeout=5.0):
    """Wait for a port to become available

    This is used in case the script was executed before the server is available,
    making its usage more flexible.
    """
    start_time = time.perf_counter()

    while True:
        try:
            _ = requests.head(url, timeout=timeout)
            break
        except requests.exceptions.RequestException as ex:
            print(ex)
            current = time.perf_counter() - start_time
            print(f"Waiting for {url} {current:.2f}s elapsed", end="\r")
            if time.perf_counter() - start_time >= timeout:
                raise TimeoutError(
                    f"Waited too long for {url} to start accepting connections."
                ) from ex
            time.sleep(1)
    end = time.perf_counter() - start_time
    if end > 1:
        print(f"Waited {end} for {url}", end="\r")
        print()


def get_keystone_token(
    username: str,
    password: str,
    project: str,
) -> typing.Tuple[str, str]:
    """Create a keystone scoped for the specified project

    :return tuple with access URL and access token
    """
    auth_data = {
        "auth": {
            "identity": {
                "methods": ["password"],
                "password": {
                    "user": {
                        "domain": {"id": "default"},
                        "name": username,
                        "password": password,
                    }
                },
            },
            "scope": {"project": {"name": project, "domain": {"id": "default"}}},
        }
    }
    auth = requests.post(
        f"{keystone_base_url}/v3/auth/tokens",
        json=auth_data,
        allow_redirects=True,
        timeout=5.0,
    )

    result = auth.json()
    if "error" in result:
        error_message = result["error"]["message"]
        raise RuntimeError(f"Keystone auth failed: {error_message}")

    # Takes the first endpoint that matches the condition
    swift_endpoints = next(
        (endpoint for endpoint in result["token"]["catalog"] if endpoint["name"] == "swift")
    )
    swift_url = swift_endpoints["endpoints"][0]["url"]
    if "0.0.0.0" in swift_url.split(":")[1]:
        swift_url = swift_url.split(":")[2]
        swift_host = ":".join(keystone_base_url.split(":")[:2])
        swift_url = f"{swift_host}:{swift_url}"
    token = auth.headers["X-Subject-Token"]
    return swift_url, token


def get_vault_token() -> str:
    """Get Vault access token for uploading headers of encrypted files"""
    auth_data = {
        "role_id": os.environ.get("VAULT_ROLE"),
        "secret_id": os.environ.get("VAULT_SECRET"),
    }
    auth = requests.post(
        f"{vault_base_url}/v1/auth/approle/login",
        json=auth_data,
        allow_redirects=True,
        timeout=5.0,
    )

    if auth.status_code >= 400:
        raise RuntimeError(f"Failed to get vault token: {auth.status_code} {auth.text}")

    result = auth.json()

    return result["auth"]["client_token"]


def get_public_key(vault_token: str, project: str) -> bytes:
    """Get project public key from Vault"""
    key_response = requests.get(
        f"{vault_base_url}/v1/c4ghtransit/keys/{project}",
        headers={"X-Vault-Token": f"{vault_token}"},
        timeout=5.0,
    )
    if key_response.status_code >= 400:
        raise RuntimeError(
            f"Failed to get the project pub key: {key_response.status_code} {key_response.text}"
        )

    resp_json = key_response.json()
    latest_version = resp_json["data"]["latest_version"]
    keys = resp_json["data"]["keys"]

    return b64decode(keys[str(latest_version)]["public_key_c4gh_64"])


def create_from_lorem(n_containers: int, n_objects: int, force_split: bool) -> list:
    """Generate lorem-ipsum containers, each with a set of lorem-ipsum objects."""
    data = []
    container_names = set()
    while len(container_names) < n_containers:
        cont_name = lorem.get_sentence(comma=(0, 0), word_range=(1, 3))[:-1]
        container_names.add(cont_name.replace(" ", "-").lower())

    for cont_name in container_names:
        objects = []
        object_names = set()
        while len(object_names) < n_objects:
            obj_name = lorem.get_sentence(comma=(0, 0), word_range=(1, 3))
            obj_name = obj_name.replace(" ", "/")
            object_names.add(obj_name)
        for obj_name in object_names:
            split = True if force_split else random.choice([True, False])
            if not split:
                obj_name += "old."
            obj_name += "txt.c4gh"

            objects.append(
                {"name": obj_name, "content": lorem.get_paragraph(), "split_header": split}
            )
        data.append(
            {
                "name": cont_name,
                "objects": objects,
            }
        )

    return data


def encrypt_data(
    object_file: BytesIO, secret_key: bytes, public_key: bytes, split: bool
) -> Tuple[str, bytes]:
    """Encrypt an in memory file and return the encrypted header and the encrypted data."""
    encrypted_file = BytesIO()
    if not split:
        public_key = fixed_key

    encrypt([(0, secret_key, public_key)], object_file, encrypted_file)
    object_file.seek(0)
    encrypted_file.seek(0)
    encrypted_data = encrypted_file.getvalue()
    if not split:
        return "", encrypted_data

    header_bytes = header.serialize(header.parse(encrypted_file))
    encrypted_data = encrypted_data[len(header_bytes) :]
    b64_encoded_header = b64encode(header_bytes).decode()

    return b64_encoded_header, encrypted_data


@dataclass
class SwiftEndpoint:
    """Swift object storage URL and access token."""

    url: str
    token: str


@dataclass
class VaultAccess:
    """Vault access token and the project's public key for header encryption."""

    token: str
    project_pubkey: bytes


def authenticate(project: str, timeout: float) -> Tuple[SwiftEndpoint, VaultAccess]:
    """Wait for keystone and vault to come up and gather swift/vault credentials for `project`."""
    username = os.environ.get("CSC_USERNAME", "swift")
    password = os.environ.get("CSC_PASSWORD", "veryfast")

    wait_for_port(keystone_base_url, timeout=timeout)
    swift_url, swift_token = get_keystone_token(username, password, project)

    wait_for_port(vault_base_url, timeout=timeout)
    vault_token = get_vault_token()
    project_pubkey = get_public_key(vault_token, project)

    return SwiftEndpoint(url=swift_url, token=swift_token), VaultAccess(
        token=vault_token, project_pubkey=project_pubkey
    )


def report_total_time(start: float) -> None:
    """Print how long the whole run took."""
    total_end = time.perf_counter() - start
    print()
    print(f"Completed in {total_end:.2f} seconds")


def register_visa(target: str, name: str, timeout: float) -> None:
    """Register a dataset/container visa with mockauth so the mock user can access it."""
    with requests.post(
        f"{aai_base_url}/api/jwk/{target}/" + quote(name, safe=""),
        timeout=timeout,
    ) as response:
        if response.status_code >= 400:
            print("Failed to send dataset to mockauth: " + response.text)


def send_header_to_vault(vault: VaultAccess, object_path: str, timeout: float, b64_encoded_header: str) -> None:
    """Send an encrypted object's Crypt4GH header to Vault.

    `object_path` is the "project/container/object_name" the header is stored under.
    """
    with requests.post(
        f"{vault_base_url}/v1/c4ghtransit/files/{object_path}",
        headers={"X-Vault-Token": f"{vault.token}"},
        json={"header": b64_encoded_header},
        timeout=timeout,
    ) as response:
        if response.status_code >= 400:
            print("Failed to send header to vault: " + response.text)


def upload_object(
    swift: SwiftEndpoint, container: str, quoted_path: str, encrypted_data: bytes, display_name: str
) -> None:
    """PUT encrypted object data into a swift container."""
    url = swift.url + f"/{container}" + quoted_path
    response = requests.put(
        url,
        headers={"X-Auth-Token": swift.token, "Content-Type": "text/plain"},
        data=encrypted_data,
        allow_redirects=True,
        timeout=5.0,
    )
    if response.status_code in {201, 202}:
        print(f"Object:\t\t {response.status_code} {display_name}")
    else:
        print(f"Object:\t\tError {response.status_code} {display_name}")
        response.raise_for_status()


@dataclass
class RunContext:
    """Shared connection details and credentials needed to upload one run of containers."""

    swift: SwiftEndpoint
    vault: VaultAccess
    project: str
    timeout: int
    seckey: bytes = field(default_factory=lambda: bytes(PrivateKey.generate()))


def _create_container(ctx: RunContext, container_name: str) -> None:
    with requests.put(
        f"{ctx.swift.url}/{container_name}",
        headers={"X-Auth-Token": ctx.swift.token},
        allow_redirects=True,
        timeout=ctx.timeout,
    ) as response:
        if response.status_code in {201, 202}:
            print(f"Container:\t{response.status_code} {container_name}")
        else:
            print(f"Container:\tERROR {response.status_code} {container_name}")
            response.raise_for_status()


def _upload_container(ctx: RunContext, cont: dict) -> None:
    """Register, create and populate a single container with its generated objects."""
    container_name = cont["name"]

    if ctx.project != "service":
        register_visa("sdapply", container_name, ctx.timeout)
        container_name = ctx.project[0:2] + "-" + container_name

    _create_container(ctx, container_name)

    for file in cont["objects"]:
        object_file = BytesIO(file["content"].encode("utf-8"))

        b64_encoded_header, encrypted_data = encrypt_data(
            object_file, ctx.seckey, ctx.vault.project_pubkey, file["split_header"]
        )
        if b64_encoded_header != "":
            object_path = f"{ctx.project}/{container_name}/{file['name']}"
            send_header_to_vault(ctx.vault, object_path, ctx.timeout, b64_encoded_header)

        quoted_path = build_path(*file["name"].split("/"))
        upload_object(ctx.swift, container_name, quoted_path, encrypted_data, file["name"])


def run(ctx: RunContext, n_containers: int, n_objects: int, force_split_header: bool):
    """Generate lorem-ipsum containers and objects and upload them to swift object storage."""
    data = create_from_lorem(n_containers, n_objects, force_split_header)

    for cont in data:
        _upload_container(ctx, cont)


def main():
    """Parse CLI arguments and upload the generated containers and objects."""
    parser = argparse.ArgumentParser(
        description="Generate containers and objects in swift object storage, and wait for metadata to be updated.",
    )
    parser.add_argument(
        "--project", default="service", help="Keystone project. Defaults to (service)"
    )
    parser.add_argument("--containers", type=int, default=10, help="Number of containers to create")
    parser.add_argument(
        "--objects",
        type=int,
        default=15,
        help="Number of objects per container to create",
    )
    parser.add_argument(
        "--headerless",
        action="store_true",
        help="Force all headers of encrypted objects to be stored separately",
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

    ctx = RunContext(swift=swift, vault=vault, project=args.project, timeout=args.timeout)
    run(ctx, args.containers, args.objects, args.headerless)

    report_total_time(total_start)


if __name__ == "__main__":
    main()
