package memory

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Tencent/WeKnora/internal/types"
)

var externalHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

// ProposeExternal reuses extraction, scoped retrieval, tombstones and model
// selection, but never applies model mutations. The host must persist the
// returned candidates/lineage, obtain domain approval and then use CRUD.
// This prevents late/uncertain HTTP results from changing stable memory.
func (s *Service) ProposeExternal(ctx context.Context, request types.ExternalMemoryInput, experience bool) (*types.ExternalMemoryResult, error) {
	scope, cfg, ok := s.enabledScope(ctx)
	if !ok || cfg.HostInferenceOnly {
		return nil, fmt.Errorf("external memory disabled")
	}
	if len(request.Sources) < 1 || len(request.Sources) > 20 {
		return nil, fmt.Errorf("invalid source count")
	}
	segment := transcriptSegment{sessionID: "external:harness"}
	seen := map[string]bool{}
	for _, source := range request.Sources {
		if source.ID == "" || len(source.ID) > 96 || seen[source.ID] || source.Revision == "" || len(source.Revision) > 256 || !externalHash.MatchString(source.Hash) || strings.TrimSpace(source.Content) == "" || utf8.RuneCountInString(source.Content) > 1000 {
			return nil, fmt.Errorf("invalid external source")
		}
		if (!experience && source.Role != "user") || (experience && source.Role != "verified_effect") {
			return nil, fmt.Errorf("unsupported external source role")
		}
		seen[source.ID] = true
		segment.lines = append(segment.lines, transcriptLine{sessionID: segment.sessionID, messageID: source.ID, at: time.Now(), content: source.Content})
	}
	existing, err := s.relevantExisting(ctx, scope, cfg, segment)
	if err != nil {
		return nil, err
	}
	forgotten, err := s.repo.ListTombstones(ctx, scope, 30)
	if err != nil {
		return nil, err
	}
	topics, err := s.repo.TopTopics(ctx, scope, topicCandidateLimit)
	if err != nil {
		return nil, err
	}
	config := *cfg
	if experience {
		config.ExtractInstructions += "\nOnly propose reusable task procedures supported by the supplied verified effects. Include applicability and failure limits. Do not infer user identity or personality."
	}
	parsed, err := s.callExtractionModel(ctx, &config, types.MemoryExtractPayload{}, segment, existing, forgotten, topics)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Check switches and clear generation again after the potentially slow model.
	current, _, enabled := s.enabledScope(ctx)
	if !enabled || current.Generation == nil || scope.Generation == nil || *current.Generation != *scope.Generation {
		return nil, fmt.Errorf("memory generation changed")
	}
	result := &types.ExternalMemoryResult{Candidates: []types.ExternalMemoryCandidate{}, ModelCallsReserved: 2, CompletionTokensReserved: extractBudgetTokens + extractBudgetRetryTokens}
	for _, decision := range parsed.Memories {
		if len(result.Candidates) >= extractMaxItemsPerRun {
			break
		}
		if decision.Action != "add" && decision.Action != "update" {
			continue
		}
		if decision.Source == nil || *decision.Source < 1 || *decision.Source > len(request.Sources) {
			continue
		}
		content := types.SanitizeMemoryContent(decision.Content)
		if content == "" {
			continue
		}
		kind := decision.Kind
		if experience {
			kind = "task"
		}
		source := request.Sources[*decision.Source-1]
		source.Content = "" // Return lineage, never echo the raw transcript.
		result.Candidates = append(result.Candidates, types.ExternalMemoryCandidate{Kind: kind, Content: content, Importance: types.ClampMemoryImportance(decision.Importance), Source: source})
	}
	return result, nil
}
