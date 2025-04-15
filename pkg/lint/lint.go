package lint

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/giantswarm/konfigure/pkg/generator"
)

const (
	overshadowErrorThreshold float64 = 0.75
)

type linterFunc func(d *discovery) (messages LinterMessages)

var allLinterFunctions = []linterFunc{
	lintUnusedConfigValues,
	lintDuplicateconfigValues,
	lintovershadowedconfigValues,
	lintUnusedConfigPatchValues,
	lintUndefinedTemplateValues,
	lintUndefinedTemplatePatchValues,
	lintUnusedSecretValues,
	lintUndefinedSecrettemplateValues,
	lintUndefinedSecretTemplatePatchValues,
	lintUnencryptedSecretValues,
	lintIncludeFiles,
}

type Config struct {
	Store            generator.Filesystem
	FilterFunctions  []string
	OnlyErrors       bool
	MaxMessages      int
	SkipFieldsRegexp []string
}

type Linter struct {
	discovery        *discovery
	funcs            []linterFunc
	onlyErrors       bool
	maxMessages      int
	skipFieldsRegexp []*regexp.Regexp
}

func New(c Config) (*Linter, error) {
	discovery, err := newDiscovery(c.Store)
	if err != nil {
		return nil, err
	}

	var skipREs []*regexp.Regexp
	{
		for _, re := range c.SkipFieldsRegexp {
			if re == "" {
				continue
			}

			matcher, err := regexp.Compile(re)
			if err != nil {
				return nil, err
			}

			skipREs = append(skipREs, matcher)

		}
	}

	l := &Linter{
		discovery:        discovery,
		funcs:            getFilteredLinterFunctions(c.FilterFunctions),
		onlyErrors:       c.OnlyErrors,
		maxMessages:      c.MaxMessages,
		skipFieldsRegexp: skipREs,
	}

	return l, nil
}

func (l *Linter) Lint(ctx context.Context) (messages LinterMessages) {
	fmt.Printf("Linting using %d functions\n\n", len(l.funcs))
	for _, f := range l.funcs {
		singleFuncMessages := f(l.discovery)
		sort.Sort(singleFuncMessages)

		for _, msg := range singleFuncMessages {
			if skipValidation(msg.Path(), l.skipFieldsRegexp) {
				continue
			}
			if l.onlyErrors && !msg.IsError() {
				continue
			}
			messages = append(messages, msg)

			if l.maxMessages > 0 && len(messages) >= l.maxMessages {
				return messages
			}
		}
	}
	return messages
}

func lintDuplicateconfigValues(d *discovery) (messages LinterMessages) {
	for path, defaultPath := range d.Config.paths {
		for _, overshadowingPatch := range defaultPath.overshadowedBy {
			patchedPath := overshadowingPatch.paths[path]
			if reflect.DeepEqual(defaultPath.value, patchedPath.value) {
				messages = append(
					messages,
					newError(overshadowingPatch.filepath, path, "is duplicate of the same path in %s", d.Config.filepath),
				)
			}
		}
	}
	return messages
}

func lintovershadowedconfigValues(d *discovery) (messages LinterMessages) {
	if len(d.Installations) == 0 {
		return // avoid division by 0
	}
	for path, configValue := range d.Config.paths {
		if len(configValue.overshadowedBy) == len(d.Installations) {
			messages = append(
				messages,
				newError(d.Config.filepath, path, "is overshadowed by all config.yaml.patch files"),
			)
		} else if float64(len(configValue.overshadowedBy)/len(d.Installations)) >= overshadowErrorThreshold {
			msg := newMessage(
				d.Config.filepath, path, "is overshadowed by %d/%d patches",
				len(configValue.overshadowedBy), len(d.Installations),
			).WithDescription("consider removing it from %s", d.Config.filepath)
			messages = append(messages, msg)
		}
	}
	return messages
}

func lintUnusedConfigPatchValues(d *discovery) (messages LinterMessages) {
	for _, configPatch := range d.ConfigPatches {
		if len(d.AppsPerInstallation[configPatch.installation]) == 0 {
			continue // avoid division by 0
		}
		for path, configValue := range configPatch.paths {
			if len(configValue.usedBy) > 0 {
				continue
			}
			messages = append(messages, newError(configPatch.filepath, path, "is unused"))
		}
	}
	return messages
}

func lintUnusedConfigValues(d *discovery) (messages LinterMessages) {
	if len(d.Installations) == 0 || len(d.Apps) == 0 {
		return // what's the point, nothing is defined
	}
	for path, configValue := range d.Config.paths {
		if len(configValue.usedBy) == 0 {
			messages = append(messages, newError(d.Config.filepath, path, "is unused"))
		} else if len(configValue.usedBy) == 1 {
			msg := newMessage(d.Config.filepath, path, "is used by just one app: %s", configValue.usedBy[0].app).
				WithDescription("consider moving this value to %s template or template patch", configValue.usedBy[0].app)
			messages = append(messages, msg)
		}
	}
	return messages
}

func lintUnusedSecretValues(d *discovery) (messages LinterMessages) {
	if len(d.Installations) == 0 {
		return // what's the point, nothing is defined
	}
	for _, secretFile := range d.Secrets {
		for path, configValue := range secretFile.paths {
			if len(configValue.usedBy) == 0 && !strings.HasPrefix(path, "sops.") {
				messages = append(messages, newError(secretFile.filepath, path, "is unused"))
			} else if len(configValue.usedBy) == 1 {
				msg := newMessage(secretFile.filepath, path, "is used by just one app: %s", configValue.usedBy[0].app).
					WithDescription("consider moving this value to %s secret-values patch", configValue.usedBy[0].app)
				messages = append(messages, msg)
			}

		}
	}
	return messages
}

func lintUndefinedSecrettemplateValues(d *discovery) (messages LinterMessages) {
	for _, template := range d.SecretTemplates {
		for path, value := range template.values {
			if !value.mayBeMissing {
				continue
			}

			// The path may be tree node (".registry"), but not tree leaf (".registry.domain").
			// Let's check for this. It's enough if it occurs just once.
			found := false
			for _, secret := range d.Secrets {
				if _, err := secret.pathmodifier.Get(PathmodifierPath(path)); err != nil {
					found = true
					break
				}
			}
			if found {
				continue
			}

			messages = append(messages, newError(template.filepath, path, "is templated but never configured"))
		}
	}
	return messages
}

func lintUndefinedSecretTemplatePatchValues(d *discovery) (messages LinterMessages) {
	for _, template := range d.SecretTemplatePatches {
		for path, value := range template.values {
			if !value.mayBeMissing {
				continue
			}

			messages = append(messages, newError(template.filepath, path, "is templated but never configured"))
		}
	}
	return messages
}

func lintUndefinedTemplateValues(d *discovery) (messages LinterMessages) {
	for _, template := range d.Templates {
		for path, value := range template.values {
			if !value.mayBeMissing {
				continue
			}

			used := false

			for _, configPatch := range d.ConfigPatches {
				if _, ok := configPatch.paths[path]; ok {
					used = true
					break
				}
			}
			if used {
				continue
			}

			for _, templatePatch := range d.TemplatePatches {
				if _, ok := templatePatch.paths[path]; ok {
					used = true
					break
				}

				for templatePatchPath := range templatePatch.paths {
					if strings.HasPrefix(templatePatchPath, path+".") {
						used = true
						break
					}
				}

				if used {
					break
				}
			}

			if used {
				continue
			}
			messages = append(messages, newError(template.filepath, path, "is templated but never configured"))
		}
	}
	return messages
}

func lintUnencryptedSecretValues(d *discovery) (messages LinterMessages) {
	if len(d.Installations) == 0 {
		return // what's the point, nothing is defined
	}
	for _, secretFile := range d.Secrets {
		for path, configValue := range secretFile.paths {
			if strings.HasPrefix(path, "sops.") && path != "sops.mac" {
				continue
			}

			stringValue, ok := (configValue.value).(string)
			if !ok {
				continue
			}
			if !strings.HasPrefix(stringValue, "vault:v1:") && (!strings.HasPrefix(stringValue, "ENC[AES256_GCM") || !strings.HasSuffix(stringValue, "]")) {
				messages = append(
					messages,
					newError(secretFile.filepath, path, "is not encrypted with Vault").
						WithDescription("valid secret values are encrypted with installation Vault's token and start with \"vault:v1:\" prefix, or SOPS encrypted and start with \"ENC[AES256_GCM\" then end with \"]\""),
				)
			}
		}
	}
	return messages
}

func lintUndefinedTemplatePatchValues(d *discovery) (messages LinterMessages) {
	for _, templatePatch := range d.TemplatePatches {
		for path, value := range templatePatch.values {
			if !value.mayBeMissing {
				continue
			}
			messages = append(messages, newError(templatePatch.filepath, path, "is templated but never configured"))
		}
	}
	return messages
}

func lintIncludeFiles(d *discovery) (messages LinterMessages) {
	used := map[string]bool{}
	exist := map[string]bool{}
	for _, includeFile := range d.Include {
		exist[includeFile.filepath] = true
		for _, filepath := range includeFile.includes {
			used[filepath] = true
		}
	}

	for _, template := range d.Templates {
		for _, filepath := range template.includes {
			used[filepath] = true
		}
	}

	for _, templatePatch := range d.TemplatePatches {
		for _, filepath := range templatePatch.includes {
			used[filepath] = true
		}
	}

	if reflect.DeepEqual(exist, used) {
		return messages
	}

	for filepath := range exist {
		if _, ok := used[filepath]; !ok {
			messages = append(messages, newError(filepath, "*", "is never included"))
		}
	}

	for filepath := range used {
		if _, ok := exist[filepath]; !ok {
			messages = append(messages, newError(filepath, "*", "is included but does not exist"))
		}
	}

	return messages
}

// ------ helper funcs -------
func getFilteredLinterFunctions(filters []string) []linterFunc {
	if len(filters) == 0 {
		return allLinterFunctions
	}

	functions := []linterFunc{}
	for _, function := range allLinterFunctions {
		name := runtime.FuncForPC(reflect.ValueOf(function).Pointer()).Name()
		name = strings.ToLower(name)
		for _, filter := range filters {
			re := regexp.MustCompile(strings.ToLower(filter))
			if re.MatchString(name) {
				functions = append(functions, function)
				break
			}
		}
	}

	return functions
}

func skipValidation(msg string, matchers []*regexp.Regexp) bool {
	for _, re := range matchers {
		if re.MatchString(msg) {
			return true
		}
	}

	return false
}
