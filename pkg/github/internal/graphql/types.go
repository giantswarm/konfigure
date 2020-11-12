package graphql

type Request struct {
	Query     string      `json:"query,omitempty"`
	Variables interface{} `json:"variables,omitempty"`
}
