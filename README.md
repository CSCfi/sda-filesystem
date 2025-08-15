# Data Gateway

[![Linting go code](https://github.com/CSCfi/sda-filesystem/actions/workflows/linting.yml/badge.svg)](https://github.com/CSCfi/sda-filesystem/actions/workflows/linting.yml)
[![Unit Tests](https://github.com/CSCfi/sda-filesystem/actions/workflows/unittest.yml/badge.svg)](https://github.com/CSCfi/sda-filesystem/actions/workflows/unittest.yml)
[![Coverage Status](https://coveralls.io/repos/github/CSCfi/sda-filesystem/badge.svg?branch=main)](https://coveralls.io/github/CSCfi/sda-filesystem?branch=main)

Data Gateway builds a FUSE (Filesystem in Userspace) layer and uses an [Amazon S3 SDK](https://docs.aws.amazon.com/code-library/latest/ug/go_2_s3_code_examples.html) to import files from [SD Connect](https://research.csc.fi/service/sd-connect/) and [SD Apply](https://research.csc.fi/service/sd-apply/), and export files to SD Connect. It is designed to be used inside an [SD Desktop](https://research.csc.fi/service/sd-desktop/) VM. Software currently supports Linux and macOS for:
- Graphical User Interface
- Command Line Interface

Released binaries are built for Linux and are available in Artifactory. **New releases will no longer be available in Github**.

## 💻 Development

<details><summary>Click to expand</summary>

### Prerequisites
- Go version 1.24+
- Docker
- On Linux, install `pkg-config`, `libfuse3-dev` and `socat` with `apt-get`.
- On macOS, install `pkg-config` and `socat` with Homebrew, and install [macFUSE](https://github.com/macfuse/macfuse/wiki/Getting-Started)

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

Since internal URLs are recommended not to be public, the environmental variable `VAULT_ADDR` has to be defined manually. Follow the instructions [here](https://gitlab.ci.csc.fi/groups/sds-dev/-/wikis/Guides/Development-tools/using-vault#accessing-vault-via-terminal) on how to make the Vault address available to shell commands. Once the address is defined, continue with the following command:

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

There are four different profiles defined in [docker-compose.yml](compose/docker-compose.yml): `krakend`, `keystone`, `fuse` and [`findata`](#findata-projects). In addition, there are matching `.env.*` files in [`dev-tools/compose`](./dev-tools/compose). By selecting a profile and its matching `.env` file, you can select which services you wish to run locally. Makefile targets `build_profiles` and `run_profiles` take advantage of this feature. Note that `build_profiles` builds and runs the selected profiles, whereas `run_profiles` just runs them.

The `Makefile` is written so that these two aforementioned targets can be given the desired profiles as arguments, for example:

```
make build_services krakend keystone
```

This command is equivalent to `make local`.

Profile `keystone` signifies that you do not want to use data from Allas. Instead, the AAI, vault, S3 storage, and the keystone service are all run locally, and the FUSE will access data generated by the `data-upload` container. The `keystone` profile must be accompanied by the `krakend` profile, since all calls to keystone-related endpoints go through KrakenD. You can, however, run the `krakend` profile without the `keystone` profile. By running `make build_services krakend`, you can use a local version of KrakenD but still use data from Allas. This may to useful in case you need to debug a problem on the KrakenD side.

Profile `fuse` sets up an Ubuntu 24.04 container, similar to the environment in SD Desktop, which you can use to run the CLI. You just need to run

```
make exec
````

to access the container, and then type `./gateway` to run the binary. The `fuse` profile is not used in the `local` target because targets `gui` and `cli` run their binaries on the developer's own computer environment.

An equivalent version of `make remote` in this instance is just `make build_services` without any profile arguments. This command only sets up `terminal-proxy`, as it is the only service without a profile.

### Findata projects

The behaviour of Findata projects is slightly different to that of the default projects. If you want to test the code with a Findata project, add profile `findata` to either the `run_profiles` or `build_profiles` command.

When the user reads a file from a Findata project, the file content is also scanned for viruses by [ClamAV](https://docs.clamav.net/Introduction.html) with the help of the `clamd` daemon. In SD Desktop, Data Gateway connects to `clamd` with a Unix socket, the location of which is available in the environment variable `CLAMAV_SOCKET`. When developing Data Gateway, this behaviour is emulated with a ClamAV docker image that has its TCP socket open to the host computer. Since Data Gateway requires a Unix socket, one is created under directory `$HOME/.clamv/` with the `socat` command, which also redirects the connections to the TCP socket. This process assumes that the container's port is exposed on localhost.

Another difference occurs during export. Not only are the objects exported to SD Connect, they are also exported to CESSNA for inspection. CESSNA has a single bucket for all data, which KrakenD knows and will add to the request. Therefore, the bucket name will remain empty on Data Gateway's side when exporing to CESSNA and the object name will have the SD Connect bucket prepended to it. In addition, CESSNA expects a journal number and the user's email in the metadata of the request.

### Running the binaries

In case the binaries are run without the help of a `Makefile`, please be reminded that both GUI and CLI versions require environment variables `PROXY_URL`, `CONFIG_ENDPOINT` and `SDS_ACCESS_TOKEN` to function. Findata projects also need `CLAMAV_SOCKET`. After the development environment is set up, you can run
```
export $(make envs)
```
to get the correct values exported.

#### Graphical User Interface

`make gui` runs the GUI in [development mode](https://wails.io/docs/reference/cli#dev):
```bash
cd cmd/gui
wails dev
```

In development mode, the application assets are automatically reloaded when they are changed, and you can inspect elements. However, in some computers, the development mode does not function properly, and you will have to open the application in the browser (the wails logs will tell you which localhost port to use). The issue should no longer be present in the [production-ready](https://wails.io/docs/reference/cli#build) binary. You can build and run it with  `make gui_prod`. It builds the binary with `make gui_build`:
```bash
cd cmd/gui
wails build -upx -trimpath -clean -s
```
Note that the `-upx` flag is optional and the app will build faster without it. You may need to add `-tags webkit2_41` to the wails commands to be able to build/run on Ubuntu 24.04. This is already taken care of in the Makefile.

[comment]: # (# For Windows)
[comment]: # (wails build -upx -trimpath -clean -s -webview2=embed)

#### Command Line Interface

The CLI binary has two subcommands, `import` and `export`, which can be used to setup the filesystem and upload files to SD Connect, respectively.

To build the binary:
```bash
go build -o ./data-gateway-cli ./cmd/cli
```

Accepted common command line arguments:
```
-http_timeout int
    	Number of seconds to wait before timing out an HTTP request (default 20)
-loglevel string
    	Logging level. Possible values: {trace,debug,info,warning,error} (default "info")
```

When running the binary, the common arguments are placed before the subcommand, and the subcommand-specific arguments are placed after the subcommand:
```bash
./data-gateway-cli [arguments] [subcommand] [subcommand arguments]
```

##### Import

Accepted command line arguments for import:
```
./data-gateway-cli import -help
Usage of import:
  -mount string
    	Path to Data Gateway mount point
```
Example run `./data-gateway-cli import -mount=$HOME/ExampleMount` will create the FUSE layer in the directory `$HOME/ExampleMount` for both `SD Connect` and `SD Apply`. If no mount point is specified, the filesystem will be mounted in `$HOME/Projects`.

##### Export

Accepted command line arguments for export:
```
./data-gateway-cli export -help
Usage of export:
  -override
    	Forcibly override data in SD Connect
```
Example run `./data-gateway-cli export example-bucket exampleFile.txt` will export file `exampleFile.txt` to bucket `example-bucket`.

In an SD Desktop VM, the user will only be able to upload files with either the GUI or CLI binary due to mutual TLS being enabled at certain endpoints in terminal-proxy. The necessary certificate files will be embedded into the binaries during a CI job.

The file that is being uploaded is assumed to be unencrypted; the program encrypts it with public keys that it fetches via KrakenD.

If you wish to test out file export with Findata projects, redefine environment variable `IS_FINDATA` in `.env` as `true`. Notice that this will only affect the project type if you run everything locally.

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

### Unit tests

The provided unit tests can be run with:

```sh
go test ./...
```
### Frontend linting and formatting

Eslint is used for both linting and formatting checks. You can find the defined rules in `eslint.config.js`.

```sh
cd frontend

pnpm run lint
# OR check and fix errors
pnpm run lint --fix
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

### Cannot open GUI application on MacOS

If clicking the icon produces a message which says that application cannot be opened or the program is killed in command line, decompressing the binary might help.

Install upx with Homebrew
```
brew install upx
```

Decompress the binary
```
upx -d data-gateway.app/Contents/MacOS/data-gateway
```

### Directory is not unmounted after program crash
#### Linux
```
fusermount -u <path>
```

#### MacOS
```
umount <path>
```

### File was updated in archive, but program displays old file
#### GUI
Click on the `Update` button to clear cached files.
#### CLI
Write `update` in the terminal where the process is running.
```
update
```

### Environment variables don't seem to be correct even though I am sure I am running the correct commands

Make sure you have not exported any of the environment variables in you command line when running docker compose. They will override any envs given in the files.


</details>

## 📜 License

<details><summary>Click to expand</summary>

Data Gateway is released under `MIT`, see [LICENSE](LICENSE).

[Wails](https://wails.io) is released under [MIT](https://github.com/wailsapp/wails/blob/master/LICENSE)

</details>
