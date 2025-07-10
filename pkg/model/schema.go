package model

type Schema struct {
	Variables []Variable `yaml:"variables"`
	Layers    []Layer    `yaml:"layers"`
	Includes  []Include  `yaml:"includes"`
}

type Variable struct {
	Name     string `yaml:"name"`
	Required bool   `yaml:"required"`
	Default  string `yaml:"default"`
}

type Layer struct {
	Id        string    `yaml:"id"`
	Path      Path      `yaml:"path"`
	Values    Values    `yaml:"values"`
	Templates Templates `yaml:"templates"`
	Patches   Patches   `yaml:"patches"`
}

type Values struct {
	Path      Path  `yaml:"path"`
	ConfigMap Value `yaml:"configMap"`
	Secret    Value `yaml:"secret"`
}

type Value struct {
	Name     string `yaml:"name"`
	Required bool   `yaml:"required"`
}

type Templates struct {
	Path      Path     `yaml:"path"`
	ConfigMap Template `yaml:"configMap"`
	Secret    Template `yaml:"secret"`
}

type Template struct {
	Name     string           `yaml:"name"`
	Required bool             `yaml:"required"`
	Values   ValueFileOptions `yaml:"values"`
}

type ValueFileOptions struct {
	Merge ValueFileMergeOptions `yaml:"merge"`
}

const (
	ValueFileMergeStrategyCustomOrder                      = "CustomOrder"
	ValueFileMergeStrategyConfigMapsInLayerOrder           = "ConfigMapsInLayerOrder"
	ValueFileMergeStrategySecretsInLayerOrder              = "SecretsInLayerOrder"
	ValueFileMergeStrategyConfigMapsAndSecretsInLayerOrder = "ConfigMapsAndSecretsInLayerOrder" // nolint:gosec
)

type RawMessage struct {
	unmarshal func(interface{}) error
}

func (msg *RawMessage) UnmarshalYAML(unmarshal func(interface{}) error) error {
	msg.unmarshal = unmarshal
	return nil
}

func (msg *RawMessage) Unmarshal(v interface{}) error {
	return msg.unmarshal(v)
}

type ValueFileMergeOptions struct {
	Strategy string     `yaml:"strategy"`
	Options  RawMessage `yaml:"options"`
}

type CustomOrderValueMergeStrategyOptions struct {
	Order []ValueMergeReference `yaml:"order"`
}

type ValueMergeReferenceType string

const (
	ValueMergeReferenceTypeConfigMap ValueMergeReferenceType = "configMap"
	ValueMergeReferenceTypeSecret    ValueMergeReferenceType = "secret"
)

type ValueMergeReference struct {
	LayerId string                  `yaml:"layerId"`
	Type    ValueMergeReferenceType `yaml:"type"`
}

type Patches struct {
	Path      Path         `yaml:"path"`
	ConfigMap PatchOptions `yaml:"configMap"`
	Secret    PatchOptions `yaml:"secret"`
}

type PatchOptions struct {
	Name     string `yaml:"name"`
	Required bool   `yaml:"required"`
}

type Include struct {
	Id        string          `yaml:"id"`
	Function  IncludeFunction `yaml:"function"`
	Path      Path            `yaml:"path"`
	Extension string          `yaml:"extension"`
}

type IncludeFunction struct {
	Name string `yaml:"name"`
}

type Path struct {
	Directory string `yaml:"directory"`
	Required  bool   `yaml:"required"`
}
