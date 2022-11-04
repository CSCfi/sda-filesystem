# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- User no longer chooses which repositories they wish to access, rather Data Gateway tries to access all of them after user has given their CSC credentials.
- small updates to UI components
- updated github actions to golang versions 1.18 and 1.19
  - github actions use newer syntax for getting tag name
  - add airlock cli to artifacts built in a release
- deprecated ioutils in code

### Added
- User can export files to SD Connect using the UI if they are the project manager

### Fixed
-  binary for linux needs to contain libQt5QuickShapes.* added that to the release

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
