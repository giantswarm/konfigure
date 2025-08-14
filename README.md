[![CircleCI](https://circleci.com/gh/giantswarm/konfigure.svg?&style=shield)](https://circleci.com/gh/giantswarm/konfigure)

# konfigure

Konfigure is a CLI to generate configuration primarily for kubernetes resources as config maps and secrets.

Documentation: [intranet](https://intranet.giantswarm.io/docs/dev-and-releng/configuration-management/) | [GitHub](https://github.com/giantswarm/giantswarm/blob/master/content/docs/dev-and-releng/configuration-management/_index.md)

## Generating values locally with the Generalized Configuration System

The Generalized Configuration System is a schema-based, dynamic configuration generation framework.

This kind of configuration is called `Konfiguration` or `kfg` and the schema is called `KonfigurationSchema` or `kfgs`.

Example:

```
SOPS_AGE_KEY_FILE="..." konfigure render \
  --schema schema.yaml \
  --dir giantswarm-configs \
  --variable "installation=${INSTALLATION}" \
  --variable "app=${APP}" \
  --name ${APP}-konfiguration
  --namespace giantswarm
```

The `--raw` flag can be passed to skip wrapping the results into a respective `ConfigMap` and `Secret` manifest. In that
case the `--name` and `--namespace` flags are ignored / not required. This mode can be used to use the resulting
configuration files for any purposes.

### The Konfiguration Schema

A Konfiguration schema is a combination of configuration layers and variables on how to render almost any structure.

A schema consists of the following main parts: `variables`, `layers`, `includes`.

#### Variables

The `variables` list of schema defines names that will be used to locate the required templates and value files across layers.

For example:

```yaml
variables:
  - name: stage
    required: true
  - name: cluster
    required: true
  - name: app
    required: true
```

#### Layers

The `layers` list of a schema defines a list of layers that describe the structure of the configuration.

Each layer consists of the following main parts: `path`, `value files`, `templates` and `patches`.

Both `values files` and `templates` have support for unencrypted and encrypted data, that will respectively can be
wrapped into a resulting kubernetes `ConfigMap` or `Secret` manifests.

For encryption, currently only [SOPS](https://github.com/getsops/sops) is supported with AGE and PGP keys.

Variables can be used under any `path` object for both folder and file names to make dynamic structures to organize
the configurations. They will be substituted under the following patter `<< VARIABLE_NAME >>` (spaces matter!).

For example:

```yaml
layers:
  # ...
  - id: stages
    path:
      directory: stages/<< stage >>
      required: true
    values:
      path:
        directory: ""
      configMap:
        name: config.yaml
        required: true
      secret:
        name: secret.yaml
        required: true
    templates:
      path:
        directory: apps/<< app >>
        required: false
      configMap:
        name: configmap.yaml
        required: false
        values:
          merge:
            strategy: ConfigMapsInLayerOrder
      secret:
        name: secret.yaml
        required: false
        values:
          merge:
            strategy: SameTypeFromCurrentLayer
  # ...
```

The `.id` of a layer must be a unique values across all layers to reference layers in other layers. More on that later.

##### Path

The `.path` of a layer defines the root folder of the layer in the repository. The `.path.directory` field can be used
with variable substitution. The `.path.required` field can make the existence of the layer optional. If so, the layer
is considered empty without raising an error.

##### Values

The `.values` of a layer defines where `value files` are located for the layer. The `.values.path.directory` field
defines a path relative to the root of the layer, empty means the root of the layer. This field can be used
with variable substitution. The `.values.configMap` and `.values.secret` field defines where the value files are
located for the layer. The `name` field of both can be used with variable substitution. The `required` field can
make the existence of these value files optional, considering their absence as an empty file without raising an error.

##### Templates

The `.templates` of a layer defines where the Go templates are located for the layer. The `.templates.path` is very
much the same as it was for value files: relative to layer root, `name` is susceptible for variable substitution. The
`.templates.configMap` and `.templates.secret` both have a `name` field that defines the file name for the given
template and can be used with variable substitution. With the `required` field they can again be made optional,
considering their absence as empty templates. The `values` field of templates defines which value files to use to
render the templates. Under the `merge.strategy` field they support multiple different strategies for convenience:

- `ConfigMapsInLayerOrder`: merge all config map value files in layer order
- `SecretsInLayerOrder`: merge all secret value files in layer order
- `SameTypeInLayerOrder`: merge all the current type value files in layer order (config maps for `.templates.configMap` and secrets for `templates.secret`)
- `SameTypeFromCurrentLayer` only use the value file for the current type from the same layer
- `ConfigMapAndSecretFromCurrentLayer`: merge the config map and the secret value file from the current layer, where the config map value file is the base, on top of that the secret value file is merged
- `ConfigMapsAndSecretsInLayerOrder`: merge all config map value files in layer order, then merge all secret value files in layer order on top of them
- `CustomOrder`: custom order, where layers can be referenced their `id` field. This merge strategy supports options as well where the custom order can be defined.

An example for custom order:

```yaml
layers:
  - id: stages
    # ..
  - id: cluster
    # ...
    values:
      merge:
        strategy: CustomOrder
        options:
          order:
          - layerId: stages
            type: ConfigMap
          - layerId: cluster
            type: Secret
```

##### Patches

The `.patches` of a layer defines a set of [JSON6902 patches](https://datatracker.ietf.org/doc/html/rfc6902), similar
to how `kustomize` supports them.

The purpose of patches is to allow more flexible modifications of the render results up to that point. Since rendered
layers are merged (folded) on top of each other, you are always merging objects and overwriting keys. But you might,
for example, want more control on working with lists. Let's say in a given layer you only want to remove an item or
add a new one to the list from previous layers, instead of overwriting the full list.

An example for declaring patches for a layer:

```yaml
layers:
  # ...
  - id: stages
    # ...
    patches:
      path:
        directory: << app >>
        required: false
      configMap:
        name: config-map-patches.yaml
        required: false
      secret:
        name: secret-patches.yaml
        required: false
```

Let's say you want to add a new init container. You could add this to `<< app >>/config-map-patches.yaml` at the root
of the layer:

```yaml
- op: add
  path: /initContainers/-
  value:
    name: sleep
    image: alpine:latest
    command: [ "sleep", "10" ]
```

The `.path` field of patches works the same way as with layers, for example. It is relative to the root of the layer,
and the `.directory` field is susceptible for variable substitution.

The `.configMap` and `.secret` fields define where the patches are located. In both cases, the `name` field is available
for variable substitution. Setting the `required` field to false will consider the patch empty in case it is missing
without raising an error.

#### Includes

The `includes` list of a schema defines a list of folders that can contain shared templates across all layer templates.

For example:

```yaml
includes:
  - id: include
    function:
      name: include
    path:
      directory: include
      required: true
    extension: .yaml.template
```

The `.id` field of an include is a unique identifier across the schema. The `.function.name` field must be unique as well.
It will be used to generate a custom function that can be used in your Go templates to include the shared templates. The
`.path.directory` is relative to the root of the repository. This field is static and is not used with variable substitution.
The `.extension` field is used to define a generic file extension for all templates. If defined, the extension can be left
off from the function call arguments, otherwise always used to locate the templates. Can be left empty to use any file
extensions and thus the full file name to the template.

The above include can be used in the following way in layer templates:

```gotemplate
instanceTypes:
  {{- include "aws-instance-types" . | nindent 2 }}
```

Where `include` is the `.function.name`, the `aws-instance-types` will be resolved to `include/aws-instance-types.yaml.template`
file from the root of the repository. The `.` is used to pass down the full context of the currently merge value files
for the given layer. It's standard Go templating, a subset of the full context can be passed down as well to render
the shared template and then include the result in the layer template.

#### Examples

See the [examples](./examples) folder.

Giant Swarm schemas are located at: https://github.com/giantswarm/konfiguration-schemas.

### How does rendering a schema work?

These are the steps `konfigure render` takes to render a subtree of a config repository based on the schema and the
passed variable values.

- all paths and file names are resolved based on the variables
- all value files, templates are loaded
- all templates are rendered individually with their value files merged based on the set rules
- all patches are loaded
- rendered templates are folded together and patched
  - we start with an empty base (accumulator)
  - merge the next layer on top of that
  - apply patches for the result merge
  - repeat for the next layer in order, taking the patched result as the base (accumulator)

This last step, for example, assuming we 3 layers: `base`, `stages`, `cluster` for a multistaged environment setup:

- we start with a result as the accumulator for both config maps and secrets
- folding `base` layer
  - we take the rendered `base` layer templates and merge them on top of the accumulator
  - apply the patches on the accumulator from the `base` layer for each type
    - note that patches do not really make sense for the first layer, cos you might as well add the results
      to the templates, but you can do it if you have a use case for it.
- folding `stages` layer
  - we take the rendered `stages` layer templates and merge them on top of the accumulator
  - apply the patches on the accumulator from the `stages` layer for each type
- folding `cluster` layer
    - we take the rendered `cluster` layer templates and merge them on top of the accumulator
    - apply the patches on the accumulator from the `cluster` layer for each type
- we ran out of layers, terminate
  - the accumulator now has the rendered result for both types of configuration

Please note that the layer order, currently, is always following the list order in the `.layers` list of the schema.

## Generating values locally (legacy)

This is the original config generation system of `konfigure` specifically tailored for Giant Swarm management cluster
app configuration with hard-coded structure to generate and conventions to follow.

Example:

```
SOPS_AGE_KEY="..." konfigure generate --raw --app-name ${APP} --installation ${INSTALLATION}
```

This will print values in YAML format ready to use in a helm release.
