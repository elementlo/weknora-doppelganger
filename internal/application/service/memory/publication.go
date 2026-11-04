package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"regexp"
	"time"
)

type publicationRepository interface {
	GetPublication(context.Context, interfaces.MemoryScope, string) (*types.MemoryPublication, error)
	PublishMemory(context.Context, interfaces.MemoryScope, *types.MemoryPublication, *types.MemoryItem, int) (*types.MemoryPublication, error)
}

var publicationHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (s *Service) PublicationContext(ctx context.Context) (string, error) {
	scope, _, ok := s.enabledScope(ctx)
	if !ok {
		return "", ErrMemoryDisabled
	}
	return *scope.Generation, nil
}
func (s *Service) ReadPublication(ctx context.Context, key string) (types.MemoryPublicationResult, error) {
	scope, err := ResolveScope(ctx)
	if err != nil || !publicationHash.MatchString(key) {
		return types.MemoryPublicationResult{}, fmt.Errorf("invalid publication")
	}
	repo, ok := s.repo.(publicationRepository)
	if !ok {
		return types.MemoryPublicationResult{}, fmt.Errorf("unsupported publication")
	}
	receipt, err := repo.GetPublication(ctx, scope, key)
	if err != nil {
		return types.MemoryPublicationResult{}, err
	}
	result := types.MemoryPublicationResult{Receipt: receipt, State: "absent"}
	if receipt == nil {
		return result, nil
	}
	result.State = "unavailable"
	item, err := s.repo.GetItem(ctx, scope, receipt.ItemID)
	if err != nil {
		return result, err
	}
	_, _, enabled := s.enabledScope(ctx)
	if enabled && item != nil && item.Status == types.MemoryStatusActive && (item.ExpiresAt == nil || item.ExpiresAt.After(time.Now())) {
		result.Item = item
		result.State = "active"
	}
	return result, nil
}
func (s *Service) PublishMemory(ctx context.Context, req types.MemoryPublicationRequest) (types.MemoryPublicationResult, error) {
	scope, cfg, ok := s.enabledScope(ctx)
	if !ok || !cfg.HostInferenceOnly {
		return types.MemoryPublicationResult{}, ErrMemoryDisabled
	}
	if !publicationHash.MatchString(req.Key) || !publicationHash.MatchString(req.PayloadHash) || req.Generation != *scope.Generation || !types.IsValidMemoryKind(req.Kind) || req.Importance < 1 || req.Importance > 5 || !req.ExpiresAt.After(time.Now()) || req.ExpiresAt.After(time.Now().Add(365*24*time.Hour)) || types.SanitizeMemoryContent(req.Content) != req.Content || req.Content == "" {
		return types.MemoryPublicationResult{}, fmt.Errorf("invalid publication")
	}
	if _, changed := types.RedactSensitive(req.Content); changed {
		return types.MemoryPublicationResult{}, ErrSensitiveContent
	}
	repo, ok := s.repo.(publicationRepository)
	if !ok {
		return types.MemoryPublicationResult{}, fmt.Errorf("unsupported publication")
	}
	if _, err := s.repo.EnsureSubject(ctx, scope); err != nil {
		return types.MemoryPublicationResult{}, err
	}
	body, _ := json.Marshal(req)
	hash := sha256.Sum256(body)
	item := &types.MemoryItem{ID: uuid.NewString(), TenantID: scope.TenantID, SubjectID: scope.SubjectID, Kind: req.Kind, Content: req.Content, Importance: req.Importance, Origin: types.MemoryOriginManual, Status: types.MemoryStatusActive, NormalizedKey: types.MemoryItemKey("", req.Content), ValidFrom: time.Now(), ExpiresAt: &req.ExpiresAt}
	receipt := &types.MemoryPublication{Key: req.Key, PayloadHash: req.PayloadHash, RequestHash: hex.EncodeToString(hash[:]), ItemID: item.ID}
	if _, err := repo.PublishMemory(ctx, scope, receipt, item, cfg.EffectiveMaxItems()); err != nil {
		return types.MemoryPublicationResult{}, err
	}
	// Derived block is a repairable projection, outside the atomic business write.
	s.rebuildBlock(ctx, scope)
	return s.ReadPublication(ctx, req.Key)
}

func (s *Service) ForgetPublication(ctx context.Context, key, payloadHash string) error {
	scope, err := ResolveScope(ctx)
	if err != nil || !publicationHash.MatchString(key) || !publicationHash.MatchString(payloadHash) {
		return fmt.Errorf("invalid publication")
	}
	if _, err = s.repo.EnsureSubject(ctx, scope); err != nil {
		return err
	}
	repo, ok := s.repo.(interface {
		ForgetPublication(context.Context, interfaces.MemoryScope, string, string) error
	})
	if !ok {
		return fmt.Errorf("unsupported publication")
	}
	if err = repo.ForgetPublication(ctx, scope, key, payloadHash); err != nil {
		return err
	}
	s.rebuildBlock(ctx, scope)
	return nil
}
