package configversion

type Index struct {
	Entries map[string][]IndexEntry `json:"entries"`
}

type IndexEntry struct {
	ConfigVersion string `json:"configVersion,omitempty"`
	Version       string `json:"version"`
}
