package project

var (
	description = "konfigure generates and lints application configuration"
	gitSHA      = "n/a"
	name        = "konfigure"
	source      = "https://github.com/giantswarm/konfigure"
	version     = "0.11.1-dev"
)

func Description() string {
	return description
}

func GitSHA() string {
	return gitSHA
}

func Name() string {
	return name
}

func Source() string {
	return source
}

func Version() string {
	return version
}
