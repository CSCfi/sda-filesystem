ARG ARTIFACTORY_SERVER=
FROM ${ARTIFACTORY_SERVER}python:3.15.0-slim-trixie@sha256:2da676046630d7bf14c6764ae1143dc7fa0377a53f94bf5a642f0b214aca5bce

COPY ./dev-tools/containers/data-upload /scripts

WORKDIR /scripts

RUN python3 -m venv venv

ENV PATH="/scripts/venv/bin:$PATH"

RUN pip install --upgrade pip
RUN apt-get update \
    && apt-get -y install libpq-dev gcc

RUN pip install -r /scripts/requirements.txt

CMD ["./cmd.sh"]
