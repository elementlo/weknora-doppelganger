package handler

import (
	"context"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type standaloneMemorySearch struct {
	interfaces.MemoryService
	calls     int
	available bool
}

func (s *standaloneMemorySearch) SearchMemory(ctx context.Context, query string, limit int) interfaces.MemorySearchResult {
	s.calls++
	return interfaces.MemorySearchResult{Available: s.available}
}
func TestStandaloneMemorySearch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		body      string
		code      int
		available bool
	}{
		{`{"query":"preference","limit":5}`, 200, true},
		{`{"query":"preference","limit":5}`, 200, false},
		{`{"query":"x","limit":1,"subject_id":"other"}`, 400, true},
		{`{"query":"x","limit":21}`, 400, true},
		{`{"query":" ","limit":1}`, 400, true},
		{`{"query":"x","limit":1} {}`, 400, true},
		{`{"query":"x","limit":0}`, 400, true},
		{`{"query":"` + strings.Repeat("x", 17000) + `","limit":1}`, 400, true},
	} {
		service := &standaloneMemorySearch{available: tc.available}
		router := gin.New()
		handler := NewMemoryHandler(service)
		router.POST("/memory/search", handler.SearchMemory)
		response := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/memory/search", strings.NewReader(tc.body))
		router.ServeHTTP(response, req)
		if response.Code != tc.code {
			t.Fatalf("status=%d expected=%d", response.Code, tc.code)
		}
		expected := 0
		if tc.code == 200 {
			expected = 1
		}
		if service.calls != expected {
			t.Fatalf("engine calls=%d expected=%d", service.calls, expected)
		}
		if tc.code == 200 && !strings.Contains(response.Body.String(), `"available":`) {
			t.Fatal("missing availability")
		}
	}
}
