package types

// ExternalMemoryInput is authorized, prefiltered context from a host such as
// Harness. It is not imported as a native WeKnora chat session.
type ExternalMemorySource struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
	Hash     string `json:"hash"`
	Role     string `json:"role"`
	Content  string `json:"content"`
}
type ExternalMemoryInput struct {
	Sources []ExternalMemorySource `json:"sources"`
}
type ExternalMemoryCandidate struct {
	Kind       string               `json:"kind"`
	Content    string               `json:"content"`
	Importance int                  `json:"importance"`
	Source     ExternalMemorySource `json:"source"`
}
type ExternalMemoryResult struct {
	Candidates               []ExternalMemoryCandidate `json:"candidates"`
	ModelCallsReserved       int                       `json:"model_calls_reserved"`
	CompletionTokensReserved int                       `json:"completion_tokens_reserved"`
}

// ExternalMemoryPlan contains no credentials; inference belongs to the host.
type ExternalMemoryPlan struct {
	Ticket           string      `json:"ticket"`
	ExpiresAt        int64       `json:"expires_at"`
	System           string      `json:"system"`
	Prompt           string      `json:"prompt"`
	Schema           interface{} `json:"schema"`
	CompletionLimits []int       `json:"completion_limits"`
	ExistingCoverage string      `json:"existing_coverage"`
}
type ExternalMemoryResolution struct {
	Sources []ExternalMemorySource `json:"sources"`
	Ticket  string                 `json:"ticket"`
	Content string                 `json:"content"`
}
