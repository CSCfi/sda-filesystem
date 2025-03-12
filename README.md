# SDA-Filesystem / Data Gateway

[![Linting go code](https://github.com/CSCfi/sda-filesystem/actions/workflows/linting.yml/badge.svg)](https://github.com/CSCfi/sda-filesystem/actions/workflows/linting.yml)
[![Unit Tests](https://github.com/CSCfi/sda-filesystem/actions/workflows/unittest.yml/badge.svg)](https://github.com/CSCfi/sda-filesystem/actions/workflows/unittest.yml)
[![Coverage Status](https://coveralls.io/repos/github/CSCfi/sda-filesystem/badge.svg?branch=master)](https://coveralls.io/github/CSCfi/sda-filesystem?branch=master)

**This project has been rebranded as Data Gateway**

Data Gateway builds a FUSE (Filesystem in Userspace) layer and uses Airlock to export files to SD Connect. Software currently supports Linux and macOS for:
- [Graphical User Interface](#graphical-user-interface)
- [Command Line Interface](#command-line-interface)

Binaries are built on each release for all supported Operating Systems.

### Requirements

Go version 1.23

Set these environment variables before running the application:
- `SDS_ACCESS_TOKEN` - a opaque token for authenticating to api gateway

Optional envronment variables:
- `CSC_PASSWORD` - password for SDA-Filesystem and Airlock CLI

## Graphical User Interface

###  Dependencies

Install [Wails](https://wails.io/docs/gettingstarted/installation) and its dependencies.

Install [pnpm](https://pnpm.io/installation)

### Build and Run

Before running/building the repository for the first time, generate the frontend assests by running:
```bash
pnpm install --prefix frontend
pnpm --prefix frontend run build
```

To run in development mode:
```bash
cd cmd/gui
wails dev
```

To build for production:
```bash
cd cmd/gui

# For Linux and macOS
wails build -upx -trimpath -clean -s

# For Windows
wails build -upx -trimpath -clean -s -webview2=embed
```

### Deploy

See [Linux setup](docs/linux-setup.md).

## Command Line Interface

Two command line binaries are released, one for SDA-Filesystem and one for Airlock.

### SDA-Fileystem

The CLI binary will require a username and password for accessing the SD-Connect Proxy API. Username is given as input. Password is either given as input or in an environmental variable.

#### Build and Run
```bash
go build -o ./go-fuse ./cmd/fuse/main.go
```
Test install.
```bash
./go-fuse -help
Usage of ./go-fuse:
  -alsologtostderr
    	log to standard error as well as files
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
    	Path to Data Gateway mount point
  -sdapply
      Connect only to SD Apply
  -stderrthreshold value
    	logs at or above this threshold go to stderr
  -v value
    	log level for V logs
  -vmodule value
    	comma-separated list of pattern=N settings for file-filtered logging

```
Example run: `./go-fuse -mount=$HOME/ExampleMount` will create the FUSE layer in the directory `$HOME/ExampleMount` for both 'SD Connect' and 'SD Apply'.

#### User input

User can update the filesystem by inputting the command `update`. This requires that no files inside the filesystem are being used. Update also clears cache. As a result of this operation, new files may be added and some old ones removed.

The filesystem can be also updated programatically with the `SIGUSR2` signal.

To update filesystem on bash in SD Desktop:
```bash
# Update CLI version
kill -s SIGUSR2 $(pgrep go-fuse)

# Update GUI version
kill -s SIGUSR2 $(pgrep sda-fuse)
```

If the user wants to update particular SD Connect files inside the filesystem, the user can input command `clear <path>`. `<path>` is the path to the file/folder that the user wishes to update. `<path>` must at least contain a bucket, i.e. `SD-Connect/project/bucket` or `SD-Connect/project/bucket/file` would be acceptable paths, but not, e.g., `SD-Connect/project`. If the user gives a path to a folder, all files inside this folder are updated but no files are added or removed. This operation clears the cache for all the neccessary files so that the new content is read from the database and sizes of these files are updated in the filesystem.

### Airlock

The CLI binary will require a username, a bucket and a filename. Password is either given as input or in an environmental variable.

#### Build and Run
```bash
go build -o ./airlock ./cmd/airlock/main.go
```
Test install.
```bash
./airlock -help
Usage of ./airlock:
  -alsologtostderr
    	log to standard error as well as files
  -debug
    	Enable debug prints
  -log_backtrace_at value
    	when logging hits line file:N, emit a stack trace
  -log_dir string
    	If non-empty, write log files in this directory
  -logtostderr
    	log to standard error instead of files
  -quiet
    	Print only errors
  -segment-size int
    	Maximum size of segments in Mb used to upload data. Valid range is 10-4000. (default 4000)
  -stderrthreshold value
    	logs at or above this threshold go to stderr
  -v value
    	log level for V logs
  -vmodule value
    	comma-separated list of pattern=N settings for file-filtered logging
```

Example run: `./airlock username ExampleBucket ExampleFile` will export file `ExampleFile` to bucket `ExampleBucket`.

## Troubleshooting
See [troubleshooting](docs/troubleshooting.md) for fixes to known issues.

## License

Data Gateway is released under `MIT`, see [LICENSE](LICENSE).

[Wails](https://wails.io) is released under [MIT](https://github.com/wailsapp/wails/blob/master/LICENSE)
