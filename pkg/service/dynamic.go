package service

import (
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/giantswarm/konfigure/v2/pkg/renderer"
)

type DynamicServiceConfig struct {
	Log logr.Logger
}

type DynamicService struct {
	log logr.Logger
}

func NewDynamicService(config DynamicServiceConfig) *DynamicService {
	return &DynamicService{log: config.Log}
}

type RenderInput struct {
	// Root directory of the config repository.
	Dir string

	// Path to the schema file.
	Schema string

	// List of variables for the schema in primitive format of 'name=value'.
	Variables []string

	// The name of the generated ConfigMap and Secret.
	Name string

	// The namespace of the generated ConfigMap and Secret.
	Namespace string

	// Additional annotations to be set on the generated ConfigMap and Secret.
	ExtraAnnotations map[string]string

	// Additional labels to be set on the generated ConfigMap and Secret.
	ExtraLabels map[string]string

	// The key to store the rendered data in the generated ConfigMap
	ConfigMapDataKey string

	// The key to store the rendered data in the generated Secret
	SecretDataKey string
}

func (s *DynamicService) Render(in RenderInput) (configmap *corev1.ConfigMap, secret *corev1.Secret, err error) {
	configmapData, secretData, err := s.RenderRaw(in.Dir, in.Schema, in.Variables)
	if err != nil {
		return nil, nil, err
	}

	s.log.Info("Wrapping into ConfigMap and Secret...")

	configmap = renderer.WrapIntoConfigMap(configmapData, in.Name, in.Namespace, in.ExtraAnnotations, in.ExtraLabels, in.ConfigMapDataKey)
	secret = renderer.WrapIntoSecret(secretData, in.Name, in.Namespace, in.ExtraAnnotations, in.ExtraLabels, in.SecretDataKey)

	return configmap, secret, nil
}

func (s *DynamicService) RenderRaw(dir, schema string, primitiveVariables []string) (configmapData string, secretData string, err error) {
	s.log.Info("Loading schema...")

	parsedSchema, err := renderer.LoadSchema(schema)
	if err != nil {
		s.log.Error(err, "Failed to load schema", "file", schema)
		return "", "", err
	}

	s.log.Info("Loading schema variables...")

	parsedSchemaVariables, err := renderer.LoadSchemaVariables(primitiveVariables, parsedSchema.Variables)
	if err != nil {
		s.log.Error(err, "Failed to load schema variables schema", "schema", schema, "variables", primitiveVariables)
		return "", "", err
	}

	s.log.Info("Loading value files...")

	valueFiles, err := renderer.LoadValueFiles(dir, parsedSchema, parsedSchemaVariables)
	if err != nil {
		s.log.Error(err, "Failed to load value files")
		return "", "", err
	}

	s.log.Info("Loading templates...")

	loadedTemplates, err := renderer.LoadTemplates(dir, parsedSchema, parsedSchemaVariables)
	if err != nil {
		s.log.Error(err, "Failed to load templates")
		return "", "", err
	}

	s.log.Info("Rendering templates...")

	renderedTemplates, err := renderer.RenderTemplates(dir, parsedSchema, loadedTemplates, valueFiles)
	if err != nil {
		s.log.Error(err, "Failed to render templates")
		return "", "", err
	}

	s.log.Info("Loading patches...")

	loadedPatches, err := renderer.LoadPatches(dir, parsedSchema, parsedSchemaVariables)
	if err != nil {
		s.log.Error(err, "Failed to load patches")
		return "", "", err
	}

	s.log.Info("Folding and applying patches to rendered templates...")

	configmapData, secretData, err = renderer.FoldAndPatchRenderedTemplates(parsedSchema, renderedTemplates, loadedPatches)
	if err != nil {
		s.log.Error(err, "Failed to fold and apply patches to rendered templates")
		return "", "", err
	}

	return configmapData, secretData, nil
}
