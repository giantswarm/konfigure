package renderer

import (
	"strings"

	sopsV3Decrypt "github.com/getsops/sops/v3/decrypt"

	"github.com/giantswarm/konfigure/pkg/decrypt"
	"github.com/giantswarm/konfigure/pkg/model"
)

func RenderLayerTemplates(dir string, schema *model.Schema, variables SchemaVariables, valueFiles *ValueFiles) (*RenderedTemplates, error) {
	renderedTemplates := &RenderedTemplates{
		ConfigMaps: make(map[string]string),
		Secrets:    make(map[string]string),
	}

	for _, layer := range schema.Layers {
		if layer.Templates.ConfigMap.Name == "" {
			renderedTemplates.ConfigMaps[layer.Id] = ""
		} else {
			segments := []PathSegment{
				{renderValue(layer.Path.Directory, variables), layer.Path.Required},
				{renderValue(layer.Templates.Path.Directory, variables), layer.Templates.Path.Required},
				{renderValue(layer.Templates.ConfigMap.Name, variables), layer.Templates.ConfigMap.Required},
			}

			configMapTemplate, err := loadFileFromPathSegments(dir, segments)
			if err != nil {
				return nil, err
			}

			renderedTemplates.ConfigMaps[layer.Id] = string(configMapTemplate)
		}

		if layer.Templates.Secret.Name == "" {
			renderedTemplates.Secrets[layer.Id] = ""
		} else {
			segments := []PathSegment{
				{renderValue(layer.Path.Directory, variables), layer.Path.Required},
				{renderValue(layer.Templates.Path.Directory, variables), layer.Templates.Path.Required},
				{renderValue(layer.Templates.Secret.Name, variables), layer.Templates.Secret.Required},
			}

			secretTemplate, err := loadFileFromPathSegments(dir, segments)
			if err != nil {
				return nil, err
			}

			var decryptedSecretTemplate []byte
			if len(strings.TrimSpace(string(secretTemplate))) == 0 {
				decryptedSecretTemplate = make([]byte, 0)
			} else {
				isSopsEncrypted := decrypt.IsSOPSEncrypted(secretTemplate)

				if isSopsEncrypted {
					decryptedSecretTemplate, err = sopsV3Decrypt.Data(secretTemplate, "yaml")
					if err != nil {
						return nil, err
					}
				} else {
					decryptedSecretTemplate = secretTemplate
				}

			}

			renderedTemplates.Secrets[layer.Id] = string(decryptedSecretTemplate)
		}
	}

	return renderedTemplates, nil
}
