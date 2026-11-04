package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
	"time"
)

func publicationID(scope interfaces.MemoryScope, key string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", scope.TenantID, scope.SubjectID) + "\x00" + key))
	return hex.EncodeToString(sum[:])
}
func (r *memoryRepository) GetPublication(ctx context.Context, scope interfaces.MemoryScope, key string) (*types.MemoryPublication, error) {
	var receipt types.MemoryPublication
	err := r.scoped(ctx, scope).Where("id = ?", publicationID(scope, key)).First(&receipt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &receipt, err
}

// Item and receipt commit in the same subject transaction, fenced against clear.
// Deliberately create-only: no automatic superseding, archival or dedup rewrite.
func (r *memoryRepository) PublishMemory(ctx context.Context, scope interfaces.MemoryScope, receipt *types.MemoryPublication, item *types.MemoryItem, maximum int) (*types.MemoryPublication, error) {
	var result *types.MemoryPublication
	err := r.withSubject(ctx, scope, func(tx *gorm.DB, subject *types.MemorySubject) error {
		if !subject.Enabled {
			return types.ErrMemoryConflict
		}
		repo := &memoryRepository{db: tx}
		previous, err := repo.GetPublication(ctx, scope, receipt.Key)
		if err != nil {
			return err
		}
		if previous != nil {
			if previous.RequestHash != receipt.RequestHash {
				return types.ErrMemoryConflict
			}
			result = previous
			return nil
		}
		var count int64
		if err = repo.scoped(ctx, scope).Model(&types.MemoryPublication{}).Count(&count).Error; err != nil {
			return err
		}
		if count >= 10000 {
			return types.ErrMemoryConflict
		}
		if err = repo.scoped(ctx, scope).Model(&types.MemoryItem{}).Where("status = ?", types.MemoryStatusActive).Count(&count).Error; err != nil {
			return err
		}
		if count >= int64(maximum) {
			return types.ErrMemoryConflict
		}
		forgotten, err := repo.HasTombstone(ctx, scope, types.MemoryFingerprint(item.Content))
		if err != nil {
			return err
		}
		if forgotten {
			return types.ErrMemoryPreviouslyForgotten
		}
		var duplicates int64
		if err = repo.scoped(ctx, scope).Model(&types.MemoryItem{}).Where("status IN ? AND (normalized_key = ? OR content = ?)", []string{types.MemoryStatusActive, types.MemoryStatusPending}, item.NormalizedKey, item.Content).Count(&duplicates).Error; err != nil {
			return err
		}
		if duplicates > 0 {
			return types.ErrMemoryConflict
		}
		if !item.ExpiresAt.After(time.Now()) {
			return types.ErrMemoryConflict
		}
		receipt.ID = publicationID(scope, receipt.Key)
		receipt.TenantID = scope.TenantID
		receipt.SubjectID = scope.SubjectID
		if err = tx.Create(item).Error; err != nil {
			return err
		}
		if err = tx.Create(receipt).Error; err != nil {
			return err
		}
		result = receipt
		return nil
	})
	return result, err
}

// ForgetPublication commits a create fence even if a submitted publication has not arrived.
// A late POST with this key can never recreate content. Item, embedding and fence share one subject transaction.
func (r *memoryRepository) ForgetPublication(ctx context.Context, scope interfaces.MemoryScope, key, payloadHash string) error {
	return r.withSubject(ctx, scope, func(tx *gorm.DB, _ *types.MemorySubject) error {
		repo := &memoryRepository{db: tx}
		receipt, err := repo.GetPublication(ctx, scope, key)
		if err != nil {
			return err
		}
		if receipt == nil {
			receipt = &types.MemoryPublication{ID: publicationID(scope, key), TenantID: scope.TenantID, SubjectID: scope.SubjectID, Key: key, PayloadHash: payloadHash, RequestHash: "erased", ItemID: ""}
			return tx.Create(receipt).Error
		}
		if receipt.PayloadHash != payloadHash {
			return types.ErrMemoryConflict
		}
		if err = tx.Where("tenant_id = ? AND subject_id = ? AND item_id = ?", scope.TenantID, scope.SubjectID, receipt.ItemID).Delete(&types.MemoryItemEmbedding{}).Error; err != nil {
			return err
		}
		if err = tx.Where("tenant_id = ? AND subject_id = ? AND id = ?", scope.TenantID, scope.SubjectID, receipt.ItemID).Delete(&types.MemoryItem{}).Error; err != nil {
			return err
		}
		return tx.Model(&types.MemoryPublication{}).Where("id = ?", receipt.ID).Update("request_hash", "erased").Error
	})
}
