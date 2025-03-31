# Data Gateway

[![Linting go code](https://github.com/CSCfi/sda-filesystem/actions/workflows/linting.yml/badge.svg)](https://github.com/CSCfi/sda-filesystem/actions/workflows/linting.yml)
[![Unit Tests](https://github.com/CSCfi/sda-filesystem/actions/workflows/unittest.yml/badge.svg)](https://github.com/CSCfi/sda-filesystem/actions/workflows/unittest.yml)
[![Coverage Status](https://coveralls.io/repos/github/CSCfi/sda-filesystem/badge.svg?branch=main)](https://coveralls.io/github/CSCfi/sda-filesystem?branch=main)

Data Gateway builds a FUSE (Filesystem in Userspace) layer and uses an [Amazon S3 SDK](https://docs.aws.amazon.com/code-library/latest/ug/go_2_s3_code_examples.html) to import files from [SD Connect](https://research.csc.fi/service/sd-connect/) and [SD Apply](https://research.csc.fi/service/sd-apply/), and export files to SD Connect. It is designed to be used inside an [SD Desktop](https://research.csc.fi/service/sd-desktop/) VM. Software currently supports Linux and macOS for:
- Graphical User Interface
- Command Line Interface

Binaries are built on each release for all supported Operating Systems in Github. In addition, Artifactory contains binaries for Linux.

## 💻 Development

<details><summary>Click to expand</summary>

### Prerequisites
- Go version 1.24+
- Docker
- On Linux, install `pkg-config` and `libfuse3-dev` with `apt-get`.
- On macOS, install `pkg-config` with Homebrew, and install [macFUSE](https://github.com/macfuse/macfuse/wiki/Getting-Started)

In addition, the GUI requires

- [Wails](https://wails.io/docs/gettingstarted/installation) and its dependencies.
- [pnpm](https://pnpm.io/installation)

### Components

Data Gateway binaries cannot function without all the following components:

- A [nginx proxy](https://gitlab.ci.csc.fi/sds-dev/sd-platform/generic-terminal-proxy) (also called `terminal-proxy`) through which all calls from the VM are routed. It's main purpose it to add various headers to the request.
- A [KrakenD API gateway](https://gitlab.ci.csc.fi/sds-dev/sd-desktop/krakend-api-gateway) that deals with routing, authentication, and many important modifications to the requests and responses.
- An AAI, which the API gateway uses to authenticate the user.
  - SDS AAI in production
- An object storage that can be accessed with AWS S3. Files in this storage are encrypted with Crypt4gh.
- A [plugin for Hashicorp Vault](https://gitlab.ci.csc.fi/sds-dev/c4gh-transit) to store Crypt4gh-encrypted file headers.
- An [Openstack Keystone](https://docs.openstack.org/keystone/latest/) service
  - CSC Pouta in production

All of these components can be run locally either from pulled images or mocked components in [docker-compose.yml](./compose/docker-compose.yml). A [Makefile](Makefile) is provided for ease of use.

### Makefile commands

You can run `make` to see the commands available to you.

#### Setting up

Start with the following command:

```
make requirements
```

This command ensures that you are logged in to Artifactory, generates the frontend assests for the GUI, and creates an `.env` file under [`dev-tools/compose`](./dev-tools/compose). This file is then filled with secrets from our test Vault, an action which will require you to login via the browser.

Once the `.env` file is created, there is one environment variable, `SDS_ACCESS_TOKEN`, that you need to fill in youself. `SDS_ACCESS_TOKEN` is an opaque token for authenticating to the api gateway with the help of the AAI. Instructions for getting a valid access token are [here](https://gitlab.ci.csc.fi/groups/sds-dev/-/wikis/KrakenD/Other-resources/OIDC-Client-and-Access-Tokens). This token will expire after a certain amount of hours, so it will have to be refetched at set intervals. In SD Desktop, the user gets a new token every time they log in.

#### Run and build

```
make all
```

builds and runs all the components locally and, once they are up and running, starts the GUI version of the filesystem. This commands is equivalent to running `make local gui`. Similarly, running `make remote cli` would start up a mock nginx proxy that connects to our test cluster KrakenD, in addition to running the CLI version of the filesystem. These four targets (`local`, `remote`, `cli`, and `gui`) can be combined in the following ways:

```
make local gui   # same as `make all`
make local cli
make remote gui
make remote cli
```

Running all components locally gives you the advantage of easily seeing all the logs, whereas connecting to the test cluster KrakenD enables you to access data from Allas.

You can stop and remove the running containers with the command `make down`.

#### More advanced use cases

There are three different profiles defined in [docker-compose.yml](compose/docker-compose.yml): `fuse`, `krakend` and `keystone`. In addition, there are matching `.env.*` files in [`dev-tools/compose`](./dev-tools/compose). By selecting a profile and its matching `.env` file, you can select which services you wish to run locally. Makefile targets `build_profiles` and `run_profiles` take advantage of this feature. Note that `build_profiles` builds and runs the selected profiles, whereas `run_profiles` just runs them.

The `Makefile` is written so that these two aforementioned targets can be given the desired profiles as arguments, for example:

```
make build_services krakend keystone
```

This command is equivalent to `make local`. The reason why profile `fuse` is not listed for `local` is due to the fact that targets `gui` and `cli` run their binaries on the developer's own computer environment. Profile `fuse` sets up an Ubuntu 24.04 container, similar to the environment in SD Desktop, which you can use to run the CLI. You just need to run

```
make exec
````

to access the container, and then type `./gateway` to run the binary.

Profile `keystone` signifies that you do not want to use data from Allas. Instead, the AAI, vault, S3 storage, and the keystone service are all run locally, and the FUSE will access data generated by the `data-upload` container. The `keystone` profile must be accompanied by the `krakend` profile, since all calls to keystone-related endpoints go through KrakenD.

By running `make build_services krakend`, you can use a local version of KrakenD but still use data from Allas. This may to useful in case you need to debug a problem on the KrakenD side.

On the other hand, an equivalent version of `make remote` in this instance is just `make build_services` without any profile arguments. This command only sets up `terminal-proxy`, as it is the only service without a profile.

### Running the binaries

In case the binaries are run without the help of a `Makefile`, it should be reminded that both GUI and CLI versions require environment variables `PROXY_URL` and `SDS_ACCESS_TOKEN` to function. After the development environment is set up, you can run
```
export $(grep -E '^PROXY_URL|^SDS_ACCESS_TOKEN' $(git rev-parse --show-toplevel)/dev-tools/compose/.env | xargs)
```
to get the correct values exported.

#### Graphical User Interface

`make gui` runs the GUI in [development mode](https://wails.io/docs/reference/cli#dev):
```bash
cd cmd/gui
wails dev
```

In development mode, the application assets are automatically reloaded when they are changed, and you can inspect elements. However, in some computers, the development mode does not function properly, and you will have to open the application in the browser (the wails logs will tell you which localhost port to use). The issue should no longer be present in the [production-ready](https://wails.io/docs/reference/cli#build) binary, which can be created by running:
```bash
cd cmd/gui
wails build -upx -trimpath -clean -s
```
or by simply with `make gui_build`. Note that the `-upx` flag is optional and the app will build faster without it. You may need to add `-tags webkit2_41` to the wails commands to be able to build/run on Ubuntu 24.04. This is already taken care of in the Makefile.

[comment]: # (# For Windows)
[comment]: # (wails build -upx -trimpath -clean -s -webview2=embed)

#### Command Line Interface

Two CLI binaries are released, one for mounting the FUSE ([SDA-Filesystem](#sda-fileystem)) and one for exporting files ([Airlock](#airlock)). The Makefile target `cli` runs the former.

##### SDA-Fileystem

To build the binary:
```bash
go build -o ./go-fuse ./cmd/fuse/main.go
```
Accepted command line arguments:
```bash
./go-fuse -help
Usage of ./go-fuse:
  -http_timeout int
    	Number of seconds to wait before timing out an HTTP request (default 60)
  -loglevel string
    	Logging level. Possible values: {trace,debug,info,warning,error} (default "info")
  -mount string
    	Path to Data Gateway mount point

```
Example run `./go-fuse -mount=$HOME/ExampleMount` will create the FUSE layer in the directory `$HOME/ExampleMount` for both `SD Connect` and `SD Apply`. If no mount point is specified, the filesystem will be mounted in `$HOME/Projects`.

##### Airlock

To build the binary:
```bash
go build -o ./airlock ./cmd/airlock/main.go
```
Accepted command line arguments:
```bash
./airlock -help
Usage of ./airlock:
  -debug
    	Enable debug prints
  -quiet
    	Print only errors
  -segment-size int
    	Maximum size of segments in Mb used to upload data. Valid range is 10-4000. (default 4000)
```
Example run `./airlock username ExampleBucket ExampleFile` will export file `ExampleFile` to bucket `ExampleBucket`.

</details>


## 📚 Usage

<details><summary>Click to expand</summary>

### User commands

User can update the CLI version of the filesystem by typing in the command line the word `update`. This requires that no files inside the filesystem are being used. Update also clears cache. As a result of this operation, new files may be added and some old ones removed. The filesystem in the GUI can be updated with a simple click of a button.

The filesystem can be also updated programatically with the `SIGUSR2` signal in both CLI and GUI.

To update the filesystem on bash in SD Desktop:
```bash
# Update CLI version
kill -s SIGUSR2 $(pgrep go-fuse)

# Update GUI version
kill -s SIGUSR2 $(pgrep sda-fuse)
```

If the user wants to update particular SD Connect files inside the filesystem, the user can input command `clear <path>`. `<path>` is the path to the file/folder that the user wishes to update. `<path>` must at least contain a bucket, i.e. `SD-Connect/project/bucket` or `SD-Connect/project/bucket/file` would be acceptable paths, but not, e.g., `SD-Connect/project`. If the user gives a path to a folder, all files inside this folder are updated but no files are added or removed. This operation clears the cache for all the relevant files so that the new content is read from the storage and sizes of these files are updated in the filesystem.

### Libfuse buffer size

The maximum read buffer size in libfuse is at the moment 262144 bytes. It can be increased to 1 MiB with:

```
dev=$(stat --format="%Hd:%Ld" $HOME/Projects) && echo "1024" > /sys/class/bdi/${dev}/read_ahead_kb
```

Since this requires root access, it may not be possible to implement this in production.

</details>


## 🧪 Testing

<details><summary>Click to expand</summary>

The provided unit tests can be run with:

```sh
go test ./...
```
</details>

## 🚀 Deployment

<details><summary>Click to expand</summary>

See [Linux setup](docs/linux-setup.md) for setting up the GUI for production. Currently all the binaries are added to the VM by the [customer-vm repository](https://gitlab.ci.csc.fi/sds-dev/sd-desktop/customer-vm/-/blob/main/config/linux/setup-sd-software.sh).

</details>

## 🛠️ Contributing

<details><summary>Click to expand</summary>

Development team members should check internal [contributing guidelines for Gitlab](https://gitlab.ci.csc.fi/groups/sds-dev/-/wikis/Guides/Contributing).

If you are not part of CSC and our development team, your help is nevertheless very welcome. Please see [contributing guidelines for Github](CONTRIBUTING.md).

</details>

## ⁉️ Troubleshooting

<details><summary>Click to expand</summary>

See [troubleshooting](docs/troubleshooting.md) for fixes to known issues.

</details>

## 📜 License

<details><summary>Click to expand</summary>

Data Gateway is released under `MIT`, see [LICENSE](LICENSE).

[Wails](https://wails.io) is released under [MIT](https://github.com/wailsapp/wails/blob/master/LICENSE)

</details>
