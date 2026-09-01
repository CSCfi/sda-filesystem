ARG ARTIFACTORY_SERVER=
FROM ${ARTIFACTORY_SERVER}ubuntu:24.04@sha256:33ceb71981b602c1a7443a53469e4dba065f7503eab3078a2d7a57a2ab987517 AS builder

RUN apt-get update && apt-get install -y \
    build-essential \
    pkg-config \
    libfuse3-dev \
    libc6-dev \
    wget \
    && rm -rf /var/lib/apt/lists/*

RUN wget https://go.dev/dl/go1.26.6.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.26.6.linux-amd64.tar.gz \
    && rm go1.26.6.linux-amd64.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /go/src/gateway

COPY . .
RUN go mod tidy
RUN go build -o /go/bin/gateway ./cmd/cli


FROM ${ARTIFACTORY_SERVER}ubuntu:24.04@sha256:33ceb71981b602c1a7443a53469e4dba065f7503eab3078a2d7a57a2ab987517

RUN apt-get update && apt-get install -y \
    build-essential \
    libfuse3-3 \
    libc6 \
    psmisc \
    fuse3 \
    bash \
    curl \
    nano \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /go/bin/gateway /usr/local/bin/gateway

WORKDIR /usr/local/bin
