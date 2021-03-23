package lint

import (
	"fmt"
	"sort"

	"github.com/giantswarm/microerror"

	"github.com/giantswarm/config-controller/pkg/generator"
	"github.com/giantswarm/config-controller/pkg/github"
)

type discovery struct {
	Config        *configFile
	ConfigPatches []*configFile
	Secrets       []*configFile

	Templates             []*templateFile
	SecretTemplates       []*templateFile
	TemplatePatches       []*templateFile
	SecretTemplatePatches []*templateFile
	Include               []*templateFile

	Installations []string
	Apps          []string

	AppsPerInstallation                  map[string][]string
	ConfigPatchesPerInstallation         map[string]*configFile
	SecretsPerInstallation               map[string]*configFile
	TemplatesPerApp                      map[string]*templateFile
	SecretTemplatesPerApp                map[string]*templateFile
	TemplatePatchesPerInstallation       map[string][]*templateFile
	SecretTemplatePatchesPerInstallation map[string][]*templateFile
}

func (d discovery) getAppTemplatePatch(installation, app string) (*templateFile, bool) {
	templatePatches, ok := d.TemplatePatchesPerInstallation[installation]
	if !ok {
		return nil, false
	}
	for _, patch := range templatePatches {
		if patch.app == app {
			return patch, true
		}
	}
	return nil, false
}

func (d discovery) getAppSecretTemplatePatch(installation, app string) (*templateFile, bool) {
	templatePatches, ok := d.SecretTemplatePatchesPerInstallation[installation]
	if !ok {
		return nil, false
	}
	for _, patch := range templatePatches {
		if patch.app == app {
			return patch, true
		}
	}
	return nil, false
}

func newDiscovery(fs generator.Filesystem) (*discovery, error) {
	d := &discovery{
		ConfigPatches: []*configFile{},
		Secrets:       []*configFile{},

		Templates:             []*templateFile{},
		SecretTemplates:       []*templateFile{},
		TemplatePatches:       []*templateFile{},
		SecretTemplatePatches: []*templateFile{},

		Installations: []string{},
		Apps:          []string{},

		AppsPerInstallation:                  map[string][]string{},
		ConfigPatchesPerInstallation:         map[string]*configFile{},
		SecretsPerInstallation:               map[string]*configFile{},
		TemplatesPerApp:                      map[string]*templateFile{},
		SecretTemplatesPerApp:                map[string]*templateFile{},
		TemplatePatchesPerInstallation:       map[string][]*templateFile{},
		SecretTemplatePatchesPerInstallation: map[string][]*templateFile{},
	}

	// collect config.yaml
	{
		filepath := "default/config.yaml"
		body, err := fs.ReadFile(filepath)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		d.Config, err = newConfigFile(filepath, body)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	uniqueInstallations := map[string]bool{}
	uniqueApps := map[string]bool{}

	installationDirs, err := fs.ReadDir("installations/")
	if err != nil {
		return nil, microerror.Mask(err)
	}

	// collect installations/*/config.yaml.patch & installations/*/secret.yaml files
	for _, inst := range installationDirs {
		if !inst.IsDir() {
			continue
		}
		uniqueInstallations[inst.Name()] = true
		filepath := fmt.Sprintf("installations/%s/config.yaml.patch", inst.Name())
		body, err := fs.ReadFile(filepath)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		patch, err := newConfigFile(filepath, body)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		d.ConfigPatches = append(d.ConfigPatches, patch)
		d.ConfigPatchesPerInstallation[inst.Name()] = patch

		filepath = fmt.Sprintf("installations/%s/secret.yaml", inst.Name())
		body, err = fs.ReadFile(filepath)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		secret, err := newConfigFile(filepath, body)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		d.Secrets = append(d.Secrets, secret)
		d.SecretsPerInstallation[inst.Name()] = secret
	}

	// collect default/apps/*/{configmap,secret}-values.yaml.template files
	defaultAppDirs, err := fs.ReadDir("default/apps/")
	if err != nil {
		return nil, microerror.Mask(err)
	}
	for _, app := range defaultAppDirs {
		if !app.IsDir() {
			continue
		}
		uniqueApps[app.Name()] = true
		filepath := fmt.Sprintf("default/apps/%s/configmap-values.yaml.template", app.Name())
		body, err := fs.ReadFile(filepath)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		template, err := newTemplateFile(filepath, body)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		d.Templates = append(d.Templates, template)
		d.TemplatesPerApp[app.Name()] = template

		filepath = fmt.Sprintf("default/apps/%s/secret-values.yaml.template", app.Name())
		body, err = fs.ReadFile(filepath)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		secret, err := newTemplateFile(filepath, body)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		d.SecretTemplates = append(d.SecretTemplates, secret)
		d.SecretTemplatesPerApp[app.Name()] = secret
	}

	// collect installations/*/apps/*/{configmap,secret}-values.yaml.patch files
	for _, inst := range installationDirs {
		if !inst.IsDir() {
			continue
		}
		d.AppsPerInstallation[inst.Name()] = []string{}
		d.TemplatePatchesPerInstallation[inst.Name()] = []*templateFile{}
		d.SecretTemplatePatchesPerInstallation[inst.Name()] = []*templateFile{}
		appDirs, err := fs.ReadDir("default/apps/")
		if err != nil {
			return nil, microerror.Mask(err)
		}
		for _, app := range appDirs {
			if !app.IsDir() {
				continue
			}
			uniqueApps[app.Name()] = true
			d.AppsPerInstallation[inst.Name()] = append(d.AppsPerInstallation[inst.Name()], app.Name())

			filepath := fmt.Sprintf("installations/%s/apps/%s/configmap-values.yaml.patch", inst.Name(), app.Name())
			body, err := fs.ReadFile(filepath)
			if err == nil {
				templatePatch, err := newTemplateFile(filepath, body)
				if err != nil {
					return nil, microerror.Mask(err)
				}
				d.TemplatePatches = append(d.TemplatePatches, templatePatch)
				d.TemplatePatchesPerInstallation[inst.Name()] = append(
					d.TemplatePatchesPerInstallation[inst.Name()],
					templatePatch,
				)
			} else if github.IsNotFound(err) {
				// fallthrough
			} else {
				return nil, microerror.Mask(err)
			}

			filepath = fmt.Sprintf("installations/%s/apps/%s/secret-values.yaml.patch", inst.Name(), app.Name())
			body, err = fs.ReadFile(filepath)
			if err == nil {
				secretPatch, err := newTemplateFile(filepath, body)
				if err != nil {
					return nil, microerror.Mask(err)
				}
				d.SecretTemplatePatches = append(d.SecretTemplatePatches, secretPatch)
				d.SecretTemplatePatchesPerInstallation[inst.Name()] = append(
					d.SecretTemplatePatchesPerInstallation[inst.Name()],
					secretPatch,
				)
			} else if github.IsNotFound(err) {
				// fallthrough
			} else {
				return nil, microerror.Mask(err)
			}
		}
	}

	// collect include files
	includeFiles, err := fs.ReadDir("include/")
	if err != nil {
		return nil, microerror.Mask(err)
	}
	for _, includeFile := range includeFiles {
		if includeFile.IsDir() {
			continue
		}
		filepath := fmt.Sprintf("include/%s", includeFile.Name())
		body, err := fs.ReadFile(filepath)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		includeTemplate, err := newTemplateFile(filepath, body)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		d.Include = append(d.Include, includeTemplate)
	}

	for k := range uniqueInstallations {
		d.Installations = append(d.Installations, k)
	}
	for k := range uniqueApps {
		d.Apps = append(d.Apps, k)
	}
	sort.Strings(d.Installations)
	sort.Strings(d.Apps)

	if err := d.populateconfigValues(); err != nil {
		return nil, microerror.Mask(err)
	}

	return d, nil
}

// populateconfigValues fills UsedBy and overshadowedBy fields in all configValue
// structs in d.Config and d.ConfigPatches. This allows linter to find unused
// values easier.
func (d *discovery) populateconfigValues() error {
	// 1. Mark all overshadowed configValues in config.yaml
	for _, configPatch := range d.ConfigPatches {
		for path := range configPatch.paths {
			if original, ok := d.Config.paths[path]; ok {
				original.overshadowedBy = append(original.overshadowedBy, configPatch)
			}
		}
	}
	// 2. Check templates for all apps x installations, then set UsedBy fields
	// in Config, ConfigPatches
	for _, installation := range d.Installations {
		configPatch, ok := d.ConfigPatchesPerInstallation[installation]
		if !ok {
			configPatch = nil
		}

		for _, app := range d.Apps {
			// mark all fields used by app template's patch
			if templatePatch, ok := d.getAppTemplatePatch(installation, app); ok {
				populatePathsWithUsedBy(templatePatch, d.Config, configPatch)
			}

			// mark all fields used by the app's default template
			if defaultTemplate, ok := d.TemplatesPerApp[app]; ok {
				populatePathsWithUsedBy(defaultTemplate, d.Config, configPatch)
			}
		}
	}

	// 3. Check SECRET templates for all apps x installations, then set UsedBy fields
	for _, installation := range d.Installations {
		secret, ok := d.SecretsPerInstallation[installation]
		if !ok {
			continue
		}
		for _, app := range d.Apps {
			defaultTemplate := d.SecretTemplatesPerApp[app]
			templatePatch, _ := d.getAppSecretTemplatePatch(installation, app)

			populateSecretPathsWithUsedBy(secret, defaultTemplate, templatePatch)
		}
	}

	return nil
}

func populatePathsWithUsedBy(source *templateFile, config, configPatch *configFile) {
	for path, templatePath := range source.values {
		if configPatch != nil {
			configValue, configValueOk := configPatch.paths[path]
			if configValueOk {
				// config patch exists and contains the path
				configValue.usedBy = appendUniqueUsedBy(configValue.usedBy, source)
				continue
			}
		}

		configValue, configValueOk := config.paths[path]
		if configValueOk {
			// the value comes from default config
			configValue.usedBy = appendUniqueUsedBy(configValue.usedBy, source)
			continue
		}

		// value is missing from config; linter will check if it's patched
		templatePath.mayBeMissing = true
	}
}

func populateSecretPathsWithUsedBy(installationSecret *configFile, defaultTemplate, templatePatch *templateFile) {
	if templatePatch != nil {
		for path, value := range templatePatch.values {
			configValue, configValueOk := installationSecret.paths[path]
			if configValueOk {
				configValue.usedBy = appendUniqueUsedBy(configValue.usedBy, templatePatch)
				continue
			}
			value.mayBeMissing = true
		}
	}

	if defaultTemplate != nil {
		for path, value := range defaultTemplate.values {
			if _, ok := templatePatch.paths[path]; ok {
				// already checked; value is overriden by patch in this case
				continue
			}
			configValue, configValueOk := installationSecret.paths[path]
			if configValueOk {
				configValue.usedBy = appendUniqueUsedBy(configValue.usedBy, defaultTemplate)
				continue
			}
			value.mayBeMissing = true
		}

	}
}

func appendUniqueUsedBy(list []*templateFile, t *templateFile) []*templateFile {
	for _, v := range list {
		if v == t {
			return list
		}
	}
	return append(list, t)
}
