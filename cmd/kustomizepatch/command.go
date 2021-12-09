package kustomizepatch

import (
	"io"
	"os"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/spf13/cobra"
)

const (
	name             = "kustomizepatch"
	shortDescription = "Generate application configuration as a kustomize patch."
	longDescription  = `
konfigure kustomizepatch is designed to be launched as an "Exec KRM Function"
for kustomize (see https://kubectl.docs.kubernetes.io/guides/extending_kustomize/exec_krm_functions/).
You can execute it on your machine to debug it or run it with Flux in a cluster.

Both modes will require kustomization to reference konfigure binary as a plugin
and contain some configuration parameters in its body:

    === dir/kustomization.yaml ===
    generators:
    - generator.yaml
    === END ===

    === dir/generator.yaml ===
    apiVersion: generators.giantswarm.io/v1
    kind: Konfigure
    metadata:
      name: konfigure-plugin
      annotations:
        config.kubernetes.io/function: |
          exec:
            path: /plugins/konfigure
    app_catalog: ""
    app_destination_namespace: ""
    app_disable_force_upgrade: true
    app_name: ""
    app_version: ""
    name: ""
    === END ===

## In-cluster, with Flux

Required environment variables:
- KONFIGURE_MODE="kustomizepatch" has to be set to execute 'konfigure
  kustomizepatch' as main command, i.e. 'konfigure' will run 'konfigure
  kustomizepatch'
- KONFIGURE_INSTALLATION - name of current Manangement Cluster, e.g. "ginger"
- KONFIGURE_GITREPO - namespace/name of GitRepository CR pointing to
  giantswarm/config, e.g. "flux-system/giantswarm-config"
- KONFIGURE_SOURCE_SERVICE - K8s address of source-controller's service, e.g.
  "source-controller.flux-system.svc"
- VAULT_ADDR, VAULT_CAPATH, VAULT_TOKEN - required by vaultclient

Cache location:
konfigure kustomizepatch will use /tmp/konfigure-cache as its cache location.
The directory is expected to exist and the command will fail if it doesn't.

## local machine

Required environment variables:
- KONFIGURE_MODE="kustomizepatch" has to be set to execute 'konfigure
  kustomizepatch' as main command, i.e. 'konfigure' will run 'konfigure
  kustomizepatch'
- KONFIGURE_DIR - path to directory containing giantswarm/config, e.g.
  "/home/me/gs/config"
- KONFIGURE_INSTALLATION - name of current Manangement Cluster, e.g. "ginger"
- VAULT_ADDR, VAULT_CAPATH, VAULT_TOKEN - required by vaultclient

Cache will not be created/used in this mode.

To build kustomization with the plugin enabled, run

    kustomize build --enable-alpha-plugins --enable-exec dir/

where 'dir' is the location of the kustomization (contains a kustomization.yaml
file).
`
)

type Config struct {
	Logger micrologger.Logger
	Stderr io.Writer
	Stdout io.Writer
}

func New(config Config) (*cobra.Command, error) {
	if config.Logger == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.Logger must not be empty", config)
	}
	if config.Stderr == nil {
		config.Stderr = os.Stderr
	}
	if config.Stdout == nil {
		config.Stdout = os.Stdout
	}

	r := &runner{
		logger: config.Logger,
		stderr: config.Stderr,
		stdout: config.Stdout,
	}

	c := &cobra.Command{
		Use:   name,
		Short: shortDescription,
		Long:  longDescription,
		RunE:  r.Run,
	}

	return c, nil
}
