#!/bin/bash


GO_VERSION='go1.16.6'
OS=$(uname -nsm)

# Check golang exists and it has a specific version
if ! [ -x "$(command -v go)" ]; then
    echo 'golang is not installed, and it is a requirement.' >&2
    exit 1
elif ! [ "$(go version | awk 'NR==1{print $3}')" == "${GO_VERSION}" ]; then
    echo "The golang version on this ${OS} machine is different than the one required: ${GO_VERSION}" >&2
    echo "continuing for now ..."
fi


# download as general package
# otherwise it will not work
export GO111MODULE=off

go get github.com/therecipe/qt/cmd/... 

export GO111MODULE=on

# dependencies not easily installed
go get github.com/therecipe/qt/internal/binding/files/docs/5.13.0
go get github.com/therecipe/qt/internal/cmd/moc@v0.0.0-20200904063919-c0c124a5770d

go install -v -tags=no_env github.com/therecipe/qt/cmd/...

# rebuild vendor
go mod vendor
[ -d "vendor/github.com/therecipe/env_linux_amd64_513" ] && rm -rf vendor/github.com/therecipe/env_linux_amd64_513

git clone https://github.com/therecipe/env_linux_amd64_513.git vendor/github.com/therecipe/env_linux_amd64_513

# actually do the setup so that a binary can be built
"$(go env GOPATH)"/bin/qtsetup

echo "Now qtdeploy can be used."
echo "If the command is not found refresh your bash/zsh shell, or start a new terminal."
echo "===> Done. "
