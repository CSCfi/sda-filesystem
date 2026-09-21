ARG ARTIFACTORY_SERVER_GHCR=ghcr.io/
FROM ${ARTIFACTORY_SERVER_GHCR}astral-sh/uv:0.12.5-python3.14-alpine3.23@sha256:56b2e7ad659cd4b8d3abb5e67556a1e7ac53adf3a13eb8c292696df9c1e70b67
ARG UV_DEFAULT_INDEX
ENV UV_DEFAULT_INDEX=$UV_DEFAULT_INDEX

COPY ./dev-tools/containers/mockauth /scripts

WORKDIR /scripts

RUN --mount=type=secret,id=uv_password \
    --mount=type=secret,id=uv_username \
    export UV_INDEX_ARTIFACTORY_PASSWORD="$(cat /run/secrets/uv_password)" \
    && export UV_INDEX_ARTIFACTORY_USERNAME="$(cat /run/secrets/uv_username)" \
    && uv pip install --system -r /scripts/requirements.txt

ENTRYPOINT ["./mock_auth.py"]
