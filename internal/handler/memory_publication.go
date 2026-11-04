package handler

import (
	"context"
	"encoding/json"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
)

type memoryPublisher interface {
	PublicationContext(context.Context) (string, error)
	PublishMemory(context.Context, types.MemoryPublicationRequest) (types.MemoryPublicationResult, error)
	ReadPublication(context.Context, string) (types.MemoryPublicationResult, error)
}

func (h *MemoryHandler) PublicationContext(c *gin.Context) {
	svc, ok := h.memoryService.(memoryPublisher)
	if !ok {
		c.JSON(501, gin.H{"success": false})
		return
	}
	generation, err := svc.PublicationContext(c.Request.Context())
	if err != nil {
		c.JSON(422, gin.H{"success": false})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": gin.H{"generation": generation}})
}
func (h *MemoryHandler) ReadPublication(c *gin.Context) {
	svc, ok := h.memoryService.(memoryPublisher)
	if !ok {
		c.JSON(501, gin.H{"success": false})
		return
	}
	result, err := svc.ReadPublication(c.Request.Context(), c.Param("key"))
	if err != nil {
		c.JSON(422, gin.H{"success": false})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": result})
}
func (h *MemoryHandler) PublishMemory(c *gin.Context) {
	svc, ok := h.memoryService.(memoryPublisher)
	if !ok {
		c.JSON(501, gin.H{"success": false})
		return
	}
	var req types.MemoryPublicationRequest
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 8192))
	decoder.DisallowUnknownFields()
	var trailing interface{}
	if err := decoder.Decode(&req); err != nil {
		c.JSON(400, gin.H{"success": false})
		return
	}
	if err := decoder.Decode(&trailing); err != io.EOF {
		c.JSON(400, gin.H{"success": false})
		return
	}
	result, err := svc.PublishMemory(c.Request.Context(), req)
	if err != nil {
		c.JSON(409, gin.H{"success": false, "error": "memory_publication_refused"})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": result})
}

func (h *MemoryHandler) ForgetPublication(c *gin.Context) {
	svc, ok := h.memoryService.(interface {
		ForgetPublication(context.Context, string, string) error
	})
	if !ok {
		c.JSON(501, gin.H{"success": false})
		return
	}
	var body struct {
		PayloadHash string `json:"payload_hash"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 1024))
	decoder.DisallowUnknownFields()
	var trailing interface{}
	if err := decoder.Decode(&body); err != nil {
		c.JSON(400, gin.H{"success": false})
		return
	}
	if err := decoder.Decode(&trailing); err != io.EOF {
		c.JSON(400, gin.H{"success": false})
		return
	}
	if err := svc.ForgetPublication(c.Request.Context(), c.Param("key"), body.PayloadHash); err != nil {
		c.JSON(409, gin.H{"success": false})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": gin.H{"erased": true}})
}
