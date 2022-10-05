[![CircleCI](https://circleci.com/gh/giantswarm/konfigure.svg?&style=shield)](https://circleci.com/gh/giantswarm/konfigure)
[![Docker Repository on Quay](https://quay.io/repository/giantswarm/konfigure/status)](https://quay.io/repository/giantswarm/konfigure)

# konfigure

Konfigure is a CLI tool and a
[kustomize](https://github.com/kubernetes-sigs/kustomize) plugin designed to
compile App configuration.

The CLI mode enables you to generate the full configuration (App CR, ConfigMap,
and Secret) for any of the Apps on any of Management Clusters. It also provides
a linter to make sure the configuration structure found in
[giantswarm/config](https://github.com/giantswarm/config) conforms with our
standards.

The _kustomize_ plugin mode enables us to utilize `konfigure` with Flux's
`kustomize-controller` to transform App collection manifests into sets of App
CRs, ConfigMaps, and Secrets.

Documentation: [intranet](https://intranet.giantswarm.io/docs/dev-and-releng/configuration-management/) | [GitHub](https://github.com/giantswarm/giantswarm/blob/master/content/docs/dev-and-releng/configuration-management/_index.md)
