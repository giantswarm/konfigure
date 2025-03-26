package generic

type Schema struct {
	Spec SchemaSpec `yaml:"spec"`
}

type SchemaSpec struct {
	Layers []Layer `yaml:"layers"`
}

type Layer struct {
	Name string `yaml:"name"`
}
