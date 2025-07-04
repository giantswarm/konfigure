package utils

import (
	"bytes"
	"sort"

	yaml3 "gopkg.in/yaml.v3"
)

func SortYAMLKeys(yamlString string) (string, error) {
	if yamlString == "" {
		return yamlString, nil
	}

	n := new(yaml3.Node)
	err := yaml3.Unmarshal([]byte(yamlString), n)
	if err != nil {
		return "", err
	}
	sortYAMLKeysNode(n)
	buf := new(bytes.Buffer)
	enc := yaml3.NewEncoder(buf)
	enc.SetIndent(2)
	err = enc.Encode(n)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Coped (and adapted) from:
// https://github.com/mikefarah/yq/blob/fe12407c936cc4dacf7495a04b5881d14e7b0f47/pkg/yqlib/operator_sort_keys.go#L32
func sortYAMLKeysNode(node *yaml3.Node) {
	if node.Kind == yaml3.DocumentNode || node.Kind == yaml3.SequenceNode {
		for _, n := range node.Content {
			sortYAMLKeysNode(n)
		}
	}
	if node.Kind != yaml3.MappingNode {
		return
	}

	keys := make([]string, len(node.Content)/2)
	keyBucket := map[string]*yaml3.Node{}
	valueBucket := map[string]*yaml3.Node{}
	var contents = node.Content
	for index := 0; index < len(contents); index = index + 2 {
		key := contents[index]
		value := contents[index+1]
		keys[index/2] = key.Value
		keyBucket[key.Value] = key
		valueBucket[key.Value] = value

		sortYAMLKeysNode(value)
	}
	sort.Strings(keys)
	sortedContent := make([]*yaml3.Node, len(node.Content))
	for index := 0; index < len(keys); index = index + 1 {
		keyString := keys[index]
		sortedContent[index*2] = keyBucket[keyString]
		sortedContent[1+(index*2)] = valueBucket[keyString]
	}
	node.Content = sortedContent
}
