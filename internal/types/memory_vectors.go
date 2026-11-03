package types

type ExternalVectorRequest struct {
	ModelID string    `json:"model_id"`
	ItemID  string    `json:"item_id,omitempty"`
	Ticket  string    `json:"ticket,omitempty"`
	Vector  []float32 `json:"vector,omitempty"`
	Query   string    `json:"query,omitempty"`
	Limit   int       `json:"limit,omitempty"`
}
type ExternalVectorItem struct {
	ItemID string `json:"item_id"`
	Text   string `json:"text"`
	Ticket string `json:"ticket"`
}
