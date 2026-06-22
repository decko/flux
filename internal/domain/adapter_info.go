package domain

// AdapterInfo holds metadata about a configured adapter for the API.
type AdapterInfo struct {
	// Type is the adapter identifier (e.g., "github", "jira", "linear").
	Type string `json:"type"`
	// Name is the human-readable display name for the adapter.
	Name string `json:"name"`
	// Health indicates the current status of the adapter connection.
	// One of: "healthy", "unhealthy", "unknown".
	Health string `json:"health"`
}
