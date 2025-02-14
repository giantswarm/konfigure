package meta

import (
	"os"
	"os/user"

	"github.com/giantswarm/k8smetadata/pkg/annotation"

	"github.com/giantswarm/konfigure/pkg/project"
)

var (
	configVersionAnnotation   = annotation.ConfigVersion
	xAppInfoAnnotation        = project.Name() + ".x-giantswarm.io/app-info"
	xCreatorAnnotation        = project.Name() + ".x-giantswarm.io/creator"
	xInstallationAnnotation   = project.Name() + ".x-giantswarm.io/installation"
	xObjectHashAnnotation     = project.Name() + ".x-giantswarm.io/object-hash"
	xProjectVersionAnnotation = project.Name() + ".x-giantswarm.io/project-version"
)

type ConfigVersion struct{}

func (ConfigVersion) Key() string { return configVersionAnnotation }

type XAppInfo struct{}

func (XAppInfo) Key() string { return xAppInfoAnnotation }

func (XAppInfo) Val(catalog, app, version string) string {
	return catalog + "/" + app + "@" + version
}

type XCreator struct{}

func (XCreator) Key() string { return xCreatorAnnotation }

func (XCreator) Default() string {
	u, err := user.Current()
	if err == nil {
		return u.Username
	}

	if os.Getenv("USER") != "" {
		return os.Getenv("USER")
	}

	return os.Getenv("USERNAME") // Windows
}

type XInstallation struct{}

func (XInstallation) Key() string { return xInstallationAnnotation }

type XObjectHash struct{}

func (XObjectHash) Key() string { return xObjectHashAnnotation }

type XProjectVersion struct{}

func (XProjectVersion) Key() string { return xProjectVersionAnnotation }

func (XProjectVersion) Val(unique bool) string {
	if unique {
		return "0.0.0"
	}

	return project.Version()
}
