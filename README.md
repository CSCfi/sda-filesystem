# SDA-Filesystem

[![Linting go code](https://github.com/CSCfi/sda-filesystem/actions/workflows/linting.yml/badge.svg)](https://github.com/CSCfi/sda-filesystem/actions/workflows/linting.yml)
[![Unit Tests](https://github.com/CSCfi/sda-filesystem/actions/workflows/unittest.yml/badge.svg)](https://github.com/CSCfi/sda-filesystem/actions/workflows/unittest.yml)
[![Coverage Status](https://coveralls.io/repos/github/CSCfi/sda-filesystem/badge.svg?branch=faeture/add-unit-tests-action)](https://coveralls.io/github/CSCfi/sda-filesystem?branch=faeture/add-unit-tests-action)

SDA-Filesystem makes use of the:

- [SD-Connect Proxy API](docs/API.md) 
- [SD-Submit Download API](https://github.com/neicnordic/sda-download/blob/main/docs/API.md) 

It build a FUSE (Filesystem in Userspace) layer. Software currently supports Linux, macOS and Windows for:
- [Graphical User Interface](#graphical-user-interface)
- [Command Line Interface](#command-line-interface)

Binaries are built on each release for all supported Operating Systems.

### Requirements

Go version 1.16

Set environment variables, before running the application:
- SD-Connect: `FS_SD_CONNECT_METADATA_API` - metadata API for building the directories, files and their associated information (size, name etc.), `FS_SD_CONNECT_DATA_API` - for downloading files;
- SD-Submit: `FS_SD_SUBMIT_API` for we can provide a list of API separated by comma and `SDS_ACCESS_TOKEN` - which should contain a JWT for authenticating to the SD-Submit download API;
- `FS_CERTS` is required by SD-Connect and SD-Submit if self signed certificates are used on the APIs.

For test environment use:

```
export FS_SD_CONNECT_METADATA_API=https://connect-metadata-api-test.sd.csc.fi           
export FS_SD_CONNECT_DATA_API=https://connect-data-api-test.sd.csc.fi
export FS_SD_SUBMIT_API=https://test.sd.csc.fi/

# Connection requires a certificate only if using untrusted (e.g. self-signed) certificates
# if signed by a trusted CA, this is not needed
# FS_CERTS should be the file that contains the necessary certificates
export FS_CERTS=cert.pem
```

## Graphical User Interface

###  Dependencies

`cgofuse` and its [dependencies on different operating systems](https://github.com/billziss-gh/cgofuse#how-to-build).

Install [Qt for Go](https://github.com/therecipe/qt). Regardless of the operating system, there are multiple ways of installing this package. Required that `GO111MODULE=on`.

### Setup

On linux install required packages and vendor dependencies
```
./dev_utils/setup-linux.sh
```

Note: for some vendor modules there might be warnings such as:
```
INFO[0427] installing full qt/bluetooth                 
go install: no install location for directory /home/<user>/sda-filesystem/vendor/github.com/therecipe/qt/bluetooth outside GOPATH
	For more details see: 'go help gopath'
```
These are ok, and are caused as of go 1.14+ 

### Build and Run

```
qtdeploy build desktop cmd/qt/main.go
./cmd/qt/deploy/darwin/qt_project.app/Contents/MacOS/qt_project  // Path slightly different for other OSs
```

If you wish to create a build for linux regardless of the OS you are currently on, you may use the provided dockerfile. Remember to name the image `therecipe/qt:linux`

### Deploy

To deploy binary to Virtual Machine (VM):
```
qtdeploy build desktop cmd/qt/main.go
tar -czf deploy.tar.gz -C cmd/qt/deploy linux
```

Copy the archive of the deployment environment.

## Command Line Interface

The CLI binary will require a username and password for accessing the SD-Connect Proxy API.

### Build and Run
```
go build -o ./go-fuse ./cmd/cli/main.go
```
Test install.
```
./go-fuse -help                        
Usage of ./go-fuse:
  -alsologtostderr
    	log to standard error as well as files
  -enable string
    	Choose which repositories you wish include in the filesystem. Possible values: {SD-Connect,SD-Submit,all} (default "all")
  -http_timeout int
    	Number of seconds to wait before timing out an HTTP request (default 20)
  -log_backtrace_at value
    	when logging hits line file:N, emit a stack trace
  -log_dir string
    	If non-empty, write log files in this directory
  -loglevel string
    	Logging level. Possible values: {debug,info,warning,error} (default "info")
  -logtostderr
    	log to standard error instead of files
  -mount string
    	Path to filesystem mount point
  -stderrthreshold value
    	logs at or above this threshold go to stderr
  -v value
    	log level for V logs
  -vmodule value
    	comma-separated list of pattern=N settings for file-filtered logging

```
Example run: `./go-fuse -mount=$HOME/ExampleMount` will create in the directory `$HOME/ExampleMount` the FUSE layer for both SD-Connect and SD-Submit.

## License

SDA-Filesystem is released under `MIT`, see [LICENSE](LICENSE).

[Qt binding package for Go](https://github.com/therecipe/qt) released under [LGPLv3](https://opensource.org/licenses/LGPL-3.0)

[CgoFuse](https://github.com/billziss-gh/cgofuse) is released under [MIT](https://github.com/billziss-gh/cgofuse/blob/master/LICENSE.txt)

Qt itself is licensed and available under multiple [licenses](https://www.qt.io/licensing).
