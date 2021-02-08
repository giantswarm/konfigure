package generator

import (
	"strconv"
	"strings"

	"github.com/giantswarm/microerror"
)

// toTagPrefix besides couple corner cases (see tests) returns ("v2", true,
// nil) when configVersion is "2.x.x". It returns ("", false, nil) otherwise
// with assumption the given string is a branch name.
func toTagPrefix(configVersion string) (tagPrefix string, isTagRange bool, err error) {
	configVersion = strings.TrimSpace(configVersion)

	split := strings.SplitN(configVersion, ".", 2)
	if !isNumber(split[0]) {
		if isNumber(strings.TrimPrefix(split[0], "v")) {
			return "", false, microerror.Maskf(executionFailedError, "configuration version for a tag range should not start with %#q prefix got %q", "v", configVersion)
		}

		// If the string doesn't start with a number and dot assume
		// this is a valid branch name.
		return "", false, nil
	}
	if len(split) != 2 || split[1] != "x.x" {
		return "", false, microerror.Maskf(executionFailedError, "configuration version starting with a number followed by dot is supposed to end with %q got %q", "x.x", configVersion)
	}

	return "v" + split[0], true, nil
}

func isNumber(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
