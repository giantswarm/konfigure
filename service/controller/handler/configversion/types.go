package configversion

type Index struct {
	Entries map[string][]IndexEntry `json:"entries"`
}

type IndexEntry struct {
	Annotations map[string]string `json:"annotations,omitempty"`
	Version     string            `json:"version"`
}
