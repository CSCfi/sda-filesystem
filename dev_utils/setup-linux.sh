#!/bin/bash


GO_VERSION='go1.16.6'
OS=$(uname -nsm)

# Check terraform exists and it has a specific version
if ! [ -x "$(command -v go)" ]; then
    echo 'terraform is not installed, and it is a requirement.' >&2
    exit 1
elif ! [ "$(go version | awk 'NR==1{print $3}')" == "${GO_VERSION}" ]; then
    echo "The terraform version on this ${OS} machine is different than the one required: ${GO_VERSION}" >&2
    echo "continuing for now ..."
fi


export GO111MODULE=off; go get -v github.com/therecipe/qt/cmd/... 

export GO111MODULE=on; go install -v -tags=no_env github.com/therecipe/qt/cmd/... && \
    go mod vendor && rm -rf vendor/github.com/therecipe/env_linux_amd64_513 && \
    git clone https://github.com/therecipe/env_linux_amd64_513.git vendor/github.com/therecipe/env_linux_amd64_513 && \
    "$(go env GOPATH)"/bin/qtsetup
