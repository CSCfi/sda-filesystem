#!/bin/bash


GO_VERSION='go1.19.4'
OS=$(uname -nsm)

# Check golang exists and it has a specific version
if ! [ -x "$(command -v go)" ]; then
    echo 'golang is not installed, and it is a requirement.' >&2
    exit 1
elif ! [ "$(go version | awk 'NR==1{print $3}')" == "${GO_VERSION}" ]; then
    echo "The golang version on this ${OS} machine is different than the one required: ${GO_VERSION}" >&2
    echo "continuing for now ..."
fi

go install github.com/wailsapp/wails/v2/cmd/wails@latest
export PATH=$GOPATH/bin:$PATH
wails doctor

echo "If the wails command is not found, refresh your bash/zsh shell, or start a new terminal."
echo "Check also that $GOPATH/bin is in PATH."
echo "===> Done. "
