package renderer

import (
	"bytes"
	"fmt"
	"strings"

	uberconfig "go.uber.org/config"
	yaml3 "gopkg.in/yaml.v3"
)

func RenderValue(value string, variables SchemaVariables) string {
	result := value

	for name, value := range variables {
		result = strings.ReplaceAll(result, fmt.Sprintf("<< %s >>", name), value)
	}

	return result
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
