package lint

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strconv"
	"strings"
	"text/template/parse"

	"github.com/Masterminds/sprig/v3"
	"github.com/ghodss/yaml"

	"github.com/giantswarm/microerror"
	pathmodifier "github.com/giantswarm/valuemodifier/path"
)

var (
	fMap                 = dummyFuncMap()
	templatePathPattern  = regexp.MustCompile(`(?:\{\{.*?[\-\s\(]?)(\.[a-zA-Z][a-zA-Z0-9_\.]+)(?:[\-\s\)]?.*?\}\})`)
	includeFilePattern   = regexp.MustCompile(`(?:\{\{.*?[\-\s]?)include(?:\s?['"])([a-zA-Z0-9\-]+)(?:\s?['"])(?:[\-\s]?.*?\}\})`)
	yamlErrorLinePattern = regexp.MustCompile(`yaml: line (\d+)`)
)

type configFile struct {
	filepath     string
	installation string // optional
	paths        map[string]*configValue
	pathmodifier *pathmodifier.Service
}

type configValue struct {
	value interface{}
	// files using this value
	usedBy []*templateFile
	// value is overshadowed by some files
	overshadowedBy []*configFile
}

// templateFile contains a representation of values and paths in a template.
// Paths map contains all paths in template extracted by valuemodifier/path.
// Values map contains values requested in template using template's dot
// notation, e.g. '{{ .some.value }}'. In that case the key would be
// 'some.value'.
//
// A simple template, like:
// ```
// keyA:
//   keyB:  "{{ .get.this.from.config }}"
// ```
// would produce the following:
// paths: {"keyA.keyB": true}
// values: {"get.this.from.config": templateValue{...}}
type templateFile struct {
	filepath     string
	installation string // optional for defaults
	app          string

	values map[string]*templateValue
	paths  map[string]bool
	// includes contains names of all include files used by this template
	includes []string
}

type templateValue struct {
	path            string
	occurrenceCount int
	// mayBeMissing is set when value is not found in config.
	// Linter will check if it's patched in by any of the template patches. If
	// yes, fine. If not, that's an error and linter will let you know.
	mayBeMissing bool
}

func newConfigFile(filepath string, body []byte) (*configFile, error) {
	if !strings.HasSuffix(filepath, ".yaml") && !strings.HasSuffix(filepath, ".yaml.patch") {
		return nil, microerror.Maskf(executionFailedError, "given file is not a value file: %q", filepath)
	}

	// extract paths with valuemodifier path service
	var pathmodifierSvc *pathmodifier.Service
	allPaths := map[string]*configValue{}
	{
		c := pathmodifier.Config{
			InputBytes: body,
			Separator:  ".",
		}
		svc, err := pathmodifier.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
		pathmodifierSvc = svc

		paths, err := svc.All()
		if err != nil {
			return nil, microerror.Maskf(executionFailedError, "error getting all paths for %q", filepath)
		}

		for _, path := range paths {
			value, err := svc.Get(path)
			if err != nil {
				return nil, microerror.Maskf(executionFailedError, "error getting %q value for %q: %s", filepath, path, err)
			}

			v := configValue{
				value:          value,
				usedBy:         []*templateFile{},
				overshadowedBy: []*configFile{},
			}
			allPaths[NormalPath(path)] = &v
		}
	}

	vf := &configFile{
		filepath:     filepath,
		paths:        allPaths,
		pathmodifier: pathmodifierSvc,
	}

	// assign installation if possible
	if strings.HasPrefix(filepath, "installations") {
		elements := strings.Split(filepath, "/")
		vf.installation = elements[1]
	}

	return vf, nil
}

func newTemplateFile(filepath string, body []byte) (*templateFile, error) {
	if !strings.HasSuffix(filepath, ".template") && !strings.HasSuffix(filepath, "values.yaml.patch") {
		return nil, microerror.Maskf(executionFailedError, "given file is not a template: %q", filepath)
	}

	tf := &templateFile{
		filepath: filepath,
		includes: []string{},
	}

	// extract included filenames
	includeStatements := includeFilePattern.FindAllStringSubmatch(string(body), -1)
	for _, matchSlice := range includeStatements {
		tf.includes = append(tf.includes, "include/"+matchSlice[1]+".yaml.template")
	}

	// extract templated values and all paths from the template
	values := map[string]*templateValue{}
	paths := map[string]bool{}
	{
		t, err := template.
			New(filepath).
			Funcs(fMap).
			Option("missingkey=zero").
			Parse(string(body))
		if err != nil {
			return nil, microerror.Mask(err)
		}

		// extract all values
		for _, node := range t.Tree.Root.Nodes {
			if node.Type() == parse.NodeText {
				continue
			}

			nodePaths := templatePathPattern.FindAllStringSubmatch(node.String(), -1)
			for _, npSlice := range nodePaths {
				np := npSlice[1]
				normalPath := NormalPath(np)
				if _, ok := values[normalPath]; !ok {
					values[normalPath] = &templateValue{
						path:            normalPath,
						occurrenceCount: 1,
					}
				} else {
					values[normalPath].occurrenceCount += 1
				}
			}
		}

		// extract all paths
		output := bytes.NewBuffer([]byte{})
		var data interface{}
		// Render template without values. All templated values will be
		// replaced by default zero values: "" for string, 0 for int, false
		// for bool etc.
		err = t.Execute(output, data)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		c := pathmodifier.Config{
			InputBytes: output.Bytes(),
			Separator:  ".",
		}

		svc, err := pathmodifier.New(c)
		if err != nil {
			// try to pretty print offending yaml
			var yamlOut interface{}
			yamlErr := yaml.Unmarshal(output.Bytes(), &yamlOut)

			if yamlErr == nil {
				return nil, microerror.Mask(err)
			}

			matches := yamlErrorLinePattern.FindAllStringSubmatch(yamlErr.Error(), -1)
			if len(matches) == 0 {
				return nil, microerror.Mask(err)
			}

			lineNo, convErr := strconv.Atoi(matches[0][1])
			if convErr != nil {
				return nil, microerror.Mask(err)
			}

			lines := strings.Split(output.String(), "\n")
			fmt.Println(red(yamlErr.Error()))
			fmt.Println("In " + filepath)
			if lineNo > 1 {
				fmt.Println("> " + lines[lineNo-2])
			}
			fmt.Println("> " + red(lines[lineNo-1]))
			if lineNo < len(lines)-2 {
				fmt.Println("> " + lines[lineNo])
			}

			return nil, microerror.Mask(err)
		}

		pathList, err := svc.All()
		if err != nil {
			return nil, microerror.Mask(err)
		}

		for _, p := range pathList {
			paths[p] = true
		}
	}
	tf.values = values
	tf.paths = paths

	// fill in installation and app if possible
	{
		elements := strings.Split(filepath, "/")
		if strings.HasPrefix(filepath, "installations") {
			tf.installation = elements[1]
			tf.app = elements[3]
		} else if strings.HasPrefix(filepath, "default") {
			tf.app = elements[2]
		}
		// else it's an include file and has neither app nor installation
	}

	return tf, nil
}

func NormalPath(path string) string {
	return strings.TrimPrefix(path, ".")
}

func PathmodifierPath(path string) string {
	if strings.HasPrefix(path, ".") {
		return path
	}
	return "." + path
}

func dummyFuncMap() template.FuncMap {
	// sprig.funcMap
	dummy := template.FuncMap{}
	for fName := range sprig.FuncMap() {
		dummy[fName] = func(args ...interface{}) string {
			return ""
		}
	}
	// built-ins, which might be affected by interface comparison
	for _, fName := range []string{"eq", "ne", "include"} {
		dummy[fName] = func(args ...interface{}) string {
			return ""
		}
	}
	return dummy
}
