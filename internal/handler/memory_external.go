package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

type externalMemoryProposer interface {
	ProposeExternal(context.Context, types.ExternalMemoryInput, bool) (*types.ExternalMemoryResult, error)
}

func (h *MemoryHandler) ExtractExternal(c *gin.Context) { h.proposeExternal(c, false) }
func (h *MemoryHandler) DistillExternal(c *gin.Context) { h.proposeExternal(c, true) }
func (h *MemoryHandler) proposeExternal(c *gin.Context, experience bool) {
	var request types.ExternalMemoryInput
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 128*1024))
	decoder.DisallowUnknownFields()
	var trailing interface{}
	if err := decoder.Decode(&request); err != nil {
		c.JSON(400, gin.H{"success": false, "error": "invalid_external_memory"})
		return
	}
	if err := decoder.Decode(&trailing); err != io.EOF {
		c.JSON(400, gin.H{"success": false, "error": "invalid_external_memory"})
		return
	}
	service, ok := h.memoryService.(externalMemoryProposer)
	if !ok {
		c.JSON(501, gin.H{"success": false, "error": "external_memory_unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	result, err := service.ProposeExternal(ctx, request, experience)
	if err != nil {
		c.JSON(422, gin.H{"success": false, "error": "external_memory_proposal_failed"})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": result})
}
