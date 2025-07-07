# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Calendar Versioning](https://calver.org/).

## [Unreleased]

## [2025.7.0] - 2025-07-07

### Added

- (admins) env `CONFIG_ENDPOINT`, which is used to fetch the static `configuration.json` file from KrakenD
- an example `configuration.json` that can be used during development
- send object unencrypted also to CESSNA if project type is not `default` (#34)
- `findata-creds` container that enables `krakend` container to have mocked Findata EC2 credentials
- during local development, the CESSNA endpoint is defined as the `keystone-swift` S3 endpoint. The objects are however sent to the admin project.
- during local development, the env `IS_FINDATA` can be used to inform `mockauth` if the project has type `findata`
- the certificates for mTLS are embedded into the binary in the build stage of the CI pipeline
- mTLS for all PUT/POST/DELETE calls related to uploading objects, thus forcing the user to use GUI/Airlock CLI, which contain the necessary certificates
- Add lint and format check job to CI (#28)
- Add eslint (#26)
- (users) test if chosen mount point can be unmounted in case previous runtime failed to unmount directory
- (users) log level `trace` that shows logs from libfuse
- `go mod vendor` to `jf audit` pipeline to pre-emptively handle changing checksums in upstream package repos
- run `wails generate module` in renovate `postUpgradeTasks`

### Changed

- (users) Update export page (#32)
- (users) Updated login page (#29)
- S3 timeout is separate from the regular http timeout, determined by the timeout value in the `configuration.json` (#18)
- the KrakenD endpoints are determined from a `configuration.json` file that is fetched from env `CONFIG_ENDPOINT` during startup (#18)
- Findata objects are encrypted with the project public key
- project assumed to have either `findata` or `default` project type
- all references to SD Submit in code are now using SD Apply
- the way decrypted body size is calculated for encrypted files with no headers
- (users) files can only be uploaded via Data Gateway GUI or the Airlock CLI
- (users) uploaded files have a size limit of 5 TiB
- (users) objects uploaded to SD Connect are encrypted with the project-specific key
- large objects are uploaded in separate chunks of at least 128MiB
- airlock gets all user info from KrakenD and is no longer dependent on files in VM
- data export uses AWS S3
- Updated csc-ui to 2.3.4 (#25)
- (users) password is no longer required
- imported files are decrypted locally
- data import uses AWS S3
- binaries now work with the current KrakenD setup
- refactored the code to use libfuse instead of cgofuse
- (users) Updated service description text on login card (#22)
- replacing field `skip-pkg-cache` with `skip-cache` for `golangci-lint-action` in GitHub workflow

### Fixed

- (users) data import should be faster than before
- (users) `_segments` containers are hidden
- wails does not exit due to unmounting
- wails install in renovate

### Removed

- (users) Findata-related arguments from the airlock CLI
- (admins) support for Windows. Will be hopefully added back later

## [2024.6.0] - 2024-06-07

### Fixed

- GUI no longer crashes after update signal was changed to SIGUSR2

## [2024.05.0] - 2024-05-03

### Removed

- golang experimental package, because `slices` and `maps` are part of standard library as of `go 1.21`

## [2024.03.1] - 2024-03-20

### Changed

- bump version to `2024.03.1`

### Fixed

- github releases pipelines for GUI build

## [2024.03.0] - 2024-03-20

### Fixed

- UI showing white screen in GPU flavoured VMs

### Changed

- bump package to `2024.03.0`

## [2024.02.2] - 2024-02-13

### Changed

- In SD Apply, `FilePath` key is used to create subdirectories in the filesystem.

## [2024.02.1] - 2024-02-06

### Added

- gitlab releases

### Changed

- Updated dependencies

## [2024.02.0] - 2024-02-06

### Changed

- Chaged to calendar versioning

## [2.2.1]

### Changed
- Now macOS uses `fuser` to check if files are open, and Windows uses handle.exe.

## [2.2.0] 

### Changed
- Remove the spinner above the toggle for SD Connect when logging in

### Added
- Enable listing more than 10000 objects in bucket with `marker` query parameter
- In CLI, the `clear <path>` command clears the cache for all files under `path`

### Fixed
- Recover buttons after failed refresh in GUI
- SD Submit changed code so that `ready` state is now lower case, make use of `strings.EqualFold` 
for checking case insensitivity
- Checks envs before terminal state because the state might not be available

## [v2.1.6] 2023-08-04

### Fixed
- github actions updating pnpm

## [v2.1.5] 2023-07-27

### Changed
- Fix checksum calculation for files that need to be encrypted before exporting

## [v2.1.4] 2023-07-04

### Changed
- sign releases for windows

## [v2.1.3] 2023-06-02

### Changed
- Prevent logs from freezing UI by adding them at discrete intervals.

## [v2.1.2] 2023-05-15

### Changed
- Updated dependencies

## [v2.1.1] 2023-05-03

### Changed
- Mdi icons are taken from @mdi/font so that they work without internet.

## [v2.1.0] 2023-04-26

### Added
- User can update the filesystem by sending the SIGUSR1 signal to the process. Does not work for Windows.

## [v2.0.2] 2023-04-11

### Changed

- Update pnpm dependencies
- switch to pnpm v8

### Added

- dependabot for pnpm
- `csc-ui-vue` and deprecated `csc-ui-vue-directive`

## [v2.0.0] 2023-03-22

### Changed

- Refactor GUI to use Wails
- Renamed:
  - `SD Connect` to `SD-Connect` in the filesystem
  - `SD Apply` to `SD-Apply` in the filesystem
- In GUI, user can choose to access SD Connect, SD Apply or both.
- User has the option to login to SD Connect in export tab if they had originally only chosen SD Apply.

### Added

- `-project` parameter, which can be used to override the SD Connect project in the VM
- `-sdapply` parameter, which indicates the user only wants to access SD Apply

## [v1.4.1] - 2023-01-09

### Changed

- Update packages

## [v1.4.0] - 2022-12-02

### Changed
- `crypt4gh` function `NewCrypt4GHWriterWithoutPrivateKey` now uses list of public keys
- refactor packages to introduce more restictive linting (consistent camelCase vars, use of `switch`, fix formatting issues)

### Added
- Option to input airlock password from environment variable
- `CSC_USERNAME` and `CSC_PASSWORD` as options to set credentials for fuse layer

### Fixed
- Uploading mistakenly returned error for unencrypted files

## [v1.3.0] - 2022-11-18

### Changed
- User no longer chooses which repositories they wish to access, rather Data Gateway tries to access all of them after user has given their CSC credentials.
- small updates to UI components
- updated github actions to golang versions 1.18 and 1.19
  - github actions use newer syntax for getting tag name
  - add airlock cli to artifacts built in a release
- deprecated ioutils in code

### Added
- User can export files to SD Connect using the UI or command line if they are the project manager

### Fixed
- binary for linux needs to contain libQt5QuickShapes.* added that to the release
- fixed dependency for building properly on all OS

## [v1.2.2] - 2022-05-05

### Fixed
- Thumbnail generation was preventing users from updating filesystem. On Linux, this is not an issue anymore. On macOS and Windows, file browsers are not allowed to open/read files altogether.

## [v1.2.1] - 2022-04-21

### Fixed
- Cache overflow bug

## [v1.2.0] - 2022-04-20

### Changed
- Adjusting to changes in the SD Connect and SD Submit APIs
- small updates to UI

### Added
- User is able to filter logs in the UI
- Filesystem can be manually updated after mount. In the command line version, the user must type 'update' to update fuse. Update will not occur if there are files in use. Cache is cleared when updating

## [v1.1.0] - 2022-03-28

### Changed
- Renamed:
  - `SD-Submit` to `SD Apply` in the UI and logs
  - `SD-Connect` to `SD Connect` in the UI and logs
- Disable buttons in the UI if the required envs are missing
- improve error message shown to the user

## [v1.0.0] - 2022-03-04

### Changed
- github action for golangci-lint bumped to v1.44
- GUI has a new look
- `README.md` update with details regarding SD-Submit
- SD-Connect Proxy API reference documentation updated
- Project rebranded as Data Gateway

### Added
- run unit tests in github actions
- filesystem supports SD-Submit service
- SD-Submit API Reference documentation
- windows build and release via github actions

### Fixed
- gosec204 issue with `exec.Command` by processing to string the user input
- filter user input in logs

## [v0.1.0] - 2021-11-03
### Added
- SDA-Filesystem GUI (Graphical User Interface based on QT) and CLI (Command Line Interface) that is aimed to work with SD-Connect service.
- unit tests
- github action for golangci-lint
- github action for releasing to linux and darwin system

[Unreleased]: https://gitlab.ci.csc.fi/sds-dev/sd-desktop/sda-filesystem/compare/2025.7.0...HEAD
[2025.7.0]: https://gitlab.ci.csc.fi/sds-dev/sd-desktop/sda-filesystem/compare/2024.6.0...2025.7.0
[2024.6.0]: https://gitlab.ci.csc.fi/sds-dev/sd-desktop/sda-filesystem/-/compare/2024.05.0...2024.6.0
[2024.05.0]: https://gitlab.ci.csc.fi/sds-dev/sd-desktop/sda-filesystem/-/compare/2024.03.1...2024.05.0
[2024.03.1]: https://gitlab.ci.csc.fi/sds-dev/sd-desktop/sda-filesystem/-/compare/2024.03.0...2024.03.1
[2024.03.0]: https://gitlab.ci.csc.fi/sds-dev/sd-desktop/sda-filesystem/-/compare/2024.02.2...2024.03.0
[2024.02.2]: https://gitlab.ci.csc.fi/sds-dev/sd-desktop/sda-filesystem/-/compare/2024.02.1...2024.02.2
[2024.02.1]: https://gitlab.ci.csc.fi/sds-dev/sd-desktop/sda-filesystem/-/compare/2024.02.0...2024.02.1
[2024.02.0]: https://gitlab.ci.csc.fi/sds-dev/sd-desktop/sda-filesystem/-/releases/2024.02.0
