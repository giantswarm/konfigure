package project

var (
	description = "konfigure generates and lints application configuration"
	gitSHA      = "n/a"
	name        = "konfigure"
	source      = "https://github.com/giantswarm/konfigure"
	version     = "2.1.0"
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
