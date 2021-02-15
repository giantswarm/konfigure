package lint

import (
	"fmt"
	"os"

	"github.com/giantswarm/microerror"
	"github.com/spf13/cobra"
)

const (
	flagBranch          = "branch"
	flagConfigVersion   = "config-version"
	flagFilterFunctions = "filter-functions"
	flagGithubToken     = "github-token"
	flagMaxMessages     = "max-messages"
	flagNoDescriptions  = "no-descriptions"
	flagNoFuncNames     = "no-function-names"
	flagOnlyErrors      = "only-errors"

	envConfigControllerGithubToken = "CONFIG_CONTROLLER_GITHUB_TOKEN" //nolint:gosec
)

type flag struct {
	Branch          string
	ConfigVersion   string
	FilterFunctions []string
	GitHubToken     string
	MaxMessages     int
	NoDescriptions  bool
	NoFuncNames     bool
	OnlyErrors      bool
}

func (f *flag) Init(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.Branch, flagBranch, "", "Branch of giantswarm/config used to generate configuraton.")
	cmd.Flags().StringVar(&f.ConfigVersion, flagConfigVersion, "", `Major part of the configuration version to use for generation (e.g. "v2").`)
	cmd.Flags().StringSliceVar(&f.FilterFunctions, flagFilterFunctions, []string{}, `Enables filtering linter functions by supplying a list of patterns to match, (e.g. "Lint.*,LintUnusedConfigValues").`)
	cmd.Flags().StringVar(&f.GitHubToken, flagGithubToken, "", fmt.Sprintf(`GitHub token to use for "opsctl create vaultconfig" calls. Defaults to the value of %s env var.`, envConfigControllerGithubToken))
	cmd.Flags().IntVar(&f.MaxMessages, flagMaxMessages, 50, "Max number of linter messages to display. Unlimited output if set to 0. Defaults to 50.")
	cmd.Flags().BoolVar(&f.NoDescriptions, flagNoDescriptions, false, "Disables output of message descriptions.")
	cmd.Flags().BoolVar(&f.NoFuncNames, flagNoFuncNames, false, "Disables output of linter function names.")
	cmd.Flags().BoolVar(&f.OnlyErrors, flagOnlyErrors, false, "Enables linter to output only errors, omitting suggestions.")
}

func (f *flag) Validate() error {
	if f.ConfigVersion == "" && f.Branch == "" {
		f.Branch = "main"
	}
	if f.GitHubToken == "" {
		f.GitHubToken = os.Getenv(envConfigControllerGithubToken)
	}
	if f.GitHubToken == "" {
		return microerror.Maskf(invalidFlagError, "--%s or $%s must not be empty", flagGithubToken, envConfigControllerGithubToken)
	}

	return nil
}
