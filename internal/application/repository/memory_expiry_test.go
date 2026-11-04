package repository

import (
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestMemoryExpiryComparesInstantsAcrossOffsets(t *testing.T) {
	repo, _ := newMemoryAdminHarness(t)
	scope := interfaces.MemoryScope{TenantID: 1, SubjectID: "web_user:timezone"}
	_, err := repo.EnsureSubject(t.Context(), scope)
	require.NoError(t, err)
	// An offset extreme makes the old string comparison fail independent of host timezone.
	future := time.Now().Add(time.Hour).In(time.FixedZone("minus12", -12*3600))
	past := time.Now().Add(-time.Hour).In(time.FixedZone("plus14", 14*3600))
	for _, value := range []struct {
		id     string
		expiry time.Time
	}{{"future", future}, {"past", past}} {
		require.NoError(t, repo.CreateItem(t.Context(), &types.MemoryItem{ID: value.id, TenantID: 1, SubjectID: scope.SubjectID, Kind: "fact", Content: "synthetic " + value.id, Status: "active", ExpiresAt: &value.expiry}))
		require.NoError(t, repo.UpsertItemEmbedding(t.Context(), scope, &types.MemoryItemEmbedding{ItemID: value.id, ModelID: "model", Dims: 2, Vector: types.EncodeEmbedding([]float32{1, 0})}))
	}
	active, err := repo.ListActiveByKinds(t.Context(), scope, []string{"fact"}, 10)
	require.NoError(t, err)
	require.Len(t, active, 1)
	require.Equal(t, "future", active[0].ID)
	hits, err := repo.rankInProcess(t.Context(), scope, interfaces.MemoryVectorQuery{ModelID: "model", Vector: []float32{1, 0}, MinScore: 0, Limit: 10}, 10)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	require.Equal(t, "future", hits[0].ItemID)
	expired, err := repo.ExpireOverdue(t.Context(), scope)
	require.NoError(t, err)
	require.EqualValues(t, 1, expired)
	kept, err := repo.GetItem(t.Context(), scope, "future")
	require.NoError(t, err)
	require.Equal(t, "active", kept.Status)
	ended, err := repo.GetItem(t.Context(), scope, "past")
	require.NoError(t, err)
	require.Equal(t, "archived", ended.Status)
}
