FROM golang:1.24.2 AS builder

RUN apt-get update && apt-get install -y \
    pkg-config \
    libfuse3-dev \
    libc6-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /go/src/gateway

COPY . .
RUN go mod tidy
RUN go build -o /go/bin/gateway cmd/fuse/main.go


FROM ubuntu:24.04

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
