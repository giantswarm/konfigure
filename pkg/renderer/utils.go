package renderer

import (
	"fmt"
	"strings"
)

func renderValue(value string, variables SchemaVariables) string {
	result := value

	for name, value := range variables {
		result = strings.ReplaceAll(result, fmt.Sprintf("<< %s >>", name), value)
	}

	return result
}
