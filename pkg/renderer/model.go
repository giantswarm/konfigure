package renderer

type SchemaVariables map[string]string

type PathSegment struct {
	Value    string
	Required bool
}

type ValueFiles struct {
	ConfigMaps map[string]string
	Secrets    map[string]string
}

type Templates struct {
	ConfigMaps map[string]string
	Secrets    map[string]string
}

type RenderedTemplates struct {
	ConfigMaps map[string]string
	Secrets    map[string]string
}

type Patches struct {
	ConfigMaps map[string]string
	Secrets    map[string]string
}
