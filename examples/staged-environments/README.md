# Staged environments

This example demonstrates a schema and repository structure for a staged environment setup.

## Study the schema

We have three layers: `base`, `stages` and `cluster`.

## Examples

All example commands are to be executed at the root of the `examples/staged-environments` folder.

Secret value files are encrypted with the `example.age` key.

### Configuration for konfiguration-1 on cluster-1 at the dev stage

```shell
SOPS_AGE_KEY_FILE=example.age konfigure render \
  --schema schema.yaml \
  --dir . \
  --variable "stage=dev" \
  --variable="cluster=cluster-1" \
  --variable="konfiguration=konfiguration-1" \
  --raw
```

This will give us:

```yaml
---
a:
  b:
    j: 12
    k: dummy
  e: 42
  f:
    g:
      - d
      - e
  h:
    - 1st
    - 2nd
    - added by patch
  x: 623
  xx: 295
cluster:
  name: cluster-1
  provider: capa
foo: baz
stage:
  name: dev
  url: dev.example.com

---
certificate:
  value: dummy
```

The first document is the config map result. The second document is the secret result.

Note these in the result:

- example of removing objects:
  - the original `.a.b` object was removed by the JSON6902 patch from [this patch file](1-stages/konfigurations/konfiguration-1/config-map-patches-dev.yaml).
  - the [cluster layer config map template](2-clusters/cluster-1/konfiguration-1/config-map-template.yaml) adds it back
    with its own values. So instead of merging `.a.b` object fields with the previous layers, it gets completely
    recreated in case of this stage on the following layers, `cluster` in this case.
- example of manipulating lists:
  - the `.a.h` list has an item `added by patch`, that is added by a JSON6902 patch from [this patch file](1-stages/konfigurations/konfiguration-1/config-map-patches-dev.yaml).
- example of using different secrets based on a variable
 - the `stages` layer [secret template for konfiguration-1](1-stages/konfigurations/konfiguration-1/secret-template.yaml)
   defines use of certificates
   - by using the power of variable substitution in the schema paths, we have multiple, stage-specific value files.
   - the [dev stage secret value file](1-stages/stages/dev/secret.yaml) simply defines the value as `dummy`.
   - you can decrypt and observe the value file with:
     ```shell
     SOPS_AGE_KEY_FILE=example.age sops --decrypt --input-type yaml --output-type yaml 1-stages/stages/dev/secret.yaml
     ```

### Configuration for konfiguration-1 on cluster-1 at the production stage

Now let's run the same example but for the production stage.

```shell
SOPS_AGE_KEY_FILE=example.age konfigure render \
  --schema schema.yaml \
  --dir . \
  --variable "stage=production" \
  --variable="cluster=cluster-1" \
  --variable="konfiguration=konfiguration-1" \
  --raw
```

The result will be:

```yaml
---
a:
  b:
    c: 12
    d: overwritten
    j: 12
    k: dummy
  e: 42
  f:
    g:
      - d
      - e
  h:
    - 1st
    - 2nd
  x: 623
  xx: 295
cluster:
  name: cluster-1
  provider: capa
foo: baz
stage:
  name: production
  url: production.example.com

---
certificate:
  value: top-secret
```

Again, the first document is the config map result. The second document is the secret result.

Note these in the result:

- in the result secret render, the `.certificate.value` is now taken from the
  [production stage secret value file](1-stages/stages/production/secret.yaml).
  - you can decrypt and observe the value file with:
    ```shell
    SOPS_AGE_KEY_FILE=example.age sops --decrypt --input-type yaml --output-type yaml 1-stages/stages/production/secret.yaml
    ```
- in the config map template render, the `.a.b` object is not removed for the production stage by a patch,
  cos there is none defined, so it is considered empty
  - instead, since objects get merged recursively, the `.a.b.c` field is inherited from the base layer template
  - the `.a.b.d` field gets overwritten by the cluster layer
  - the `.a.b.j` and `.a.b.k` fields are merged into the `.a.b` object fields by the cluster layer
- the `.a.f.g` list is overwritten, as the merge library is not merging lists as object, but overwrites them like it does
  with primitive types.
  > ℹ️ If you need more fine-grained manipulation of lists, you can use JSON6902 patches.
