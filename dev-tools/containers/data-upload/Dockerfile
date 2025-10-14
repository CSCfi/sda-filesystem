FROM python:3.14-slim

COPY ./dev-tools/scripts /scripts

WORKDIR /scripts

RUN python3 -m venv venv

ENV PATH="/scripts/venv/bin:$PATH"

RUN pip install --upgrade pip
RUN pip install -r /scripts/requirements.txt

CMD ["python", "generate_data.py"]
