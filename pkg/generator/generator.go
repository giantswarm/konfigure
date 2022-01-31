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
	"github.com/giantswarm/microerror"
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
		return nil, microerror.Maskf(invalidConfigError, "%T.Fs must not be empty", config)
	}
	if config.DecryptTraverser == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.DecryptTraverser must not be empty", config)
	}
	if config.Installation == "" {
		return nil, microerror.Maskf(invalidConfigError, "%T.Installation must not be empty", config)
	}

	g := Generator{
		fs:               config.Fs,
		decryptTraverser: config.DecryptTraverser,

		installation: config.Installation,
		verbose:      config.Verbose,
	}

	return &g, nil
}

func (g Generator) generateRawConfig(ctx context.Context, app string) (configmap string, secret string, err error) {
	configmap, secret, err = g.generateRawConfigUnsorted(ctx, app)
	if err != nil {
		return "", "", microerror.Mask(err)
	}

	configmap, err = sortYAMLKeys(configmap)
	if err != nil {
		return "", "", microerror.Mask(err)
	}
	secret, err = sortYAMLKeys(secret)
	if err != nil {
		return "", "", microerror.Mask(err)
	}

	return
}

// generateRawConfigUnsorted creates final configmap values and secret values
// for helm to use by performing the following operations:
// 1. Get configmap template data and patch it with installation-specific
//    overrides (if available)
// 2. Get global configmap template for the app and render it with template
//    data (result of 1.)
// 3. Get installation-specific configmap patch for the app template (if available)
// 4. Patch global template (result of 2.) with installation-specific (result
//    of 3.) app overrides
// 5. Get installation-specific secret template data and decrypt it
// 6. Merge config and secret values before templating app secret
// 7. Get global secret template for the app (if available) and render it with
//    installation secret template data (result of 5.)
// 8. Get installation-specific secret template patch (if available) and
//    decrypt it
// 9. Patch secret template (result of 6.) with decrypted patch values (result
//    of 7.)
func (g Generator) generateRawConfigUnsorted(ctx context.Context, app string) (configmap string, secret string, err error) {
	// 1.
	configmapContext, err := g.getWithPatchIfExists(
		ctx,
		"default/config.yaml",
		"installations/"+g.installation+"/config.yaml.patch",
	)
	if err != nil {
		return "", "", microerror.Mask(err)
	}
	g.logMessage(ctx, "loaded patched config values")

	// 2.
	g.logMessage(ctx, "rendering configmap-values")
	configmapBase, err := g.getRenderedTemplate(
		ctx,
		"default/apps/"+app+"/configmap-values.yaml.template",
		configmapContext,
	)
	if err != nil {
		return "", "", microerror.Mask(err)
	}
	g.logMessage(ctx, "rendered configmap-values template")
	// 3.
	var configmapPatch string
	{
		g.logMessage(ctx, "rendering configmap-values patch (if it exists)")
		filepath := "installations/" + g.installation + "/apps/" + app + "/configmap-values.yaml.patch"
		patch, err := g.getRenderedTemplate(ctx, filepath, configmapContext)
		if IsNotFound(err) {
			configmapPatch = ""
		} else if err != nil {
			return "", "", microerror.Mask(err)
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
		return "", "", microerror.Mask(err)
	}
	if configmapPatch != "" {
		g.logMessage(ctx, "patched configmap-values")
	}

	// 5.
	secretContext, err := g.getWithPatchIfExists(
		ctx,
		"installations/"+g.installation+"/secret.yaml",
		"",
	)
	if err != nil {
		return "", "", microerror.Mask(err)
	}
	g.logMessage(ctx, "loaded installation secret")

	decryptedBytes, err := g.decryptTraverser.Traverse(ctx, []byte(secretContext))
	if err != nil {
		return "", "", microerror.Mask(err)
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
		return "", "", microerror.Mask(err)
	}
	g.logMessage(ctx, "merged config and secret values")

	// 7.
	secretTemplate, err := g.getWithPatchIfExists(
		ctx,
		"default/apps/"+app+"/secret-values.yaml.template",
		"",
	)
	if IsNotFound(err) {
		g.logMessage(ctx, "secret-values template not found, generated configmap")
		return configmap, "", nil
	} else if err != nil {
		return "", "", microerror.Mask(err)
	}
	g.logMessage(ctx, "loaded secret-values template")

	secret, err = g.renderTemplate(ctx, secretTemplate, secretContextFinal)
	if err != nil {
		return "", "", microerror.Mask(err)
	}
	g.logMessage(ctx, "rendered secret-values")

	// 8.
	var secretPatch string
	{
		filepath := "installations/" + g.installation + "/apps/" + app + "/secret-values.yaml.patch"
		patch, err := g.getRenderedTemplate(ctx, filepath, secretContext)
		if IsNotFound(err) {
			secretPatch = ""
		} else if err != nil {
			return "", "", microerror.Mask(err)
		} else {
			g.logMessage(ctx, "loaded secret-values patch")
			decryptedBytes, err := g.decryptTraverser.Traverse(ctx, []byte(patch))
			if err != nil {
				return "", "", microerror.Mask(err)
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
		return "", "", microerror.Mask(err)
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
		return "", microerror.Mask(err)
	}
	sortYAMLKeysNode(n)
	buf := new(bytes.Buffer)
	enc := yaml3.NewEncoder(buf)
	enc.SetIndent(2)
	err = enc.Encode(n)
	if err != nil {
		return "", microerror.Mask(err)
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
func (g Generator) GenerateConfig(ctx context.Context, app string, meta metav1.ObjectMeta) (*corev1.ConfigMap, *corev1.Secret, error) {
	cm, s, err := g.generateRawConfig(ctx, app)
	if err != nil {
		return nil, nil, microerror.Mask(err)
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
func (g Generator) getWithPatchIfExists(ctx context.Context, filepath, patchFilepath string) (string, error) {
	var err error

	var base []byte
	{
		base, err = g.fs.ReadFile(filepath)
		if err != nil {
			return "", microerror.Mask(err)
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
			if IsNotFound(err) {
				return string(base), nil
			}
			return "", microerror.Mask(err)
		}
	}

	result, err := applyPatch(ctx, base, patch)
	if err != nil {
		return "", microerror.Mask(err)
	}
	return result, nil
}

func (g Generator) getRenderedTemplate(ctx context.Context, filepath, templateData string) (string, error) {
	templateBytes, err := g.fs.ReadFile(filepath)
	if err != nil {
		return "", microerror.Mask(err)
	}

	result, err := g.renderTemplate(ctx, string(templateBytes), templateData)
	if err != nil {
		return "", microerror.Mask(err)
	}

	return result, nil
}

func applyPatch(ctx context.Context, base, patch []byte) (string, error) {
	patcher, err := uberconfig.NewYAML(
		uberconfig.Source(bytes.NewBuffer(base)),
		uberconfig.Source(bytes.NewBuffer(patch)),
	)
	if err != nil {
		return "", microerror.Mask(err)
	}

	value := patcher.Get(uberconfig.Root).Value() // nolint:staticcheck
	output, err := yaml3.Marshal(value)
	if err != nil {
		return "", microerror.Mask(err)
	}

	return string(output), nil
}

func (g Generator) renderTemplate(ctx context.Context, templateText string, templateData string) (string, error) {
	c := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(templateData), &c)
	if err != nil {
		return "", microerror.Mask(err)
	}

	funcMap := sprig.TxtFuncMap()
	funcMap["include"] = g.include

	t, err := template.New("main").Funcs(funcMap).Option("missingkey=error").Parse(templateText)
	if err != nil {
		return "", microerror.Mask(err)
	}

	// render final template
	out := bytes.NewBuffer([]byte{})
	err = t.Execute(out, c)
	if err != nil {
		return "", microerror.Mask(err)
	}

	return out.String(), nil
}

func (g Generator) include(templateName string, templateData interface{}) (string, error) {
	contents, err := g.fs.ReadFile(path.Join("include", templateName+".yaml.template"))
	if err != nil {
		return "", microerror.Mask(err)
	}

	t, err := template.New(templateName).Funcs(sprig.TxtFuncMap()).Option("missingkey=error").Parse(string(contents))
	if err != nil {
		return "", microerror.Mask(err)
	}

	out := bytes.NewBuffer([]byte{})
	err = t.Execute(out, templateData)
	if err != nil {
		return "", microerror.Mask(err)
	}

	return out.String(), nil
}

func (g Generator) logMessage(ctx context.Context, format string, params ...interface{}) {
	if g.verbose {
		fmt.Fprintf(os.Stderr, "generator: "+format+"\n", params...)
	}
}
