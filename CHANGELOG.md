# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/giantswarm/config-controller/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/giantswarm/config-controller/releases/tag/v0.1.0
