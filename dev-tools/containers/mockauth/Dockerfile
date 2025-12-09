FROM python:3.14-slim-bookworm

COPY --chmod=755 ./dev-tools/containers/mockauth/mock_auth.py /usr/local/bin

RUN pip install --upgrade pip
RUN pip install authlib aiohttp

ENTRYPOINT ["/usr/local/bin/mock_auth.py"]
