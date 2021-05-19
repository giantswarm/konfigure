# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2021-05-19

### Added

- Add `version` command.

## [0.1.0] - 2021-05-12

### Added

- Create App CR, with injected ConfigMap and Secret details, when generating config.
- Add suffix to ConfigMap and Secret names.
- Get Vault credentials from Secret.

### Changed

- Start of a new history. Previous commits have been imported from https://github.com/giantswarm/config-controller.
- Use local filesystem instead of GitHub as configuration source.

[Unreleased]: https://github.com/giantswarm/konfigure/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/giantswarm/konfigure/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/giantswarm/konfigure/compare/fc16094...v0.1.0
