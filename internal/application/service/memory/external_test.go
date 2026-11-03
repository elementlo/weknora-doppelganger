package memory

import (
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestExternalProposalReusesModelWithoutApplyingFacts(t *testing.T) {
	svc, tenants, _, models, _ := newExtractionHarness(t)
	ctx := enabledCtx(t, tenants, 7, "alice")
	tenants.set(7, &types.MemoryConfig{Enabled: true, ExtractModelID: "model"})
	models.response = `{"memories":[{"action":"add","kind":"fact","content":"合成偏好","source":1,"importance":3},{"action":"delete","source":1},{"action":"add","kind":"fact","content":"无来源","source":99}]}`
	request := types.ExternalMemoryInput{Sources: []types.ExternalMemorySource{{ID: "m1", Revision: "r1", Hash: strings.Repeat("a", 64), Role: "user", Content: "合成偏好"}}}
	result, err := svc.ProposeExternal(ctx, request, false)
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
	require.Equal(t, "r1", result.Candidates[0].Source.Revision)
	require.Empty(t, result.Candidates[0].Source.Content)
	_, total, err := svc.ListItems(ctx, "", 20, 0)
	require.NoError(t, err)
	require.Zero(t, total)
	request.Sources[0].Role = "verified_effect"
	result, err = svc.ProposeExternal(ctx, request, true)
	require.NoError(t, err)
	require.Equal(t, "task", result.Candidates[0].Kind)
	request.Sources[0].Role = "tool"
	_, err = svc.ProposeExternal(ctx, request, false)
	require.Error(t, err)
}
func TestExternalProposalRejectsMissingLineageBeforeModel(t *testing.T) {
	svc, tenants, _, _, _ := newExtractionHarness(t)
	ctx := enabledCtx(t, tenants, 7, "alice")
	for _, request := range []types.ExternalMemoryInput{{}, {Sources: []types.ExternalMemorySource{{ID: "m", Revision: "r", Hash: "invalid", Role: "user", Content: "data"}}}} {
		_, err := svc.ProposeExternal(ctx, request, false)
		require.Error(t, err)
	}
}
