#!/usr/bin/env python3
"""Mock SDS-AAI"""

import json
import logging
from os import getenv, environ
from time import time
from typing import Tuple
from ast import literal_eval as make_tuple

from aiohttp import web, ClientSession
from authlib.jose import RSAKey, jwt

FORMAT = "[%(asctime)s][%(levelname)-8s](L:%(lineno)s) %(funcName)s: %(message)s"
logging.basicConfig(format=FORMAT, datefmt="%Y-%m-%d %H:%M:%S")

LOG = logging.getLogger("server")
LOG.setLevel(getenv("LOG_LEVEL", "INFO"))

header = {
    "alg": "RS256",
    "typ": "at+JWT",
}


def generate_token() -> Tuple:
    """Generate RSA Key pair to be used to sign token."""
    key = RSAKey.generate_key(is_private=True)
    public_jwk = key.as_dict(is_private=False, alg="RS256")
    private_jwk = key.as_dict(is_private=True)

    return (public_jwk, private_jwk)


def get_desktop_token() -> str:
    iat = int(time())
    ttl = 36000
    exp = iat + ttl
    access_token = {
        "sub": "desktop",
        "iss": mock_auth_url_docker,
        "aud": environ.get("AAI_RESOURCE"),
        "auth_time": iat,
        "exp": exp,
        "iat": iat,
        "client_id": "desktop",
        "scope": "desktop",
    }
    return jwt.encode(header, access_token, jwk_pair[1]).decode("utf-8")


async def get_pouta_token() -> str:
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
            }
        }
    }

    async with ClientSession() as session:
        async with session.post(f"{mock_keystone_url_docker}/v3/auth/tokens", json=auth_data) as resp:
            result = await resp.json()

            if "error" in result:
                error_message = result["error"]["message"]
                raise RuntimeError(f"Keystone auth failed: {error_message}")

            return resp.headers["X-Subject-Token"]


def parse_visa_issuers() -> dict[str, Tuple]:
    issuer_envs = {k: v for k, v in environ.items() if k.endswith('_ISSUER_NAME')}
    jwks: dict[str, Tuple] = {}

    for name_key, issuer_name in issuer_envs.items():
        prefix = name_key[:-len('_ISSUER_NAME')]
        jku_key = f"{prefix}_ISSUER_JKU"
        issuer_jku = environ.get(jku_key)

        LOG.info(f"Prefix: {prefix}")
        LOG.info(f"  ISSUER_NAME: {issuer_name}")
        LOG.info(f"  ISSUER_JKU:  {issuer_jku}")
        LOG.info("")

        service = issuer_name.strip("/").split("/")[-1]
        service_token = generate_token()
        jwks[service] = {
            "issuer": issuer_name,
            "jku": issuer_jku,
            "token": service_token
        }
        LOG.info(f"Added visa with parameters {jwks[service]}")

    return jwks


sds_access_token = environ.get("SDS_ACCESS_TOKEN")
mock_auth_url_docker = environ.get("AAI_BASE_URL")
mock_keystone_url_docker = environ.get("KEYSTONE_BASE_URL")
client_id = environ.get("AAI_CLIENT_ID")
client_secret = environ.get("AAI_CLIENT_SECRET")

project = environ.get("CSC_PROJECT", "service")
is_findata = environ.get("IS_FINDATA", "")
username = environ.get("CSC_USERNAME", "swift")
password = environ.get("CSC_PASSWORD", "veryfast")
email = environ.get("USER_EMAIL", "")

jwk_pair = generate_token()
desktop_token = get_desktop_token()
pouta_token = ""

visa_jwks = parse_visa_issuers()
passport = []

async def token(req: web.Request) -> web.Response:
    """Auth endpoint."""
    post = await req.post()
    match post["grant_type"]:
        case "client_credentials":
            if post["client_id"] != client_id or post["client_secret"] != client_secret:
                return web.Response(status=400, text="invalid credentials")

            iat = int(time())
            ttl = 36000
            exp = iat + ttl
            access_token = {
                "sub": post["client_id"],
                "iss": mock_auth_url_docker,
                "aud": post["resource"],
                "auth_time": iat,
                "exp": exp,
                "iat": iat,
                "client_id": post["client_id"],
                "scope": post["scope"],
            }
            data = {
                "access_token": jwt.encode(header, access_token, jwk_pair[1]).decode("utf-8"),
                "token_type": "Bearer",
                "expires_in": ttl,
                "scope": post["scope"],
            }
            LOG.info(data)

            return web.json_response(data)
        case _:
            return web.Response(status=400, text="invalid grant_type")


async def jwk_response(req: web.Request) -> web.Response:
    """Mock JSON Web Key server."""
    data = {"keys": [jwk_pair[0]]}

    LOG.info(data)

    return web.json_response(data)


async def userinfo(req: web.Request) -> web.Response:
    auth = req.headers["Authorization"]
    if auth != "Bearer " + desktop_token and auth != "Bearer " + sds_access_token:
        return web.Response(status=400, text="invalid token")

    findata_projects = ""
    if is_findata.lower() in ("yes", "true", "t", "1"):
        findata_projects = project

    user_info = {
        "CSCUserName": username,
        "sdDesktopProjects": project,
        "sdDesktopFindataProjects": findata_projects,
        "sdConnectProjects": project,
        "projectPI": project,
        "pouta_access_token": pouta_token,
        "email": email,
        "ga4gh_passport_v1": passport,
    }
    if auth == "Bearer " + sds_access_token:
        user_info["access_token"] = desktop_token

    LOG.info(user_info)

    return web.json_response(user_info)


async def jwk_response_visas(req: web.Request) -> web.Response:
    """Mock JSON Web Key server for visa validation."""
    service = req.match_info["service"]

    if service not in visa_jwks:
        return web.Response(status=404, text="invalid service")

    data = {"keys": [visa_jwks[service]["token"][0]]}

    LOG.info(data)

    return web.json_response(data)


async def post_visa_dataset(req: web.Request) -> web.Response:
    """Endpoint for adding a new dataset visa to passport"""
    service = req.match_info["service"]
    dataset = req.match_info["dataset"]

    iat = int(time())
    ttl = 36000
    exp = iat + ttl
    header = {
        "alg": "RS256",
        "jku": visa_jwks[service]["jku"],
        "typ": "JWT"
    }
    payload = {
        "sub": email,
        "iss": visa_jwks[service]["issuer"],
        "exp": exp,
        "iat": iat,
        "ga4gh_visa_v1": {
            "type": "ControlledAccessGrants",
            "value": dataset,
            "source": visa_jwks[service]["issuer"]
        }
    }
    visa = jwt.encode(header, payload, visa_jwks[service]["token"][1]).decode("utf-8")
    passport.append(visa)

    return web.Response(status=200)


async def init() -> web.Application:
    """Start server."""
    global pouta_token
    app = web.Application()
    app.router.add_post("/idp/profile/oidc/token", token)
    app.router.add_get("/idp/profile/oidc/keyset", jwk_response)
    app.router.add_get("/idp/profile/oidc/userinfo", userinfo)

    app.router.add_get("/api/jwk/{service}", jwk_response_visas)
    app.router.add_post("/api/jwk/{service}/{dataset}", post_visa_dataset)

    pouta_token = await get_pouta_token()

    return app


if __name__ == "__main__":
    web.run_app(init(), port=8000)
