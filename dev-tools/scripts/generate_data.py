#!/usr/bin/env python3

import argparse
import getpass
import json
import os
import pathlib
import random
import time
import typing
from base64 import b64decode, b64encode
from io import BytesIO
from typing import Tuple
from urllib.parse import quote

import lorem
import requests
from crypt4gh.lib import encrypt, header
from nacl.public import PrivateKey


keystone_base_url = os.environ.get("KEYSTONE_BASE_URL", "http://127.0.0.1:5001")
proxy_base_url = os.environ.get("PROXY_URL", "http://127.0.0.1:80/")

fixed_key = b64decode(os.environ.get("FIXED_PUBLIC_KEY"))


def build_path(*parts: str) -> str:
    return "/" + "/".join(quote(part, safe="") for part in parts)


def wait_for_port(url, timeout=5.0):
    """Wait for a port to become available

    This is used in case the script was executed before the server is available,
    making its usage more flexible.
    """
    start_time = time.perf_counter()

    while True:
        try:
            r = requests.head(url)
            break
        except requests.exceptions.RequestException as ex:
            print(ex)
            current = time.perf_counter() - start_time
            print(f"Waiting for {url} {current:.2f}s elapsed", end="\r")
            if time.perf_counter() - start_time >= timeout:
                raise TimeoutError(
                    "Waited too long for {} to start accepting "
                    "connections.".format(url)
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


def get_aai_token() -> str:
    """Get a JWT token from aai to be used when making requests to krakend"""
    sds_access_token = os.environ.get("SDS_ACCESS_TOKEN")
    auth_response = requests.get(
        f"{proxy_base_url}/profile",
        headers={"Authorization": f"Bearer {sds_access_token}"},
        timeout=5.0,
    )
    if auth_response.status_code >= 400:
        print("Failed to get aai token: " + auth_response.text)
        return ""

    return auth_response.json()["access_token"]


def get_public_key(aai_token: str) -> bytes:
    key_response = requests.get(
        f"{proxy_base_url}/desktop/project-key",
        headers={"Authorization": f"Bearer {aai_token}"},
        timeout=5.0,
    )
    if key_response.status_code >= 400:
        raise RuntimeError(
            f"Failed to get the project pub key: {key_response.status_code} {key_response.text}"
        )

    key_json = key_response.json()

    return b64decode(key_json["public_key_c4gh_64"])


def create_from_lorem(n_containers: int, n_objects: int) -> list:
    data = []
    container_names = set()
    while len(container_names) < n_containers:
        cont_name = lorem.get_sentence(comma=(0, 0), word_range=(1, 3))[:-1]
        container_names.add(cont_name.replace(" ", "-").lower())

    for cont_name in container_names:
        objects = []
        object_names = set()
        while len(object_names) < n_objects:
            obj_name = lorem.get_sentence(comma=(0, 0), word_range=(1, 3)) + "txt.c4gh"
            obj_name = obj_name.replace(" ", "/")
            object_names.add(obj_name)
        for obj_name in object_names:
            objects.append(
                {
                    "name": obj_name,
                    "content": lorem.get_paragraph(),
                    "split_header": random.choice([True, False])
                }
            )
        data.append(
            {
                "name": cont_name,
                "objects": objects,
            }
        )

    return data


def encrypt_data(object_file: BytesIO, secret_key: bytes, public_key: bytes, split: bool) -> Tuple[str, bytes]:
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
    encrypted_data = encrypted_data[len(header_bytes):]
    b64_encoded_header = b64encode(header_bytes).decode()

    return b64_encoded_header, encrypted_data


def run(
    swift_url: str,
    swift_token: str,
    aai_token: str,
    project_pubkey: bytes,
    json_path=None,
    n_containers=5,
    n_objects=3,
    timeout=60,
):
    data: list
    if json_path:
        with open(json_path, "r") as fp:
            data = json.load(fp)
        print(f"Populating data from {json_path}")
        n_containers = len(data)
        n_objects = len(data[0]["objects"])
    else:
        data = create_from_lorem(n_containers, n_objects)

    seckey = bytes(PrivateKey.generate())

    for cont in data:
        # create container
        container_name = cont["name"]
        with requests.put(
            f'{swift_url}/{container_name}',
            headers={"X-Auth-Token": swift_token},
            allow_redirects=True,
            timeout=timeout,
        ) as response:
            if response.status_code in {201, 202}:
                print(f'Container:\t{response.status_code} {container_name}')
            else:
                print(f'Container:\tERROR {response.status_code} {container_name}')
                response.raise_for_status()

        for file in cont["objects"]:
            object_file = BytesIO(file["content"].encode('utf-8'))

            b64_encoded_header, encrypted_data = encrypt_data(object_file, seckey, project_pubkey, file["split_header"])
            if b64_encoded_header != "":
                with requests.post(
                    f'{proxy_base_url}/desktop/file-headers/{container_name}',
                    headers={"Authorization": f"Bearer {aai_token}"},
                    json={"header": b64_encoded_header},
                    params={"object": file["name"]},
                    timeout=timeout,
                ) as response:
                    if response.status_code >= 400:
                        print("Failed to send header to vault: " + response.text)

            print(f"header: {b64_encoded_header != ""}")
            url = (
                swift_url
                + f'/{container_name}/'
                + "/".join(quote(part, safe="") for part in file["name"].split("/"))
            )
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



if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="Generate containers and objects in swift object storage, and wait for metadata to be updated.",
        epilog="By default, runs with a Swift TempAuth account. use '--keystone' to authenticate agains keystone.",
    )
    parser.add_argument(
        "--containers", type=int, default=10, help="Number of containers to create"
    )
    parser.add_argument(
        "--objects",
        type=int,
        default=15,
        help="Number of objects per container to create",
    )

    parser.add_argument(
        "--timeout",
        type=int,
        default=60,
        help="Maximum time to wait before data is generated and metadata is updated, for each run",
    )

    parser.add_argument(
        "--from-json",
        type=pathlib.Path,
        default=None,
        help="Generate data from a pre-existing json structure",
    )

    args = parser.parse_args()
    total_start = time.perf_counter()

    username = os.environ.get("CSC_USERNAME", "swift")
    password = os.environ.get("CSC_PASSWORD", "veryfast")
    project = os.environ.get("CSC_PROJECT", "service")

    wait_for_port(keystone_base_url, timeout=args.timeout)
    swift_base_url, keystone_token = get_keystone_token(
        username, password, project
    )

    aai_jwt_token = get_aai_token()

    wait_for_port(proxy_base_url, timeout=args.timeout)
    public_key = get_public_key(aai_jwt_token)

    print("Uploading...")

    run(
        swift_base_url,
        keystone_token,
        aai_jwt_token,
        public_key,
        args.from_json,
        args.containers,
        args.objects,
        args.timeout,
    )

    total_end = time.perf_counter() - total_start
    print()
    print(f"Completed in {total_end:.2f} seconds")
