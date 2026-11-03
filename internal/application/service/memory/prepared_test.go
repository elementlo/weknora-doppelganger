package memory

import (
	"context"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestPreparedExternalHasNoModelAndFencesContentSubjectKindAndClear(t *testing.T) {
	svc, tenants, _, models, _ := newExtractionHarness(t)
	ctx := enabledCtx(t, tenants, 7, "alice")
	tenants.set(7, &types.MemoryConfig{Enabled: true, HostInferenceOnly: true, ExtractModelID: "unavailable"})
	req := types.ExternalMemoryInput{Sources: []types.ExternalMemorySource{{ID: "s", Revision: "1", Hash: strings.Repeat("a", 64), Role: "user", Content: "synthetic preference"}}}
	plan, err := svc.PrepareExternal(ctx, req, false)
	require.NoError(t, err)
	require.NotEmpty(t, plan.Prompt)
	require.Equal(t, "complete", plan.ExistingCoverage)
	resolution := types.ExternalMemoryResolution{Sources: req.Sources, Ticket: plan.Ticket, Content: `{"memories":[{"action":"add","kind":"preference","content":"先结论","source":1,"importance":3}]}`}
	result, err := svc.ResolveExternal(ctx, resolution, false)
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
	altered := resolution
	altered.Sources = append([]types.ExternalMemorySource{}, resolution.Sources...)
	altered.Sources[0].Content = "changed"
	_, err = svc.ResolveExternal(ctx, altered, false)
	require.Error(t, err)
	_, err = svc.ResolveExternal(ctx, resolution, true)
	require.Error(t, err)
	other := enabledCtx(t, tenants, 7, "bob")
	// enabledCtx sets defaults; restore host mode for both principals.
	tenants.set(7, &types.MemoryConfig{Enabled: true, HostInferenceOnly: true, ExtractModelID: "unavailable"})
	_, err = svc.ResolveExternal(other, resolution, false)
	require.Error(t, err)
	_, err = svc.ProposeExternal(ctx, req, false)
	require.Error(t, err)
	require.Empty(t, models.calls)
	_, err = svc.Clear(ctx)
	require.NoError(t, err)
	_, err = svc.ResolveExternal(ctx, resolution, false)
	require.Error(t, err)
}
func TestHostVectorRejectsMalformedAndWrongDimensionsWithoutInference(t *testing.T) {
	svc, tenants, _, _, _ := newExtractionHarness(t)
	ctx := enabledCtx(t, tenants, 7, "alice")
	tenants.set(7, &types.MemoryConfig{Enabled: true, HostInferenceOnly: true, EmbeddingModelID: "host", HostEmbeddingDimensions: 2})
	_, err := svc.SearchExternalVector(ctx, types.ExternalVectorRequest{ModelID: "host", Query: "q", Limit: 5, Vector: []float32{1}})
	require.Error(t, err)
	_, err = svc.SearchExternalVector(ctx, types.ExternalVectorRequest{ModelID: "other", Query: "q", Limit: 5, Vector: []float32{1, 0}})
	require.Error(t, err)
	_, ok := svc.embedder(context.Background(), &types.MemoryConfig{Enabled: true, HostInferenceOnly: true, EmbeddingModelID: "host"})
	require.False(t, ok)
}
func TestExternalVectorTicketScopeContentAndClearFencing(t *testing.T) {
	svc, tenants, _, _, _ := newExtractionHarness(t)
	ctx := enabledCtx(t, tenants, 7, "alice")
	host := &types.MemoryConfig{Enabled: true, HostInferenceOnly: true, EmbeddingModelID: "host", HostEmbeddingDimensions: 2}
	tenants.set(7, host)
	item, err := svc.Remember(ctx, types.MemoryItem{Kind: types.MemoryKindPreference, Content: "synthetic vector note", Importance: 3})
	require.NoError(t, err)
	plans, err := svc.PrepareVectors(ctx, types.ExternalVectorRequest{ModelID: "host"})
	require.NoError(t, err)
	require.Len(t, plans, 1)
	request := types.ExternalVectorRequest{ModelID: "host", ItemID: item.ID, Ticket: plans[0].Ticket, Vector: []float32{1, 0}}
	require.NoError(t, svc.CommitVector(ctx, request))
	result, err := svc.SearchExternalVector(ctx, types.ExternalVectorRequest{ModelID: "host", Query: "completely unrelated tokens", Limit: 5, Vector: []float32{1, 0}})
	require.NoError(t, err)
	require.Equal(t, 1, result.VectorHits)
	other := enabledCtx(t, tenants, 7, "bob")
	tenants.set(7, host)
	require.Error(t, svc.CommitVector(other, request))
	_, err = svc.Clear(ctx)
	require.NoError(t, err)
	require.Error(t, svc.CommitVector(ctx, request))
}
