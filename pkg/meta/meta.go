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
	// XObjectHash is set on objects managed by the controllers. It is used
	// to determine whether the managed object needs update.
	XObjectHash
	// XProjectVersion is set on generated ConfigMap and Secret to show what
	// version of konfigure was used to generate them.
	XProjectVersion
}

type LabelType struct {
	// ManagedBy is standard "giantswarm.io/managed-by" label.
	ManagedBy
	// Version is standard "konfigure.giantswarm.io/version" label.
	Version
}
