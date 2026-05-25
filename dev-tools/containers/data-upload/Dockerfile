ARG ARTIFACTORY_SERVER=
FROM ${ARTIFACTORY_SERVER}python:3.14.4-slim-trixie

COPY ./dev-tools/containers/data-upload /scripts

WORKDIR /scripts

RUN python3 -m venv venv

ENV PATH="/scripts/venv/bin:$PATH"

RUN pip install --upgrade pip
RUN apt-get update \
    && apt-get -y install libpq-dev gcc

RUN pip install -r /scripts/requirements.txt

CMD ["./cmd.sh"]
