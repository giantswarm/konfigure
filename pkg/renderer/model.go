package renderer

type SchemaVariables map[string]string

type ValueFiles struct {
	ConfigMaps map[string]string
	Secrets    map[string]string
}
