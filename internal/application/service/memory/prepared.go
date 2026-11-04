package memory

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Tickets have a process lifetime and a short deadline. A restart requires a
// fresh prepare; no source transcript is persisted and no callback URL is used.
var preparedKey = func() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	return key
}()

func externalSegment(sources []types.ExternalMemorySource, experience bool) (transcriptSegment, error) {
	segment := transcriptSegment{sessionID: "external:harness"}
	if len(sources) < 1 || len(sources) > 20 {
		return segment, fmt.Errorf("invalid sources")
	}
	seen := map[string]bool{}
	for _, source := range sources {
		if source.ID == "" || len(source.ID) > 96 || seen[source.ID] || source.Revision == "" || len(source.Revision) > 256 || !externalHash.MatchString(source.Hash) || strings.TrimSpace(source.Content) == "" || utf8.RuneCountInString(source.Content) > 1000 {
			return segment, fmt.Errorf("invalid sources")
		}
		if (!experience && source.Role != "user") || (experience && source.Role != "verified_effect") {
			return segment, fmt.Errorf("invalid source role")
		}
		seen[source.ID] = true
		segment.lines = append(segment.lines, transcriptLine{sessionID: segment.sessionID, messageID: source.ID, at: time.Now(), content: source.Content})
	}
	return segment, nil
}
func ticketMAC(scope interfaces.MemoryScope, sources []types.ExternalMemorySource, experience bool, expires int64) string {
	encoded, _ := json.Marshal(struct {
		Tenant              uint64
		Subject, Generation string
		Sources             []types.ExternalMemorySource
		Experience          bool
		Expires             int64
	}{scope.TenantID, scope.SubjectID, *scope.Generation, sources, experience, expires})
	mac := hmac.New(sha256.New, preparedKey)
	mac.Write(encoded)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (s *Service) PrepareExternal(ctx context.Context, request types.ExternalMemoryInput, experience bool) (*types.ExternalMemoryPlan, error) {
	scope, cfg, ok := s.enabledScope(ctx)
	if !ok {
		return nil, fmt.Errorf("memory disabled")
	}
	segment, err := externalSegment(request.Sources, experience)
	if err != nil {
		return nil, err
	}
	// The host principal can serve several product scopes. Without a host approval
	// allowlist, subject-wide notes/topics are not authorized extraction context.
	if cfg.HostInferenceOnly {
		instructions := cfg.ExtractInstructions
		if experience {
			instructions += "\nOnly propose reusable task procedures supported by the supplied verified effects. Include applicability and failure limits. Do not infer user identity or personality."
		}
		expires := time.Now().Add(2 * time.Minute).UnixMilli()
		return &types.ExternalMemoryPlan{Ticket: strconv.FormatInt(expires, 10) + "." + ticketMAC(scope, request.Sources, experience, expires), ExpiresAt: expires,
			System: extractionSystemPrompt, Prompt: buildExtractionPrompt(segment, nil, nil, nil, instructions), Schema: json.RawMessage(extractionSchema), CompletionLimits: []int{extractBudgetTokens, extractBudgetRetryTokens}, ExistingCoverage: "host_sources_only"}, nil
	}
	// This boundary MUST be model-free, including hidden embedding selection.
	// Existing context is the bounded importance prefix; disclose coverage.
	existing, err := s.repo.ListActiveByKinds(ctx, scope, types.MemoryKinds, extractRelevantCandidates+1)
	if err != nil {
		return nil, err
	}
	coverage := "complete"
	if len(existing) > extractRelevantCandidates {
		coverage = "importance_prefix"
		existing = existing[:extractRelevantCandidates]
	}
	forgotten, err := s.repo.ListTombstones(ctx, scope, 30)
	if err != nil {
		return nil, err
	}
	topics, err := s.repo.TopTopics(ctx, scope, topicCandidateLimit)
	if err != nil {
		return nil, err
	}
	instructions := cfg.ExtractInstructions
	if experience {
		instructions += "\nOnly propose reusable task procedures supported by the supplied verified effects. Include applicability and failure limits. Do not infer user identity or personality."
	}
	expires := time.Now().Add(2 * time.Minute).UnixMilli()
	return &types.ExternalMemoryPlan{Ticket: strconv.FormatInt(expires, 10) + "." + ticketMAC(scope, request.Sources, experience, expires), ExpiresAt: expires,
		System: extractionSystemPrompt, Prompt: buildExtractionPrompt(segment, existing, forgotten, topics, instructions), Schema: json.RawMessage(extractionSchema), CompletionLimits: []int{extractBudgetTokens, extractBudgetRetryTokens}, ExistingCoverage: coverage}, nil
}
func (s *Service) ResolveExternal(ctx context.Context, request types.ExternalMemoryResolution, experience bool) (*types.ExternalMemoryResult, error) {
	scope, _, ok := s.enabledScope(ctx)
	if !ok {
		return nil, fmt.Errorf("memory disabled")
	}
	if _, err := externalSegment(request.Sources, experience); err != nil {
		return nil, err
	}
	parts := strings.Split(request.Ticket, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid ticket")
	}
	expires, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || expires <= time.Now().UnixMilli() || expires > time.Now().Add(2*time.Minute).UnixMilli() || !hmac.Equal([]byte(parts[1]), []byte(ticketMAC(scope, request.Sources, experience, expires))) {
		return nil, fmt.Errorf("stale ticket")
	}
	if len(request.Content) > 64*1024 {
		return nil, fmt.Errorf("output too large")
	}
	parsed, err := parseExtractionResponse(request.Content)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := &types.ExternalMemoryResult{Candidates: []types.ExternalMemoryCandidate{}}
	for _, decision := range parsed.Memories {
		if len(result.Candidates) >= extractMaxItemsPerRun {
			break
		}
		if (decision.Action != "add" && decision.Action != "update") || decision.Source == nil || *decision.Source < 1 || *decision.Source > len(request.Sources) {
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
		valid := false
		for _, k := range types.MemoryKinds {
			if kind == k {
				valid = true
			}
		}
		if !valid {
			continue
		}
		source := request.Sources[*decision.Source-1]
		source.Content = ""
		result.Candidates = append(result.Candidates, types.ExternalMemoryCandidate{Kind: kind, Content: content, Importance: types.ClampMemoryImportance(decision.Importance), Source: source})
	}
	// No remote effect; recheck generation after reads before returning candidates.
	current, _, enabled := s.enabledScope(ctx)
	if !enabled || current.Generation == nil || *current.Generation != *scope.Generation {
		return nil, fmt.Errorf("generation changed")
	}
	return result, nil
}
