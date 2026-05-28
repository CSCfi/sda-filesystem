ARG ARTIFACTORY_SERVER=
FROM ${ARTIFACTORY_SERVER}python:3.14.4-slim-trixie

COPY ./dev-tools/containers/mockauth /scripts

WORKDIR /scripts

RUN python3 -m venv venv

ENV PATH="/scripts/venv/bin:$PATH"

RUN pip install --upgrade pip
RUN pip install -r /scripts/requirements.txt

ENTRYPOINT ["./mock_auth.py"]
