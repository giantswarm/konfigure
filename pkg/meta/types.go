package meta

type Object interface {
	GetAnnotations() map[string]string
	SetAnnotations(map[string]string)

	GetLabels() map[string]string
	SetLabels(map[string]string)
}
