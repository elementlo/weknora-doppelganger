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

type preparedMemory interface {
	PrepareExternal(context.Context, types.ExternalMemoryInput, bool) (*types.ExternalMemoryPlan, error)
	ResolveExternal(context.Context, types.ExternalMemoryResolution, bool) (*types.ExternalMemoryResult, error)
}

func (h *MemoryHandler) PrepareExtraction(c *gin.Context) { h.prepared(c, false, false) }
func (h *MemoryHandler) ResolveExtraction(c *gin.Context) { h.prepared(c, false, true) }
func (h *MemoryHandler) PrepareExperience(c *gin.Context) { h.prepared(c, true, false) }
func (h *MemoryHandler) ResolveExperience(c *gin.Context) { h.prepared(c, true, true) }
func (h *MemoryHandler) prepared(c *gin.Context, experience, resolve bool) {
	service, ok := h.memoryService.(preparedMemory)
	if !ok {
		c.JSON(501, gin.H{"success": false, "error": "external_memory_unavailable"})
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 256*1024))
	decoder.DisallowUnknownFields()
	input := types.ExternalMemoryInput{}
	resolution := types.ExternalMemoryResolution{}
	var target interface{} = &input
	if resolve {
		target = &resolution
	}
	var trailing interface{}
	if err := decoder.Decode(target); err != nil {
		c.JSON(400, gin.H{"success": false, "error": "invalid_external_memory"})
		return
	}
	if err := decoder.Decode(&trailing); err != io.EOF {
		c.JSON(400, gin.H{"success": false, "error": "invalid_external_memory"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	var data interface{}
	var err error
	if resolve {
		data, err = service.ResolveExternal(ctx, resolution, experience)
	} else {
		data, err = service.PrepareExternal(ctx, input, experience)
	}
	if err != nil {
		c.JSON(422, gin.H{"success": false, "error": "external_memory_preparation_failed"})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": data})
}
