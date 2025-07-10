package renderer

import (
	"bytes"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/giantswarm/konfigure/pkg/utils"

	"github.com/pkg/errors"

	"github.com/Masterminds/sprig/v3"
	uberconfig "go.uber.org/config"
	yaml3 "gopkg.in/yaml.v3"
	"sigs.k8s.io/yaml"

	"github.com/giantswarm/konfigure/pkg/model"

	jsonpatch "github.com/evanphx/json-patch"
)

func RenderTemplates(dir string, schema *model.Schema, templates *Templates, valueFiles *ValueFiles) (*RenderedTemplates, error) {
	renderedTemplates := &RenderedTemplates{
		ConfigMaps: make(map[string]string),
		Secrets:    make(map[string]string),
	}

	extraIncludeFunctions := GenerateIncludeFunctions(dir, schema.Includes)

	for _, layer := range schema.Layers {
		configMapMergedValueFiles, err := MergeValueFileReferences(layer.Templates.ConfigMap.Values, *valueFiles)
		if err != nil {
			return nil, err
		}

		renderedConfigMap, err := RenderTemplate(templates.ConfigMaps[layer.Id], configMapMergedValueFiles, extraIncludeFunctions)
		if err != nil {
			return nil, err
		}

		renderedTemplates.ConfigMaps[layer.Id] = renderedConfigMap

		secretMergedValueFiles, err := MergeValueFileReferences(layer.Templates.Secret.Values, *valueFiles)
		if err != nil {
			return nil, err
		}

		renderedSecret, err := RenderTemplate(templates.Secrets[layer.Id], secretMergedValueFiles, extraIncludeFunctions)
		if err != nil {
			return nil, err
		}

		renderedTemplates.Secrets[layer.Id] = renderedSecret
	}

	return renderedTemplates, nil
}

func MergeValueFileReferences(valueMergeOptions model.ValueMergeOptions, valueFiles ValueFiles) (string, error) {
	var valuesToMerge []string

	for _, valueMergeReference := range valueMergeOptions.Merge {
		switch valueMergeReference.Type {
		case model.ValueMergeReferenceTypeConfigMap:
			valuesToMerge = append(valuesToMerge, valueFiles.ConfigMaps[valueMergeReference.LayerId])
		case model.ValueMergeReferenceTypeSecret:
			valuesToMerge = append(valuesToMerge, valueFiles.Secrets[valueMergeReference.LayerId])
		default:
			return "", errors.Errorf("unknown value merge type %s", valueMergeReference.Type)
		}
	}

	return MergeYamlDocuments(valuesToMerge)
}

// MergeYamlDocuments This is what used to be generator.Generator.applyPatch, but dynamic
func MergeYamlDocuments(valuesToMerge []string) (string, error) {
	if len(valuesToMerge) == 0 {
		return "", nil
	}

	options := []uberconfig.YAMLOption{
		uberconfig.Permissive(),
	}

	for _, valueToMerge := range valuesToMerge {
		options = append(options, uberconfig.Source(bytes.NewBuffer([]byte(valueToMerge))))
	}

	patcher, err := uberconfig.NewYAML(options...)

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

func GenerateIncludeFunctions(dir string, includes []model.Include) template.FuncMap {
	funcMap := sprig.TxtFuncMap()

	for _, include := range includes {
		funcMap[include.Function.Name] = generateIncludeFunction(dir, include)
	}

	return funcMap
}

func generateIncludeFunction(dir string, include model.Include) func(templateName string, templateData interface{}) (string, error) {
	return func(templateName string, templateData interface{}) (string, error) {
		templateFilePath := path.Join(dir, include.Path.Directory, templateName+include.Extension)
		contents, err := os.ReadFile(path.Clean(templateFilePath))
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
}

// RenderTemplate This is what used to be generator.Generator.renderTemplate, but dynamic
func RenderTemplate(text, data string, functions template.FuncMap) (string, error) {
	c := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(data), &c)
	if err != nil {
		return "", err
	}

	t, err := template.New("main").Funcs(functions).Option("missingkey=error").Parse(text)
	if err != nil {
		return "", err
	}

	out := bytes.NewBuffer([]byte{})
	err = t.Execute(out, c)
	if err != nil {
		return "", err
	}

	return out.String(), nil
}

func MergeAndPatchRenderedTemplates(schema *model.Schema, renderedTemplates *RenderedTemplates, patches *Patches) (configmap string, secret string, err error) {
	layerOrder := getLayerOrder(schema)

	for _, layer := range layerOrder {
		configmap, err = MergeAndPatchRenderedTemplate(configmap, renderedTemplates.ConfigMaps[layer], patches.ConfigMaps[layer])
		if err != nil {
			return "", "", err
		}

		secret, err = MergeAndPatchRenderedTemplate(secret, renderedTemplates.Secrets[layer], patches.Secrets[layer])
		if err != nil {
			return "", "", err
		}
	}

	configmap, err = utils.SortYAMLKeys(configmap)
	if err != nil {
		return "", "", err
	}

	secret, err = utils.SortYAMLKeys(secret)
	if err != nil {
		return "", "", err
	}

	return configmap, secret, nil
}

func MergeAndPatchRenderedTemplate(accumulator, renderedTemplate, patches string) (string, error) {
	merged, err := MergeYamlDocuments([]string{accumulator, renderedTemplate})
	if err != nil {
		return "", err
	}

	patched, err := ApplyPatch(merged, patches)
	if err != nil {
		return "", err
	}

	return patched, nil
}

func ApplyPatch(document, patch string) (string, error) {
	json6902Patch, err := trimAndConvertYamlToJson(patch)
	if err != nil {
		return "", err
	}

	decodedPatch, err := jsonpatch.DecodePatch([]byte(json6902Patch))
	if err != nil {
		return "", err
	}

	jsonDocument, err := trimAndConvertYamlToJson(document)
	if err != nil {
		return "", err
	}

	patchedDocument, err := decodedPatch.Apply([]byte(jsonDocument))
	if err != nil {
		return "", err
	}

	result, err := convertJsonToYaml(string(patchedDocument))
	if err != nil {
		return "", err
	}

	return result, nil
}

func trimAndConvertYamlToJson(document string) (string, error) {
	result := strings.TrimSpace(document)

	json, err := yaml.YAMLToJSON([]byte(result))
	if err != nil {
		return "", err
	}

	return string(json), nil
}

func convertJsonToYaml(document string) (string, error) {
	yamlDocument, err := yaml.JSONToYAML([]byte(document))
	if err != nil {
		return "", err
	}

	return string(yamlDocument), nil
}

func getLayerOrder(schema *model.Schema) []string {
	var result []string

	for _, layer := range schema.Layers {
		result = append(result, layer.Id)
	}

	return result
}
