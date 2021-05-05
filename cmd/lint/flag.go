package lint

import (
	"regexp"
	"strings"

	"github.com/giantswarm/microerror"
	"github.com/spf13/cobra"
)

const (
	flagDir              = "dir"
	flagFilterFunctions  = "filter-functions"
	flagMaxMessages      = "max-messages"
	flagNoDescriptions   = "no-descriptions"
	flagNoFuncNames      = "no-function-names"
	flagOnlyErrors       = "only-errors"
	flagSkipFieldsRegexp = "skip-fields-regexp"
)

type flag struct {
	Dir              string
	FilterFunctions  []string
	MaxMessages      int
	NoDescriptions   bool
	NoFuncNames      bool
	OnlyErrors       bool
	SkipFieldsRegexp string
}

func (f *flag) Init(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.Dir, flagDir, ".", `Directory containing configuration source (e.g cloned "giantswarm/config" repo).`)
	cmd.Flags().StringSliceVar(&f.FilterFunctions, flagFilterFunctions, []string{}, `Enables filtering linter functions by supplying a list of patterns to match, (e.g. "Lint.*,LintUnusedConfigValues").`)
	cmd.Flags().IntVar(&f.MaxMessages, flagMaxMessages, 50, "Max number of linter messages to display. Unlimited output if set to 0. Defaults to 50.")
	cmd.Flags().BoolVar(&f.NoDescriptions, flagNoDescriptions, false, "Disables output of message descriptions.")
	cmd.Flags().BoolVar(&f.NoFuncNames, flagNoFuncNames, false, "Disables output of linter function names.")
	cmd.Flags().BoolVar(&f.OnlyErrors, flagOnlyErrors, false, "Enables linter to output only errors, omitting suggestions.")
	cmd.Flags().StringVar(&f.SkipFieldsRegexp, flagSkipFieldsRegexp, "", "List of regexp matchers to match field paths, which don't require validation.")
}

func (f *flag) Validate() error {
	res := strings.Split(f.SkipFieldsRegexp, ",")
	if res[0] == "" {
		return nil
	}

	for _, re := range res {
		_, err := regexp.Compile(re)
		if err != nil {
			return microerror.Maskf(invalidFlagError, "%#q must be a valid regex string", re)
		}
	}

	return nil
}
