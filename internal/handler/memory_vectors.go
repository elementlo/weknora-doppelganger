package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

type externalVectors interface {
	PrepareVectors(context.Context, types.ExternalVectorRequest) ([]types.ExternalVectorItem, error)
	CommitVector(context.Context, types.ExternalVectorRequest) error
	SearchExternalVector(context.Context, types.ExternalVectorRequest) (interfaces.MemorySearchResult, error)
}

func (h *MemoryHandler) PrepareMemoryVectors(c *gin.Context) { h.vectorExternal(c, "prepare") }
func (h *MemoryHandler) CommitMemoryVector(c *gin.Context)   { h.vectorExternal(c, "commit") }
func (h *MemoryHandler) SearchMemoryVector(c *gin.Context)   { h.vectorExternal(c, "search") }
func (h *MemoryHandler) vectorExternal(c *gin.Context, action string) {
	service, ok := h.memoryService.(externalVectors)
	if !ok {
		c.JSON(501, gin.H{"success": false})
		return
	}
	var request types.ExternalVectorRequest
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 128*1024))
	decoder.DisallowUnknownFields()
	var trailing interface{}
	if err := decoder.Decode(&request); err != nil {
		c.JSON(400, gin.H{"success": false})
		return
	}
	if err := decoder.Decode(&trailing); err != io.EOF {
		c.JSON(400, gin.H{"success": false})
		return
	}
	var data interface{}
	var err error
	switch action {
	case "prepare":
		data, err = service.PrepareVectors(c.Request.Context(), request)
	case "commit":
		err = service.CommitVector(c.Request.Context(), request)
		data = gin.H{"stored": err == nil}
	case "search":
		var result interfaces.MemorySearchResult
		result, err = service.SearchExternalVector(c.Request.Context(), request)
		data = gin.H{"available": result.Available, "items": result.Items, "mode": result.RankingMode, "vector_hits": result.VectorHits, "vector_skip_reason": result.VectorSkipReason}
	}
	if err != nil {
		c.JSON(422, gin.H{"success": false, "error": "external_vector_failed"})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": data})
}
