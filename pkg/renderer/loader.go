package renderer

import (
	"fmt"
	"os"
	"strings"

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

func LoadValueFiles(schema *model.Schema, variables SchemaVariables) (*ValueFiles, error) {
	valueFiles := &ValueFiles{}

	for _, layer := range schema.Layers {
		valueFiles.ConfigMaps[layer.Id] = ""
		valueFiles.Secrets[layer.Id] = ""
	}

	return nil, nil
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
