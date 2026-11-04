package memory

import (
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func TestPublicationAtomicReceiptAndForget(t *testing.T) {
	svc, db, tenants := newMemoryHarness(t)
	require.NoError(t, db.AutoMigrate(&types.MemoryPublication{}))
	ctx := enabledCtx(t, tenants, 1, "publisher")
	cfg := tenants.configs[1]
	cfg.HostInferenceOnly = true
	generation, err := svc.PublicationContext(ctx)
	require.NoError(t, err)
	req := types.MemoryPublicationRequest{Key: strings.Repeat("a", 64), PayloadHash: strings.Repeat("b", 64), Generation: generation, Kind: "preference", Content: "合成验收喜欢简洁说明", Importance: 3, ExpiresAt: time.Now().Add(time.Hour).UTC().Truncate(time.Millisecond)}
	require.NoError(t, db.Exec(`CREATE TRIGGER fail_receipt BEFORE INSERT ON memory_publications BEGIN SELECT RAISE(ABORT,'receipt_failure'); END`).Error)
	_, err = svc.PublishMemory(ctx, req)
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Model(&types.MemoryItem{}).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Exec("DROP TRIGGER fail_receipt").Error)
	result, err := svc.PublishMemory(ctx, req)
	require.NoError(t, err)
	require.Equal(t, "active", result.State)
	replay, err := svc.PublishMemory(ctx, req)
	require.NoError(t, err)
	require.Equal(t, result.Receipt.ItemID, replay.Receipt.ItemID)
	changed := req
	changed.Content = "另一条合成说明"
	_, err = svc.PublishMemory(ctx, changed)
	require.Error(t, err)
	foreign := enabledCtx(t, tenants, 2, "publisher")
	foreignResult, err := svc.ReadPublication(foreign, req.Key)
	require.NoError(t, err)
	require.Equal(t, "absent", foreignResult.State)
	duplicate := req
	duplicate.Key = strings.Repeat("c", 64)
	_, err = svc.PublishMemory(ctx, duplicate)
	require.Error(t, err)
	require.NoError(t, svc.DeleteItem(ctx, result.Receipt.ItemID))
	deleted, err := svc.ReadPublication(ctx, req.Key)
	require.NoError(t, err)
	require.Equal(t, "unavailable", deleted.State)
	replay, err = svc.PublishMemory(ctx, req)
	require.NoError(t, err)
	require.Nil(t, replay.Item)
	_, err = svc.Clear(ctx)
	require.NoError(t, err)
	cleared, err := svc.ReadPublication(ctx, req.Key)
	require.NoError(t, err)
	require.NotNil(t, cleared.Receipt)
	require.Equal(t, "unavailable", cleared.State)
	stale := req
	stale.Key = strings.Repeat("d", 64)
	stale.Content = "新的合成内容"
	_, err = svc.PublishMemory(ctx, stale)
	require.Error(t, err)
}
