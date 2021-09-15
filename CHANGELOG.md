# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Improve include file discovery in linter.

## [0.3.7] - 2021-08-23

### Changed

- Replace github.com/dgrijalva/jwt-go
- Update valuemodifier to v0.4.0

### Fixed

- Fix generated YAML keys sorting.

## [0.3.6] - 2021-08-12

### Fixed

- Fix template path pattern.

## [0.3.5] - 2021-06-25

### Fixed

- Mark all config fields used in secrets.

## [0.3.4] - 2021-06-03

### Fixed

- Remove debug line.

## [0.3.3] - 2021-06-02

### Fixed

- Merge config and secret values before rendering secret template.

## [0.3.2] - 2021-05-28

### Fixed

- Use `App.spec.config` instead of `App.spec.userConfig`.
- Do not format strings in the rendered ConfigMap and Secret data.

## [0.3.1] - 2021-05-24

### Fixed

- Fix `populateSecretPathsWithUsedBy` when secret patch is not defined.

## [0.3.0] - 2021-05-21

### Added

- Add `--app-destination-namespace` flag to `generate` command.

## Removed

- Remove `--namespace` flag from `generate` command.
- Remove defaulting to "giantswarm" from `--name` flag in `generate` command.

### Fixed

- Do not render `status:` in `generate` command.

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

[Unreleased]: https://github.com/giantswarm/konfigure/compare/v0.3.7...HEAD
[0.3.7]: https://github.com/giantswarm/konfigure/compare/v0.3.6...v0.3.7
[0.3.6]: https://github.com/giantswarm/konfigure/compare/v0.3.5...v0.3.6
[0.3.5]: https://github.com/giantswarm/konfigure/compare/v0.3.4...v0.3.5
[0.3.4]: https://github.com/giantswarm/konfigure/compare/v0.3.3...v0.3.4
[0.3.3]: https://github.com/giantswarm/konfigure/compare/v0.3.2...v0.3.3
[0.3.2]: https://github.com/giantswarm/konfigure/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/giantswarm/konfigure/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/giantswarm/konfigure/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/giantswarm/konfigure/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/giantswarm/konfigure/compare/fc16094...v0.1.0
