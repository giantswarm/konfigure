# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.16.0] - 2024-02-20

### Added

- Added the `includeSelf` template function that reads include templates from the `include-self` folder in preparation
  of splitting the config repository to default and include submodules + specific config repositories

## [0.15.1] - 2023-10-24

### Changed

- Drop support for the `latest.tar.gz` and track the artifact advertised in the GitRepository .status.artifact.url.

## [0.15.0] - 2023-09-13

### Changed

- Log file paths in error messages so that flux logs tell us exactly where something went wrong

## [0.14.4] - 2023-04-06

## [0.14.3] - 2023-01-11

### Fixed

- Fix where git command is run in `konfigure generate` command.

### Changed

- Bump `giantswarm/valuemodifier` to `v0.5.2`

## [0.14.1] - 2022-11-09

### Fixed

- Fix `konfigure generate` failing on `config` repo shallow copies without tags.

## [0.14.0] - 2022-11-04

### Changed

- `konfigure generate` with `--raw` flat now generates single merged yaml document.
- `konfigure generate` with `--raw` doesn't require `--app-destination-namespace`, `--app-catalog`, `--app-version` and `--name` flags anymore.

## [0.13.0] - 2022-10-27

### Changed

- Update dependencies to support Flux v0.36.0.
- Update go.mod dependencies and update Golang to v1.19.

## [0.12.1] - 2022-10-20

### Changed

- Fetch SOPS keys from multiple namespaces.

## [0.12.0] - 2022-09-29

### Added

- `konfigure lint` now handles SOPS encrypted files

## [0.11.0] - 2022-08-03

### Changed

- The Vault client configuration is validated when `konfigure` actually tries to decrypt something with it instead at initialisation time, so e.g. `VAULT_*` environment variables can be omitted now when Vault is not used.

## [0.10.0] - 2022-07-21

## [0.9.0] - 2022-06-27

### Fixed

- `konfigure lint` now respects template escape markers.

## [0.8.0] - 2022-05-25

### Changed

- Update dependencies to support Flux v0.30.2.

## [0.7.0] - 2022-05-10

### Added

- Support SOPS with GnuPGP and AGE encryption.

## [0.6.0] - 2022-04-14

### Added

- Make errors related to `giantswarm/config` structure more descriptive.

## [0.5.6] - 2022-03-16

### Added

- Push image to docker hub as this is the registry we use in
management-clusters-fleet.

- Log additional context for errors occurring in `konfigure kustomizepatch`.

## [0.5.5] - 2022-02-21

### Fixed

- Fix InCluster flag in `generate` command.

## [0.5.4] - 2022-02-20

### Fixed

- Fixed new InCluster app config option and set to true

## [0.5.3] - 2022-02-18

### Fixed

- Add `giantswarm.io/managed-by` label so new app CRs in collections are not
blocked by app-admission-controller.

## [0.5.2] - 2022-02-15

### Fixed

- Remove logging - it breaks kustomize output.

## [0.5.1] - 2022-02-15

### Added

- Improve timeout when calling source-controller.
- Improve logging when running `konfigure kustomizepatch`.

## [0.5.0] - 2022-02-07

### Added

- Add `kustomizepatch` command, enabling konfigure to run as a kustomize plugin.

## [0.4.0] - 2022-02-03

### Fixed

- Replaced `giantswarm/valuemodifier` with `uber-go/config` for the purpose of merging YAML patches.


## [0.3.8] - 2021-09-15

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

[Unreleased]: https://github.com/giantswarm/konfigure/compare/v0.16.0...HEAD
[0.16.0]: https://github.com/giantswarm/konfigure/compare/v0.15.1...v0.16.0
[0.15.1]: https://github.com/giantswarm/konfigure/compare/v0.15.0...v0.15.1
[0.15.0]: https://github.com/giantswarm/konfigure/compare/v0.14.4...v0.15.0
[0.14.4]: https://github.com/giantswarm/konfigure/compare/v0.14.3...v0.14.4
[0.14.3]: https://github.com/giantswarm/konfigure/compare/v0.14.2...v0.14.3
[0.14.2]: https://github.com/giantswarm/konfigure/compare/v0.14.1...v0.14.2
[0.14.1]: https://github.com/giantswarm/konfigure/compare/v0.14.0...v0.14.1
[0.14.0]: https://github.com/giantswarm/konfigure/compare/v0.13.0...v0.14.0
[0.13.0]: https://github.com/giantswarm/konfigure/compare/v0.12.1...v0.13.0
[0.12.1]: https://github.com/giantswarm/konfigure/compare/v0.12.0...v0.12.1
[0.12.0]: https://github.com/giantswarm/konfigure/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/giantswarm/konfigure/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/giantswarm/konfigure/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/giantswarm/konfigure/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/giantswarm/konfigure/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/giantswarm/konfigure/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/giantswarm/konfigure/compare/v0.5.6...v0.6.0
[0.5.6]: https://github.com/giantswarm/konfigure/compare/v0.5.5...v0.5.6
[0.5.5]: https://github.com/giantswarm/konfigure/compare/v0.5.4...v0.5.5
[0.5.4]: https://github.com/giantswarm/konfigure/compare/v0.5.3...v0.5.4
[0.5.3]: https://github.com/giantswarm/konfigure/compare/v0.5.2...v0.5.3
[0.5.2]: https://github.com/giantswarm/konfigure/compare/v0.5.1...v0.5.2
[0.5.1]: https://github.com/giantswarm/konfigure/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/giantswarm/konfigure/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/giantswarm/konfigure/compare/v0.3.8...v0.4.0
[0.3.8]: https://github.com/giantswarm/konfigure/compare/v0.3.7...v0.3.8
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
