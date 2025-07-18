package utils

import "testing"

func Test_sortYAMLKeys_null(t *testing.T) {
	t.Parallel()

	out, err := SortYAMLKeys("")
	if err != nil {
		t.Fatalf("err = %#q, want %#v", err, nil)
	}

	if out != "" {
		t.Fatalf("out = %v, want %v", out, "")
	}
}
