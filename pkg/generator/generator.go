package generator

import (
	"bytes"
	"context"
	"html/template"
	"path"
	"regexp"
	"strings"

	"github.com/Masterminds/sprig"
	"github.com/ghodss/yaml"
	"github.com/giantswarm/microerror"
	pathmodifier "github.com/giantswarm/valuemodifier/path"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
				secrets.yaml
				apps/
					azure-operator/
						configmap-values.yaml.patch.template
*/

var (
	multipleDashPattern     = regexp.MustCompile("-{2,}")
	invalidCharacterPattern = regexp.MustCompile("[^a-z0-9]+")
)

type Config struct {
	Fs               Filesystem
	DecryptTraverser DecryptTraverser
}

type Generator struct {
	fs               Filesystem
	decryptTraverser DecryptTraverser
}

func New(config *Config) (*Generator, error) {
	if config.Fs == nil {
		return nil, microerror.Maskf(invalidConfigError, "%T.Fs must not be empty", config)
	}
	g := Generator{
		fs:               config.Fs,
		decryptTraverser: config.DecryptTraverser,
	}

	return &g, nil
}

// GenerateConfig creates final configmap values and secret values for helm to
// use by performing the following operations:
// 1. Get configmap template data and patch it with installation-specific
//    overrides (if available)
// 2. Get global configmap template for the app and render it with template
//    data (result of 1.)
// 3. Get installation-specific configmap template for the app patch and render
//    it with template data (result of 1.)
// 4. Patch global template (result of 2.) with installation-specific (result
//    of 3.) app overrides (if available)
// 5. Get global secrets template data
// 6. Decrypt global secrets if DecryptTraverser has been provided.
// 7. Get installation-specific secrets template for the app (if available) and
//    render it with installation secrets template data (result of 5.)
func (g Generator) GenerateRawConfig(ctx context.Context, installation, app string) (configmap string, secrets string, err error) {
	// 1.
	configmapContext, err := g.getWithPatchIfExists(
		ctx,
		"default/config.yaml",
		"installations/"+installation+"/config.yaml.patch",
	)
	if err != nil {
		return "", "", microerror.Mask(err)
	}

	// 2.
	configmapBase, err := g.getRenderedTemplate(
		ctx,
		"default/apps/"+app+"/configmap-values.yaml.template",
		configmapContext,
	)
	if err != nil {
		return "", "", microerror.Mask(err)
	}

	// 3.
	var configmapPatch string
	{
		filepath := "installations/" + installation + "/apps/" + app + "/configmap-values.yaml.patch.template"
		patch, err := g.getRenderedTemplate(ctx, filepath, configmapContext)
		if IsNotFound(err) {
			configmapPatch = ""
		} else if err != nil {
			return "", "", microerror.Mask(err)
		} else {
			configmapPatch = patch
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

	// 5.
	secretsContext, err := g.getWithPatchIfExists(
		ctx,
		"installations/"+installation+"/secrets.yaml",
		"",
	)
	if err != nil {
		return "", "", microerror.Mask(err)
	}

	// 6.
	if g.decryptTraverser != nil {
		decryptedBytes, err := g.decryptTraverser.Traverse(ctx, []byte(secretsContext))
		if err != nil {
			return "", "", microerror.Mask(err)
		}
		secretsContext = string(decryptedBytes)
	}

	// 7.
	secretsTemplate, err := g.getWithPatchIfExists(
		ctx,
		"default/apps/"+app+"/secret-values.yaml.template",
		"",
	)
	if IsNotFound(err) {
		return configmap, "", nil
	} else if err != nil {
		return "", "", microerror.Mask(err)
	}

	secrets, err = g.renderTemplate(ctx, secretsTemplate, secretsContext)
	if err != nil {
		return "", "", microerror.Mask(err)
	}

	return configmap, secrets, nil
}

func (g Generator) GenerateConfig(ctx context.Context, installation, app, ref string) (configmap *corev1.ConfigMap, secrets *corev1.Secret, err error) {
	cm, s, err := g.GenerateRawConfig(ctx, installation, app)
	if err != nil {
		return nil, nil, microerror.Mask(err)
	}

	name := generateResourceName(app, ref)
	meta := metav1.ObjectMeta{
		Name:      name,
		Namespace: "giantswarm",
	}

	configmap = &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: meta,
		Data: map[string]string{
			"configmap-values.yaml": cm,
		},
	}

	secrets = &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: meta,
		Data: map[string][]byte{
			"secret-values.yaml": []byte(s),
		},
	}

	return configmap, secrets, nil
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
	var basePathSvc *pathmodifier.Service
	{
		c := pathmodifier.DefaultConfig()
		c.InputBytes = base
		svc, err := pathmodifier.New(c)
		if err != nil {
			return "", microerror.Mask(err)
		}
		basePathSvc = svc
	}

	var patchPathSvc *pathmodifier.Service
	{
		c := pathmodifier.DefaultConfig()
		c.InputBytes = patch
		svc, err := pathmodifier.New(c)
		if err != nil {
			return "", microerror.Mask(err)
		}
		patchPathSvc = svc
	}

	patchedPaths, err := patchPathSvc.All()
	if err != nil {
		return "", microerror.Mask(err)
	}

	for _, p := range patchedPaths {
		value, err := patchPathSvc.Get(p)
		if err != nil {
			return "", microerror.Mask(err)
		}

		err = basePathSvc.Set(p, value)
		if err != nil {
			return "", microerror.Mask(err)
		}
	}

	outputBytes, err := basePathSvc.OutputBytes()
	if err != nil {
		return "", microerror.Mask(err)
	}

	return string(outputBytes), nil
}

func (g Generator) renderTemplate(ctx context.Context, templateText string, templateData string) (string, error) {
	c := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(templateData), &c)
	if err != nil {
		return "", microerror.Mask(err)
	}

	funcMap := sprig.FuncMap()
	funcMap["include"] = g.include

	t, err := template.New("main").Funcs(funcMap).Parse(templateText)
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

	t, err := template.New(templateName).Funcs(sprig.FuncMap()).Parse(string(contents))
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

func generateResourceName(elements ...string) string {
	name := strings.Join(elements, "-")
	name = string(invalidCharacterPattern.ReplaceAll([]byte(name), []byte("-")))
	name = string(multipleDashPattern.ReplaceAll([]byte(name), []byte("-")))
	name = strings.ToLower(strings.Trim(name, "-"))
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}
