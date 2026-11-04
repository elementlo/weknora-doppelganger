package handler

import (
	"context"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
)

type publicationStub struct {
	interfaces.MemoryService
	calls int
}

func (s *publicationStub) PublicationContext(context.Context) (string, error) {
	return "generation", nil
}
func (s *publicationStub) ReadPublication(context.Context, string) (types.MemoryPublicationResult, error) {
	return types.MemoryPublicationResult{State: "absent"}, nil
}
func (s *publicationStub) PublishMemory(context.Context, types.MemoryPublicationRequest) (types.MemoryPublicationResult, error) {
	s.calls++
	return types.MemoryPublicationResult{}, nil
}
func TestPublicationHandlerStrictBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &publicationStub{}
	handler := NewMemoryHandler(service)
	router := gin.New()
	router.POST("/publish", handler.PublishMemory)
	for _, body := range []string{`{"subject_id":"other"}`, `{} {}`, strings.Repeat(" ", 8193) + `{}`, `{broken`} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest("POST", "/publish", strings.NewReader(body)))
		require.Equal(t, 400, rec.Code)
	}
	require.Zero(t, service.calls)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("POST", "/publish", strings.NewReader(`{}`)))
	require.Equal(t, 200, rec.Code)
	require.Equal(t, 1, service.calls)
}
