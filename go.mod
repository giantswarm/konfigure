module github.com/giantswarm/konfigure

go 1.14

require (
	github.com/Masterminds/sprig/v3 v3.2.0
	github.com/fatih/color v1.10.0
	github.com/ghodss/yaml v1.0.0
	github.com/giantswarm/apiextensions/v3 v3.18.0
	github.com/giantswarm/microerror v0.3.0
	github.com/giantswarm/micrologger v0.5.0
	github.com/giantswarm/operatorkit/v4 v4.0.0
	github.com/giantswarm/valuemodifier v0.3.1
	github.com/go-test/deep v1.0.7 // indirect
	github.com/google/go-cmp v0.5.4
	github.com/hashicorp/go-retryablehttp v0.6.7 // indirect
	github.com/hashicorp/vault/api v1.0.5-0.20201001211907-38d91b749c77
	github.com/hashicorp/vault/sdk v0.1.14-0.20201109203410-5e6e24692b32 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/kr/pretty v0.2.1 // indirect
	github.com/mitchellh/mapstructure v1.3.3 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/spf13/cobra v1.1.1
	github.com/tidwall/pretty v1.0.1 // indirect
	go.mongodb.org/mongo-driver v1.4.2 // indirect
	go.uber.org/zap v1.14.1 // indirect
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0 // indirect
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	honnef.co/go/tools v0.0.1-2020.1.3 // indirect
	k8s.io/api v0.18.9
	k8s.io/apimachinery v0.18.9
)

replace (
	github.com/coreos/etcd v3.3.10+incompatible => github.com/coreos/etcd v3.3.25+incompatible
	github.com/coreos/etcd v3.3.13+incompatible => github.com/coreos/etcd v3.3.25+incompatible
	github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2 // CVE-2021-3121
	github.com/opencontainers/runc v0.1.1 => github.com/opencontainers/runc v1.0.0-rc93
	sigs.k8s.io/cluster-api => github.com/giantswarm/cluster-api v0.3.10-gs
)
