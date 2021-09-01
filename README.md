# SD Connect FUSE
This desktop software converses with the [sd-connect-metadata-api](https://gitlab.ci.csc.fi/sds-dev/sd-connect-metadata-api) and [sd-connect-data-api](https://gitlab.ci.csc.fi/sds-dev/sd-connect-data-api) to download files.

Set environment variables `SD_CONNECT_METADATA_API`, `SD_CONNECT_DATA_API` and `SD_CONNECT_CERTS` before running program.

For test environment use:

```
export SD_CONNECT_METADATA_API=https://connect-metadata-api-test.sd.csc.fi
export SD_CONNECT_DATA_API=https://connect-data-api-test.sd.csc.fi
export SD_CONNECT_CERTS=cert.pem	#SD_CONNECT_CERTS should be the file that contains the necessary certificates
```

## Graphical User Interface

###  Dependencies
Go version 1.16

cgofuse and its [dependencies on different operating systems](https://github.com/billziss-gh/cgofuse#how-to-build).

Install [Qt for Go](https://github.com/therecipe/qt/wiki/Installation). Regardless of the operating system, there are [multiple ways](https://github.com/therecipe/qt/wiki/Available-Tools) of installing this package. Required that `GO111MODULE=on`.


### Setup

Install required packages and vendor dependencies
```
./dev_utils/setup-linux.sh
```

Note: for some vendor modules there might be warnings such as:
```
INFO[0427] installing full qt/bluetooth                 
go install: no install location for directory /home/<user>/sd-connect-fuse-master/vendor/github.com/therecipe/qt/bluetooth outside GOPATH
	For more details see: 'go help gopath'
```
These are ok and are caused as of go 1.14+ 
```
go command now verifies that the main module's vendor/modules.txt file is consistent with its go.mod file.
```


### Run

```
qtdeploy build desktop cmd/qt/main.go
./cmd/qt/deploy/darwin/qt_project.app/Contents/MacOS/qt_project  // Path slightly different for other OS`
```


### Deploy

To deploy binary to Virtual Machine (VM):
```
qtdeploy build desktop cmd/qt/main.go
tar -czf deploy.tar.gz -C cmd/qt/deploy linux
```

Copy the archive of the deployment environment.

## Command Line Interface

```
go build -o ./go-fuse ./cmd/cli/main.go
./go-fuse
```
