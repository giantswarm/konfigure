package generator

import (
	"bytes"
	"context"
	"html/template"
	"path"

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
}

type Generator struct {
	fs               Filesystem
	decryptTraverser DecryptTraverser

	installation string
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
	}

	return &g, nil
}

// generateRawConfig creates final configmap values and secret values for helm to
// use by performing the following operations:
// 1. Get configmap template data and patch it with installation-specific
//    overrides (if available)
// 2. Get global configmap template for the app and render it with template
//    data (result of 1.)
// 3. Get installation-specific configmap patch for the app template (if available)
// 4. Patch global template (result of 2.) with installation-specific (result
//    of 3.) app overrides
// 5. Get installation-specific secret template data and decrypt it
// 6. Get global secret template for the app (if available) and render it with
//    installation secret template data (result of 5.)
// 7. Get installation-specific secret template patch (if available) and
//    decrypt it
// 8. Patch secret template (result of 6.) with decrypted patch values (result
//    of 7.)
func (g Generator) generateRawConfig(ctx context.Context, app string) (configmap string, secret string, err error) {
	// 1.
	configmapContext, err := g.getWithPatchIfExists(
		ctx,
		"default/config.yaml",
		"installations/"+g.installation+"/config.yaml.patch",
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
		filepath := "installations/" + g.installation + "/apps/" + app + "/configmap-values.yaml.patch"
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
	secretContext, err := g.getWithPatchIfExists(
		ctx,
		"installations/"+g.installation+"/secret.yaml",
		"",
	)
	if err != nil {
		return "", "", microerror.Mask(err)
	}

	decryptedBytes, err := g.decryptTraverser.Traverse(ctx, []byte(secretContext))
	if err != nil {
		return "", "", microerror.Mask(err)
	}
	secretContext = string(decryptedBytes)

	// 6.
	secretTemplate, err := g.getWithPatchIfExists(
		ctx,
		"default/apps/"+app+"/secret-values.yaml.template",
		"",
	)
	if IsNotFound(err) {
		return configmap, "", nil
	} else if err != nil {
		return "", "", microerror.Mask(err)
	}

	secret, err = g.renderTemplate(ctx, secretTemplate, secretContext)
	if err != nil {
		return "", "", microerror.Mask(err)
	}

	// 7.
	var secretPatch string
	{
		filepath := "installations/" + g.installation + "/apps/" + app + "/secret-values.yaml.patch"
		patch, err := g.getRenderedTemplate(ctx, filepath, secretContext)
		if IsNotFound(err) {
			secretPatch = ""
		} else if err != nil {
			return "", "", microerror.Mask(err)
		} else {
			decryptedBytes, err := g.decryptTraverser.Traverse(ctx, []byte(patch))
			if err != nil {
				return "", "", microerror.Mask(err)
			}
			secretPatch = string(decryptedBytes)
		}
	}

	// 8.
	if secretPatch == "" {
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

	return configmap, secret, nil
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
