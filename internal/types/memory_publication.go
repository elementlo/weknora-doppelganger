package types

import "time"

// MemoryPublication is a content-free, durable idempotency receipt. It survives
// user forget: absence of an item must never allow an old key to recreate it.
type MemoryPublication struct {
	ID          string    `json:"-" gorm:"primaryKey;type:varchar(64)"`
	TenantID    uint64    `json:"-" gorm:"not null"`
	SubjectID   string    `json:"-" gorm:"type:varchar(512);not null"`
	Key         string    `json:"key" gorm:"column:publication_key;type:varchar(64);not null"`
	RequestHash string    `json:"-" gorm:"type:varchar(64);not null"`
	PayloadHash string    `json:"payload_hash" gorm:"type:varchar(64);not null"`
	ItemID      string    `json:"item_id" gorm:"type:varchar(36);not null"`
	CreatedAt   time.Time `json:"created_at"`
}

func (MemoryPublication) TableName() string { return "memory_publications" }

type MemoryPublicationRequest struct {
	Key         string    `json:"key"`
	PayloadHash string    `json:"payload_hash"`
	Generation  string    `json:"generation"`
	Kind        string    `json:"kind"`
	Content     string    `json:"content"`
	Importance  int       `json:"importance"`
	ExpiresAt   time.Time `json:"expires_at"`
}
type MemoryPublicationResult struct {
	Receipt *MemoryPublication `json:"receipt"`
	Item    *MemoryItem        `json:"item"`
	// A receipt records historical commit; item may since be forgotten or edited.
	State string `json:"state"`
}
