package utils

import yaml3 "gopkg.in/yaml.v3"

// IsSOPSEncrypted Each SOPS-encrypted file carries the `sops` key, that in turn carries metadata
// necessary to decrypt it, hence this key is good for discovering files for SOPS decryption.
// Only YAML files are supported.
func IsSOPSEncrypted(data []byte) bool {
	values := make(map[interface{}]interface{})

	err := yaml3.Unmarshal(data, &values)
	if err != nil {
		// We only support YAML, so if it cannot be unmarshalled as one,
		// then it cannot be SOPS encrypted YAML file, let's just return false.
		return false
	}

	_, ok := values["sops"]
	return ok
}
