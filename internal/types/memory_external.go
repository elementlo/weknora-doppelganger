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
