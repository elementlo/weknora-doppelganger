package memory

import (
	"context"
	"crypto/hmac"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type externalVectorKey struct{}
type externalQueryVector struct {
	model  string
	vector []float32
}

func validExternalVector(v []float32) bool {
	if len(v) < 1 || len(v) > 4096 {
		return false
	}
	var norm float64
	for _, x := range v {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return false
		}
		norm += float64(x) * float64(x)
	}
	return norm > 0
}
func vectorSources(item *types.MemoryItem) []types.ExternalMemorySource {
	return []types.ExternalMemorySource{{ID: item.ID, Revision: item.UpdatedAt.UTC().Format(time.RFC3339Nano), Hash: types.MemoryItemKey(item.Topic, item.Content), Content: embeddableText(item, nil)}}
}
func (s *Service) PrepareVectors(ctx context.Context, request types.ExternalVectorRequest) ([]types.ExternalVectorItem, error) {
	scope, cfg, ok := s.enabledScope(ctx)
	if !ok || request.ModelID == "" || request.ModelID != cfg.EmbeddingModelID {
		return nil, fmt.Errorf("vector model unavailable")
	}
	items, err := s.repo.ItemsMissingEmbeddings(ctx, scope, request.ModelID, 20)
	if err != nil {
		return nil, err
	}
	result := []types.ExternalVectorItem{}
	expires := time.Now().Add(2 * time.Minute).UnixMilli()
	for _, item := range items {
		sources := vectorSources(item)
		sources[0].Role = request.ModelID
		result = append(result, types.ExternalVectorItem{ItemID: item.ID, Text: sources[0].Content, Ticket: strconv.FormatInt(expires, 10) + "." + ticketMAC(scope, sources, false, expires)})
	}
	return result, nil
}
func (s *Service) CommitVector(ctx context.Context, request types.ExternalVectorRequest) error {
	scope, cfg, ok := s.enabledScope(ctx)
	if !ok || request.ModelID == "" || request.ModelID != cfg.EmbeddingModelID || !validExternalVector(request.Vector) || (cfg.HostEmbeddingDimensions > 0 && len(request.Vector) != cfg.HostEmbeddingDimensions) {
		return fmt.Errorf("invalid vector")
	}
	item, err := s.repo.GetItem(ctx, scope, request.ItemID)
	if err != nil || item == nil || item.Status != types.MemoryStatusActive {
		return fmt.Errorf("item unavailable")
	}
	parts := strings.Split(request.Ticket, ".")
	if len(parts) != 2 {
		return fmt.Errorf("invalid ticket")
	}
	expires, err := strconv.ParseInt(parts[0], 10, 64)
	sources := vectorSources(item)
	sources[0].Role = request.ModelID
	if err != nil || expires <= time.Now().UnixMilli() || !hmac.Equal([]byte(parts[1]), []byte(ticketMAC(scope, sources, false, expires))) {
		return fmt.Errorf("stale vector ticket")
	}
	return s.repo.UpsertItemEmbedding(ctx, scope, &types.MemoryItemEmbedding{ItemID: item.ID, SourceContent: item.Content, SourceTopic: item.Topic, ModelID: request.ModelID, Dims: len(request.Vector), Vector: types.EncodeEmbedding(request.Vector)})
}
func (s *Service) SearchExternalVector(ctx context.Context, request types.ExternalVectorRequest) (interfaces.MemorySearchResult, error) {
	scope, cfg, ok := s.enabledScope(ctx)
	if !ok || request.ModelID == "" || request.ModelID != cfg.EmbeddingModelID || !validExternalVector(request.Vector) || (cfg.HostEmbeddingDimensions > 0 && len(request.Vector) != cfg.HostEmbeddingDimensions) || strings.TrimSpace(request.Query) == "" || len([]rune(request.Query)) > 2000 || request.Limit < 1 || request.Limit > 20 {
		return interfaces.MemorySearchResult{}, fmt.Errorf("invalid vector search")
	}
	// Search uses the existing lexical/vector fusion and active/expiry filtering.
	result := s.SearchMemory(context.WithValue(ctx, externalVectorKey{}, externalQueryVector{request.ModelID, request.Vector}), request.Query, request.Limit)
	current, _, enabled := s.enabledScope(ctx)
	if !enabled || *current.Generation != *scope.Generation {
		return interfaces.MemorySearchResult{}, fmt.Errorf("generation changed")
	}
	return result, nil
}
