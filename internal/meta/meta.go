package meta

var (
	Annotation AnnotationType
	Label      LabelType
)

type AnnotationType struct {
	// ConfigVersion is standard "config.giantswarm.io/version" annotation.
	ConfigVersion
	// XAppInfo is set on generated ConfigMap and Secret to show what App
	// they were generated for.
	XAppInfo
	// XCreator is used in the CLI mode. The value is OS username. It is
	// set on generated ConfigMap and Secret.
	XCreator
	// XInstallation s set on generated ConfigMap and Secret to show what
	// installation they were generated for.
	XInstallation
	// XProjectVersion is set on generated ConfigMap and Secret to show what
	// version of config-controller was used to generate them.
	XProjectVersion
}

type LabelType struct {
	// ManagedBy is standard "giantswarm.io/managed-by" label.
	ManagedBy
}
