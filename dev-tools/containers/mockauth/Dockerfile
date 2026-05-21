ARG ARTIFACTORY_SERVER=
FROM ${ARTIFACTORY_SERVER}python:3.14.4-slim-trixie

COPY --chmod=755 ./dev-tools/containers/mockauth/mock_auth.py /usr/local/bin

RUN pip install --upgrade pip
RUN pip install authlib aiohttp

ENTRYPOINT ["/usr/local/bin/mock_auth.py"]
