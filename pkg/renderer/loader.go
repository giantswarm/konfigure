package renderer

import (
	"fmt"
	"os"
	"strings"

	"github.com/getsops/sops/v3/decrypt"

	"gopkg.in/yaml.v3"

	"github.com/giantswarm/konfigure/pkg/model"
)

func LoadSchemaVariables(flagValues []string, variables []model.Variable) (SchemaVariables, error) {
	schemaVariables := make(SchemaVariables)

	parsedFlagValues := make(map[string]string)
	for _, flagValue := range flagValues {
		parts := strings.Split(flagValue, "=")
		if len(parts) != 2 {
			return schemaVariables, fmt.Errorf("invalid flag value: %s", flagValue)
		}
		parsedFlagValues[parts[0]] = parts[1]
	}

	for _, variable := range variables {
		parsedValue, found := parsedFlagValues[variable.Name]
		if !found {
			if variable.Required {
				return schemaVariables, fmt.Errorf("variable %s is required", variable.Name)
			} else {
				parsedValue = variable.Default
			}
		}

		schemaVariables[variable.Name] = parsedValue
	}

	return schemaVariables, nil
}

func LoadSchema(path string) (*model.Schema, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var schema model.Schema
	if err := yaml.Unmarshal(content, &schema); err != nil {
		return nil, err
	}

	return &schema, nil
}

func LoadValueFiles(dir string, schema *model.Schema, variables SchemaVariables) (*ValueFiles, error) {
	valueFiles := &ValueFiles{
		ConfigMaps: make(map[string]string),
		Secrets:    make(map[string]string),
	}

	for _, layer := range schema.Layers {
		// Config maps
		if layer.Values.ConfigMap.Name == "" {
			valueFiles.ConfigMaps[layer.Id] = ""
		} else {
			segments := []PathSegment{
				{renderValue(layer.Path.Directory, variables), layer.Path.Required},
				{renderValue(layer.Values.Path.Directory, variables), layer.Values.Path.Required},
				{renderValue(layer.Values.ConfigMap.Name, variables), layer.Values.ConfigMap.Required},
			}

			fmt.Println(segments)

			configMapValueFile, err := loadFileFromPathSegments(dir, segments)

			if err != nil {
				return nil, err
			}

			valueFiles.ConfigMaps[layer.Id] = string(configMapValueFile)
		}

		// Secrets
		if layer.Values.Secret.Name == "" {
			valueFiles.Secrets[layer.Id] = ""
		} else {
			segments := []PathSegment{
				{renderValue(layer.Path.Directory, variables), layer.Path.Required},
				{renderValue(layer.Values.Path.Directory, variables), layer.Values.Path.Required},
				{renderValue(layer.Values.Secret.Name, variables), layer.Values.Secret.Required},
			}

			fmt.Println(segments)

			secretValueFile, err := loadFileFromPathSegments(dir, segments)

			if err != nil {
				return nil, err
			}

			var decryptedSecretValueFile []byte
			if len(strings.TrimSpace(string(secretValueFile))) == 0 {
				decryptedSecretValueFile = make([]byte, 0)
			} else {
				decryptedSecretValueFile, err = decrypt.Data(secretValueFile, "yaml")

				if err != nil {
					return nil, err
				}
			}

			valueFiles.Secrets[layer.Id] = string(decryptedSecretValueFile)
		}
	}

	return valueFiles, nil
}

func loadFileFromPathSegments(dir string, segments []PathSegment) ([]byte, error) {
	path := dir

	for _, segment := range segments {
		path = strings.Join([]string{path, segment.Value}, string(os.PathSeparator))

		_, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				if segment.Required {
					return nil, fmt.Errorf("required path %s does not exist", path)
				}

				return make([]byte, 0), nil
			} else {
				return nil, err
			}
		}
	}

	return os.ReadFile(path)
}

func renderValue(value string, variables SchemaVariables) string {
	result := value

	for name, value := range variables {
		result = strings.ReplaceAll(result, fmt.Sprintf("<< %s >>", name), value)
	}

	return result
}

func loadFile(path string, required bool) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && !required {
			return nil, nil
		}
		return nil, err
	}

	return content, nil
}
