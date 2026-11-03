package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

// SearchMemory exposes the existing scoped recall engine without starting a
// chat session or an Agent loop. Principal and tenant come from middleware.
func (h *MemoryHandler) SearchMemory(c *gin.Context) {
	var request struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid_memory_search"})
		return
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF || strings.TrimSpace(request.Query) == "" || utf8.RuneCountInString(request.Query) > 2000 || request.Limit < 1 || request.Limit > 20 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid_memory_search"})
		return
	}
	result := h.memoryService.SearchMemory(c.Request.Context(), request.Query, request.Limit)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"available": result.Available, "items": result.Items}})
}
