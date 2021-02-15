# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Add configuration linter under `lint` command.
- Add logs when generating config via CLI.

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

[Unreleased]: https://github.com/giantswarm/config-controller/compare/v0.2.3...HEAD
[0.2.3]: https://github.com/giantswarm/config-controller/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/giantswarm/config-controller/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/giantswarm/config-controller/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/giantswarm/config-controller/releases/tag/v0.2.0
[0.1.0]: https://github.com/giantswarm/config-controller/releases/tag/v0.1.0
