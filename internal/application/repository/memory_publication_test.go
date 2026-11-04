package repository

import (
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func TestPublicationClearFence(t *testing.T) {
	repo, db := newMemoryAdminHarness(t)
	require.NoError(t, db.AutoMigrate(&types.MemoryPublication{}))
	scope := interfaces.MemoryScope{TenantID: 1, SubjectID: "web_user:publication"}
	subject, err := repo.EnsureSubject(t.Context(), scope)
	require.NoError(t, err)
	stale := scope.WithGeneration(subject.ExtractionState.Generation)
	_, err = repo.ClearSubjectMemory(t.Context(), scope)
	require.NoError(t, err)
	expires := time.Now().Add(time.Hour)
	item := &types.MemoryItem{ID: "publication-item", TenantID: 1, SubjectID: scope.SubjectID, Kind: "fact", Content: "synthetic", Status: "active", ExpiresAt: &expires}
	receipt := &types.MemoryPublication{Key: strings.Repeat("a", 64), PayloadHash: strings.Repeat("b", 64), RequestHash: strings.Repeat("c", 64), ItemID: item.ID}
	_, err = repo.PublishMemory(t.Context(), stale, receipt, item, 200)
	require.ErrorIs(t, err, types.ErrMemoryGenerationChanged)
	got, err := repo.GetPublication(t.Context(), scope, receipt.Key)
	require.NoError(t, err)
	require.Nil(t, got)
	found, err := repo.GetItem(t.Context(), scope, item.ID)
	require.NoError(t, err)
	require.Nil(t, found)
}
