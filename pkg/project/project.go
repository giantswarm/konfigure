package project

var (
	description = "The config-controller does something."
	gitSHA      = "n/a"
	name        = "config-controller"
	source      = "https://github.com/giantswarm/config-controller"
	version     = "0.2.3"
)

// AppControlPlaneVersion is always 0.0.0 for control plane app CRs. These CRs
// are processed by config-controller-unique which always runs the latest
// version.
func AppControlPlaneVersion() string {
	return "0.0.0"
}

func Description() string {
	return description
}

func GitSHA() string {
	return gitSHA
}

func Name() string {
	return name
}

func Source() string {
	return source
}

func Version() string {
	return version
}
