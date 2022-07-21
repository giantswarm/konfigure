module github.com/giantswarm/konfigure

go 1.16

require (
	github.com/Masterminds/sprig/v3 v3.2.0
	github.com/Microsoft/go-winio v0.4.15-0.20190919025122-fc70bd9a86b5 // indirect
	github.com/containerd/continuity v0.0.0-20200709052629-daa8e1ccc0bc // indirect
	github.com/fatih/color v1.13.0
	github.com/fluxcd/pkg/untar v0.1.0
	github.com/ghodss/yaml v1.0.0
	github.com/giantswarm/apiextensions-application v0.3.0
	github.com/giantswarm/app/v6 v6.7.0
	github.com/giantswarm/k8smetadata v0.9.2
	github.com/giantswarm/microerror v0.4.0
	github.com/giantswarm/micrologger v0.6.0
	github.com/giantswarm/valuemodifier v0.5.0
	github.com/go-test/deep v1.0.7 // indirect
	github.com/google/go-cmp v0.5.7
	github.com/hashicorp/vault/api v1.7.2
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/spf13/cobra v1.2.1
	go.mozilla.org/sops/v3 v3.7.2
	go.uber.org/config v1.4.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.21.4
	k8s.io/apimachinery v0.21.4
	k8s.io/client-go v0.21.4
	sigs.k8s.io/controller-runtime v0.9.7
	sigs.k8s.io/kustomize/kyaml v0.13.6
	sigs.k8s.io/yaml v1.3.0
)

replace (
	github.com/coreos/etcd v3.3.10+incompatible => github.com/coreos/etcd v3.3.25+incompatible
	github.com/coreos/etcd v3.3.13+incompatible => github.com/coreos/etcd v3.3.25+incompatible
	github.com/dgrijalva/jwt-go => github.com/golang-jwt/jwt/v4 v4.0.0
	github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2 // CVE-2021-3121
	github.com/opencontainers/runc v0.1.1 => github.com/opencontainers/runc v1.0.0-rc93
	sigs.k8s.io/cluster-api => github.com/giantswarm/cluster-api v0.3.10-gs
	sigs.k8s.io/kustomize/api => sigs.k8s.io/kustomize/api v0.11.4
	sigs.k8s.io/kustomize/kyaml => sigs.k8s.io/kustomize/kyaml v0.13.6
)
