# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.10] - 2021-04-13

### Fixed

- Use `text/template` instead of `html` to avoid escaping strings.
- Return a more descriptive error when given invalid YAML.

## [0.2.9] - 2021-04-02

### Fixed

- Bump protobuf to v1.3.2 (CVE-2021-3121)

## [0.2.8] - 2021-03-24

### Fixed

- Prevent panic when linter cross-references apps and installations.

## [0.2.7] - 2021-03-23

### Fixed

- Skip non-existent application patches when linting.

## [0.2.6] - 2021-02-22

### Fixed

- Bring back `application/v1alpha1` API extension to the registered schemas.

## [0.2.5] - 2021-02-22


### Added

- Add `skip-validation-regexp` to skip selected fields validation.

### Deleted

- Delete `App` CR controller.


## [0.2.4] - 2021-02-16

### Added

- Add configuration linter under `lint` command.
- Add logs when generating config via CLI.
- Reconcile Config CRs.

### Fixed

- Throw errors when template keys are missing.

## [0.2.3] - 2021-01-28

### Fixed

- Add missing `giantswarm.io/monitoring-*` annotations.
- Update configuration ConfigMap ans Secret only when they change.
- Retry App CR modifications on conflicts.

## [0.2.2] - 2021-01-19

### Fixed

- Add `giantswarm.io/monitoring: "true"` label to the Service to make sure the
  app is scraped by the new monitoring platform.
- Resolve catalog URL using storage URL from AppCatalog CR rather than using
  simple format string.

## [0.2.1] - 2021-01-14

### Fixed

- Remove old ConfigMap and Secret when a new config version is set.

### Fixed

- Use `config.giantswarm.io/version` Chart annotation to determine configuration version.

## [0.2.0] - 2021-01-12

### Added
- Add `values` handler, which generates App ConfigMap and Secret.
- Allow caching tags and pulled repositories.
- Handle `app-operator.giantswarm.io/pause` annotation.
- Clear `app-operator.giantswarm.io/pause` if App CR does is not annotated with config version.
- Annotate App CR with config version defined in catalog's `index.yaml`.

## [0.1.0] - 2020-11-26

### Added

- Create CLI/daemon scaffolding.
- Generate application configuration using `generate` command.

[Unreleased]: https://github.com/giantswarm/config-controller/compare/v0.2.10...HEAD
[0.2.10]: https://github.com/giantswarm/config-controller/compare/v0.2.9...v0.2.10
[0.2.9]: https://github.com/giantswarm/config-controller/compare/v0.2.8...v0.2.9
[0.2.8]: https://github.com/giantswarm/config-controller/compare/v0.2.7...v0.2.8
[0.2.7]: https://github.com/giantswarm/config-controller/compare/v0.2.6...v0.2.7
[0.2.6]: https://github.com/giantswarm/config-controller/compare/v0.2.5...v0.2.6
[0.2.5]: https://github.com/giantswarm/config-controller/compare/v0.2.4...v0.2.5
[0.2.4]: https://github.com/giantswarm/config-controller/compare/v0.2.3...v0.2.4
[0.2.3]: https://github.com/giantswarm/config-controller/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/giantswarm/config-controller/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/giantswarm/config-controller/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/giantswarm/config-controller/releases/tag/v0.2.0
[0.1.0]: https://github.com/giantswarm/config-controller/releases/tag/v0.1.0
