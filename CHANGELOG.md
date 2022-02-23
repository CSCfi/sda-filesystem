# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- github action for golangci-lint bumped to v1.44
- GUI has a new look

### Added
- run unit tests in github actions
- filesystem supports SD-Submit service
- support for windows (no github actions yet)

### Fixed
- gosec204 issue with `exec.Command` by processing to string the user input
- filter user input in logs

## [v0.1.0] - 2021-11-03
### Added
- SDA-Filesystem GUI (Graphical User Interface based on QT) and CLI (Command Line Interface) that is aimed to work with SD-Connect service.
- unit tests
- github action for golangci-lint
- github action for releasing to linux and darwin system
