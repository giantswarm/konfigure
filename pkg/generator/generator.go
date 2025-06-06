package generator

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"sort"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/getsops/sops/v3/decrypt"
	"github.com/pkg/errors"
	uberconfig "go.uber.org/config"
	yaml3 "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

/*
	- .yaml is a values source
	- .yaml.patch overrides values source
	- .yaml.template is a template
	- .yaml.template.patch overrides template

	Folder structure:
		default/
			config.yaml
			apps/
				aws-operator/
					...
				azure-operator/
					configmap-values.yaml.template
					secret-values.yaml.template
		installations/
			ghost/
				...
			godsmack/
				config.yaml.patch
				secret.yaml
				apps/
					azure-operator/
						configmap-values.yaml.patch
						secret-values.yaml.patch
*/

const (
	appsDefaultPath   = "default/apps/"
	installationsPath = "installations/"
)

type Config struct {
	Fs               Filesystem
	DecryptTraverser DecryptTraverser

	Installation string
	Verbose      bool
}

type Generator struct {
	fs               Filesystem
	decryptTraverser DecryptTraverser

	installation string
	verbose      bool
}

func New(config Config) (*Generator, error) {
	if config.Fs == nil {
		return nil, &InvalidConfigError{message: fmt.Sprintf("%T.Fs must not be empty", config)}
	}
	if config.DecryptTraverser == nil {
		return nil, &InvalidConfigError{message: fmt.Sprintf("%T.DecryptTraverser must not be empty", config)}
	}
	if config.Installation == "" {
		return nil, &InvalidConfigError{message: fmt.Sprintf("%T.Installation must not be empty", config)}
	}

	g := Generator{
		fs:               config.Fs,
		decryptTraverser: config.DecryptTraverser,

		installation: config.Installation,
		verbose:      config.Verbose,
	}

	return &g, nil
}

func (g *Generator) GenerateRawConfig(ctx context.Context, app string) (configmap string, secret string, err error) {
	configmap, secret, err = g.GenerateRawConfigUnsorted(ctx, app)
	if err != nil {
		return "", "", err
	}

	configmap, err = sortYAMLKeys(configmap)
	if err != nil {
		return "", "", err
	}
	secret, err = sortYAMLKeys(secret)
	if err != nil {
		return "", "", err
	}

	return
}

// GenerateRawConfigUnsorted creates final configmap values and secret values
// for helm to use by performing the following operations:
//  1. Get configmap template data and patch it with installation-specific
//     overrides (if available)
//  2. Get global configmap template for the app and render it with template
//     data (result of 1.)
//  3. Get installation-specific configmap patch for the app template (if available)
//  4. Patch global template (result of 2.) with installation-specific (result
//     of 3.) app overrides
//  5. Get installation-specific secret template data and decrypt it
//  6. Merge config and secret values before templating app secret
//  7. Get global secret template for the app (if available) and render it with
//     installation secret template data (result of 5.)
//  8. Get installation-specific secret template patch (if available) and
//     decrypt it
//  9. Patch secret template (result of 6.) with decrypted patch values (result
//     of 7.)
func (g *Generator) GenerateRawConfigUnsorted(ctx context.Context, app string) (configmap string, secret string, err error) {
	// Check if installation folder exists at all. If not, return a descriptive
	// error.
	if _, err := g.fs.ReadDir(installationsPath + g.installation); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", &NotFoundError{
				message: fmt.Sprintf("cannot generate config for installation %s, because \"installations/%s\" does not exist", g.installation, g.installation),
			}
		} else {
			return "", "", err
		}
	}
	// Check if app folder exists at all. If not, return a descriptive error.
	if _, err := g.fs.ReadDir(appsDefaultPath + app); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", &NotFoundError{
				message: fmt.Sprintf("cannot generate config for app %s, because \"default/apps/%s\" does not exist", app, app),
			}
		} else {
			return "", "", err
		}
	}

	// 1.
	configmapContext, err := g.getWithPatchIfExists(
		ctx,
		"default/config.yaml",
		installationsPath+g.installation+"/config.yaml.patch",
	)
	if err != nil {
		return "", "", err
	}
	g.logMessage(ctx, "loaded patched config values")

	// 2.
	g.logMessage(ctx, "rendering configmap-values")
	configmapBase, err := g.getRenderedTemplate(
		ctx,
		appsDefaultPath+app+"/configmap-values.yaml.template",
		configmapContext,
	)
	if err != nil {
		return "", "", err
	}

	g.logMessage(ctx, "rendered configmap-values template")
	// 3.
	var configmapPatch string
	{
		g.logMessage(ctx, "rendering configmap-values patch (if it exists)")
		filepath := installationsPath + g.installation + "/apps/" + app + "/configmap-values.yaml.patch"
		patch, err := g.getRenderedTemplate(ctx, filepath, configmapContext)
		if errors.Is(err, &NotFoundError{}) {
			configmapPatch = ""
		} else if err != nil {
			return "", "", err
		} else {
			configmapPatch = patch
			g.logMessage(ctx, "rendered configmap-values patch")
		}
	}

	// 4.
	configmap, err = applyPatch(
		ctx,
		[]byte(configmapBase),
		[]byte(configmapPatch),
	)
	if err != nil {
		return "", "", err
	}
	if configmapPatch != "" {
		g.logMessage(ctx, "patched configmap-values")
	}

	// 5.
	installationSecretPath := installationsPath + g.installation + "/secret.yaml"
	secretContext, err := g.getWithPatchIfExists(
		ctx,
		installationSecretPath,
		"",
	)
	if err != nil {
		return "", "", err
	}
	g.logMessage(ctx, "loaded installation secret")

	decryptedBytes, err := g.decryptSecret(ctx, []byte(secretContext))
	if err != nil {
		return "", "", errors.Errorf("failed to decrypt secret in %q: %s", installationSecretPath, err)
	}

	secretContext = string(decryptedBytes)
	g.logMessage(ctx, "decrypted installation secret")

	// 6.
	secretContextFinal, err := applyPatch(
		ctx,
		[]byte(configmapContext),
		[]byte(secretContext),
	)
	if err != nil {
		return "", "", err
	}
	g.logMessage(ctx, "merged config and secret values")

	// 7.
	secretTemplate, err := g.getWithPatchIfExists(
		ctx,
		appsDefaultPath+app+"/secret-values.yaml.template",
		"",
	)
	if errors.Is(err, &NotFoundError{}) {
		g.logMessage(ctx, "secret-values template not found, generated configmap")
		return configmap, "", nil
	} else if err != nil {
		return "", "", err
	}
	g.logMessage(ctx, "loaded secret-values template")

	secret, err = g.renderTemplate(ctx, secretTemplate, secretContextFinal)
	if err != nil {
		return "", "", err
	}
	g.logMessage(ctx, "rendered secret-values")

	// 8.
	var secretPatch string
	{
		filepath := installationsPath + g.installation + "/apps/" + app + "/secret-values.yaml.patch"
		patch, err := g.getRenderedTemplate(ctx, filepath, secretContext)
		if errors.Is(err, &NotFoundError{}) {
			secretPatch = ""
		} else if err != nil {
			return "", "", err
		} else {
			g.logMessage(ctx, "loaded secret-values patch")

			decryptedBytes, err := g.decryptSecret(ctx, []byte(patch))
			if err != nil {
				return "", "", errors.Errorf("failed to decrypt app secret in %q: %s", filepath, err)
			}

			secretPatch = string(decryptedBytes)
			g.logMessage(ctx, "decrypted secret-values patch")
		}
	}

	// 9.
	if secretPatch == "" {
		g.logMessage(ctx, "generated configmap and secret")
		return configmap, secret, nil
	}
	secret, err = applyPatch(
		ctx,
		[]byte(secret),
		[]byte(secretPatch),
	)
	if err != nil {
		return "", "", err
	}
	g.logMessage(ctx, "patched secret-values, generated configmap and secret")

	return configmap, secret, nil
}

func sortYAMLKeys(yamlString string) (string, error) {
	if yamlString == "" {
		return yamlString, nil
	}

	n := new(yaml3.Node)
	err := yaml3.Unmarshal([]byte(yamlString), n)
	if err != nil {
		return "", err
	}
	sortYAMLKeysNode(n)
	buf := new(bytes.Buffer)
	enc := yaml3.NewEncoder(buf)
	enc.SetIndent(2)
	err = enc.Encode(n)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Coped (and adapted) from:
// https://github.com/mikefarah/yq/blob/fe12407c936cc4dacf7495a04b5881d14e7b0f47/pkg/yqlib/operator_sort_keys.go#L32
func sortYAMLKeysNode(node *yaml3.Node) {
	if node.Kind == yaml3.DocumentNode || node.Kind == yaml3.SequenceNode {
		for _, n := range node.Content {
			sortYAMLKeysNode(n)
		}
	}
	if node.Kind != yaml3.MappingNode {
		return
	}

	keys := make([]string, len(node.Content)/2)
	keyBucket := map[string]*yaml3.Node{}
	valueBucket := map[string]*yaml3.Node{}
	var contents = node.Content
	for index := 0; index < len(contents); index = index + 2 {
		key := contents[index]
		value := contents[index+1]
		keys[index/2] = key.Value
		keyBucket[key.Value] = key
		valueBucket[key.Value] = value

		sortYAMLKeysNode(value)
	}
	sort.Strings(keys)
	sortedContent := make([]*yaml3.Node, len(node.Content))
	for index := 0; index < len(keys); index = index + 1 {
		keyString := keys[index]
		sortedContent[index*2] = keyBucket[keyString]
		sortedContent[1+(index*2)] = valueBucket[keyString]
	}
	node.Content = sortedContent
}

// GenerateConfig generates ConfigMap and Secret for a given App. The generated
// CM and Secret metadata are configured with the provided value.
func (g *Generator) GenerateConfig(ctx context.Context, app string, meta metav1.ObjectMeta) (*corev1.ConfigMap, *corev1.Secret, error) {
	cm, s, err := g.GenerateRawConfig(ctx, app)
	if err != nil {
		return nil, nil, err
	}

	configmap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: meta,
		Data: map[string]string{
			"configmap-values.yaml": cm,
		},
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: meta,
		Data: map[string][]byte{
			"secret-values.yaml": []byte(s),
		},
	}

	return configmap, secret, nil
}

// getWithPatchIfExists provides contents of filepath overwritten by patch at
// patchFilepath. File at patchFilepath may be non-existent, resulting in pure
// file at filepath being returned.
func (g *Generator) getWithPatchIfExists(ctx context.Context, filepath, patchFilepath string) (string, error) {
	var err error

	var base []byte
	{
		base, err = g.fs.ReadFile(filepath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return "", &NotFoundError{message: fmt.Sprintf("File not found: %q: %s", filepath, err)}
			}

			return "", err
		}
	}

	// patch is not obligatory
	if patchFilepath == "" {
		return string(base), nil
	}

	var patch []byte
	{
		patch, err = g.fs.ReadFile(patchFilepath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return string(base), nil
			}
			return "", err
		}
	}

	result, err := applyPatch(ctx, base, patch)
	if err != nil {
		return "", errors.Errorf("failed to apply patch from %q onto %q: %s", patchFilepath, filepath, err)
	}
	return result, nil
}

func (g *Generator) getRenderedTemplate(ctx context.Context, filepath, templateData string) (string, error) {
	templateBytes, err := g.fs.ReadFile(filepath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", &NotFoundError{message: fmt.Sprintf("Template not found: %q: %s", filepath, err)}
		}

		return "", err
	}

	result, err := g.renderTemplate(ctx, string(templateBytes), templateData)
	if err != nil {
		return "", errors.Errorf("failed to render template from %q: %s", filepath, err)
	}

	return result, nil
}

func applyPatch(ctx context.Context, base, patch []byte) (string, error) {
	patcher, err := uberconfig.NewYAML(
		uberconfig.Permissive(),
		uberconfig.Source(bytes.NewBuffer(base)),
		uberconfig.Source(bytes.NewBuffer(patch)),
	)
	if err != nil {
		return "", err
	}

	value := patcher.Get(uberconfig.Root).Value() // nolint:staticcheck

	output, err := yaml3.Marshal(value)
	if err != nil {
		return "", err
	}

	return string(output), nil
}

func (g *Generator) renderTemplate(ctx context.Context, templateText string, templateData string) (string, error) {
	c := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(templateData), &c)
	if err != nil {
		return "", err
	}

	funcMap := sprig.TxtFuncMap()
	funcMap["include"] = g.include
	funcMap["includeSelf"] = g.includeSelf

	t, err := template.New("main").Funcs(funcMap).Option("missingkey=error").Parse(templateText)
	if err != nil {
		return "", err
	}

	// render final template
	out := bytes.NewBuffer([]byte{})
	err = t.Execute(out, c)
	if err != nil {
		return "", err
	}

	return out.String(), nil
}

func (g *Generator) include(templateName string, templateData interface{}) (string, error) {
	return g.includeFromRoot("include", templateName, templateData)
}

func (g *Generator) includeSelf(templateName string, templateData interface{}) (string, error) {
	return g.includeFromRoot("include-self", templateName, templateData)
}

func (g *Generator) includeFromRoot(root string, templateName string, templateData interface{}) (string, error) {
	templateFilePath := path.Join(root, templateName+".yaml.template")
	contents, err := g.fs.ReadFile(templateFilePath)
	if err != nil {
		return "", err
	}

	t, err := template.New(templateName).Funcs(sprig.TxtFuncMap()).Option("missingkey=error").Parse(string(contents))
	if err != nil {
		return "", errors.Errorf("failed to parse template in file %q: %s", templateFilePath, err)
	}

	out := bytes.NewBuffer([]byte{})
	err = t.Execute(out, templateData)
	if err != nil {
		return "", errors.Errorf("failed to render template from %q: %s", templateFilePath, err)
	}

	return out.String(), nil
}

// TODO: get rid of the Vault decryption eventually. For now supporting both
//
//	options doesn't feel like a bad idea, having both of them opens a door
//	for less stressfull and coordinated migration, allowing us to migrate
//	less-critical apps first, etc.
func (g *Generator) decryptSecret(ctx context.Context, data []byte) ([]byte, error) {
	// The secret.yaml may happen to be empty in which case we do not need
	// to decrypt it at all.
	if len(data) == 0 {
		return data, nil
	}

	// Check if file is SOPS-encrypted
	forSOPS, err := isSOPSEncrypted(ctx, data)
	if err != nil {
		return nil, err
	}

	// If SOPS-encrypted decrypt with SOPS API, otherwise fallback to the
	// Vault method
	var decryptedBytes []byte
	if forSOPS {
		decryptedBytes, err = decrypt.Data(data, "yaml")
	} else {
		decryptedBytes, err = g.decryptTraverser.Traverse(ctx, data)
	}

	if err != nil {
		return nil, err
	}
	return decryptedBytes, nil
}

// Each SOPS-encrypted file carries the `sops` key, that in turn carries metadata
// necessary to decrypt it, hence this key is good for discovering files for SOPS
// decryption.
func isSOPSEncrypted(ctx context.Context, data []byte) (bool, error) {
	values := make(map[interface{}]interface{})

	err := yaml3.Unmarshal([]byte(data), &values)
	if err != nil {
		return false, err
	}

	_, ok := values["sops"]
	return ok, nil
}

func (g *Generator) logMessage(ctx context.Context, format string, params ...interface{}) {
	if g.verbose {
		fmt.Fprintf(os.Stderr, "generator: "+format+"\n", params...)
	}
}
