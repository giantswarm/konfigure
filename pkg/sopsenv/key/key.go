package key

const (
	// keysSource* tell Konfigure how to prepare environment for SOPS.
	// For `kubernetes` it downloads keys from Kubernetes Secrets, while
	// for `local` it runs SOPS against given- or SOPS-default directories.
	KeysSourceKubernetes = "kubernetes"
	KeysSourceLocal      = "local"
)
