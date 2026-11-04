package memory

import (
	"context"
	"crypto/hmac"
	"fmt"
	"math"
	"sort"
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
	// Product host supplies an approval allowlist before any content is ranked or returned.
	// A non-nil empty list means no accessible memories; nil retains the diagnostic API.
	if request.ItemIDs != nil {
		if len(request.ItemIDs) > 200 {
			return interfaces.MemorySearchResult{}, fmt.Errorf("allowlist capacity")
		}
		seen := map[string]bool{}
		ids := []string{}
		items := map[string]*types.MemoryItem{}
		now := time.Now()
		for _, id := range request.ItemIDs {
			if id == "" || seen[id] {
				return interfaces.MemorySearchResult{}, fmt.Errorf("invalid allowlist")
			}
			seen[id] = true
			item, err := s.repo.GetItem(ctx, scope, id)
			if err != nil {
				return interfaces.MemorySearchResult{}, err
			}
			if item != nil && item.Status == types.MemoryStatusActive && (item.ExpiresAt == nil || item.ExpiresAt.After(now)) {
				ids = append(ids, id)
				items[id] = item
			}
		}
		vectors, err := s.repo.ItemEmbeddings(ctx, scope, ids, request.ModelID)
		if err != nil {
			return interfaces.MemorySearchResult{}, err
		}
		type ranked struct {
			item  *types.MemoryItem
			score float64
		}
		rankedItems := []ranked{}
		for _, id := range ids {
			score := types.CosineSimilarity(request.Vector, vectors[id])
			lexical := strings.Contains(strings.ToLower(items[id].Content), strings.ToLower(strings.TrimSpace(request.Query)))
			if score >= minCosine || lexical {
				if lexical {
					score += 1
				}
				rankedItems = append(rankedItems, ranked{items[id], score})
			}
		}
		sort.Slice(rankedItems, func(i, j int) bool {
			if rankedItems[i].score == rankedItems[j].score {
				return rankedItems[i].item.ID < rankedItems[j].item.ID
			}
			return rankedItems[i].score > rankedItems[j].score
		})
		result := interfaces.MemorySearchResult{Available: true, RankingMode: "host_allowlist_hybrid", Items: []*types.MemoryItem{}}
		for _, item := range rankedItems {
			if len(result.Items) >= request.Limit {
				break
			}
			result.Items = append(result.Items, item.item)
		}
		current, _, enabled := s.enabledScope(ctx)
		if !enabled || *current.Generation != *scope.Generation {
			return interfaces.MemorySearchResult{}, fmt.Errorf("generation changed")
		}
		return result, nil
	}
	// Search uses the existing lexical/vector fusion and active/expiry filtering.
	result := s.SearchMemory(context.WithValue(ctx, externalVectorKey{}, externalQueryVector{request.ModelID, request.Vector}), request.Query, request.Limit)
	current, _, enabled := s.enabledScope(ctx)
	if !enabled || *current.Generation != *scope.Generation {
		return interfaces.MemorySearchResult{}, fmt.Errorf("generation changed")
	}
	return result, nil
}
