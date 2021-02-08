package generator

import (
	"reflect"
	"strconv"
	"testing"
)

func Test_toTagPrefix(t *testing.T) {
	notNil := func(err error) bool { return err != nil }

	testCases := []struct {
		name               string
		inputConfigVersion string
		expectedTagPrefix  string
		expectedIsTagRange bool
		errorMatcher       func(err error) bool
	}{
		{
			name:               "case 0: valid major",
			inputConfigVersion: "3.x.x",
			expectedTagPrefix:  "v3",
			expectedIsTagRange: true,
			errorMatcher:       nil,
		},
		{
			name:               "case 1: valid branch",
			inputConfigVersion: "my-branch",
			expectedTagPrefix:  "",
			expectedIsTagRange: false,
			errorMatcher:       nil,
		},
		{
			name:               "case 2: valid empty string",
			inputConfigVersion: "",
			errorMatcher:       nil,
		},
		{
			name:               "case 3: valid - starts with number but no dot",
			inputConfigVersion: "3abc",
			expectedTagPrefix:  "",
			expectedIsTagRange: false,
			errorMatcher:       nil,
		},
		{
			name:               "case 4: valid - starts with v and number but no dot",
			inputConfigVersion: "v3abc",
			expectedTagPrefix:  "",
			expectedIsTagRange: false,
			errorMatcher:       nil,
		},
		{
			name:               "case 5: invalid - starts with v, number and dot",
			inputConfigVersion: "v3.x.x",
			errorMatcher:       notNil,
		},
		{
			name:               "case 6: invalid - starts with v, number and dot",
			inputConfigVersion: "v3.abc",
			errorMatcher:       notNil,
		},
		{
			name:               "case 7: invalid - provided minor",
			inputConfigVersion: "1.2.x",
			errorMatcher:       notNil,
		},
		{
			name:               "case 8: invalid - provided minor and patch",
			inputConfigVersion: "1.2.3",
			errorMatcher:       notNil,
		},
		{
			name:               "case 9: invalid - starts with number and dot",
			inputConfigVersion: "100.not-x.x",
			errorMatcher:       notNil,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Log(tc.name)

			tagPrefix, isTagRange, err := toTagPrefix(tc.inputConfigVersion)

			switch {
			case err == nil && tc.errorMatcher == nil:
				// correct; carry on
			case err != nil && tc.errorMatcher == nil:
				t.Fatalf("error == %#v, want nil", err)
			case err == nil && tc.errorMatcher != nil:
				t.Fatalf("error == nil, want non-nil")
			case !tc.errorMatcher(err):
				t.Fatalf("error == %#v, want matching", err)
			}

			if tc.errorMatcher != nil {
				return
			}

			if !reflect.DeepEqual(tagPrefix, tc.expectedTagPrefix) {
				t.Fatalf("tagPrefix = %v, want %v", tagPrefix, tc.expectedTagPrefix)
			}

			if !reflect.DeepEqual(isTagRange, tc.expectedIsTagRange) {
				t.Fatalf("isTagRange = %v, want %v", isTagRange, tc.expectedIsTagRange)
			}
		})
	}
}
