FROM python:3.14-slim-bookworm

COPY ./dev-tools/containers/data-upload /scripts

WORKDIR /scripts

RUN python3 -m venv venv

ENV PATH="/scripts/venv/bin:$PATH"

RUN pip install --upgrade pip
RUN apt-get update \
    && apt-get -y install libpq-dev gcc \
    && pip install psycopg2

RUN pip install -r /scripts/requirements.txt

CMD ["./cmd.sh"]
