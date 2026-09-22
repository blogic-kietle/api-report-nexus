package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"api-report-nexus/internal/delivery/http/response"
	"api-report-nexus/internal/usecase/exportpdf"

	domain "api-report-nexus/internal/domain/pdf"
	"api-report-nexus/internal/infrastructure/session"

	"github.com/gin-gonic/gin"
)

type PDFStream struct {
	Sessions  *session.Store
	Renderer  exportpdf.Renderer
	Timeout   time.Duration
	MaxChunk  int64
	ChunkSize int
}

func (h PDFStream) Init(c *gin.Context) {
	var req struct {
		SessionID   string `json:"sessionId"`
		FileName    string `json:"fileName"`
		TotalChunks int    `json:"totalChunks"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" || req.FileName == "" || req.TotalChunks == 0 {
		response.Fail(c, http.StatusBadRequest, "Missing required parameters")
		return
	}
	if err := h.Sessions.Init(req.SessionID, req.FileName, req.TotalChunks); errors.Is(err, session.ErrExists) {
		response.Fail(c, http.StatusConflict, "Session already exists")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Stream session initialized"})
}

func (h PDFStream) Chunk(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.MaxChunk)

	sessionID := c.PostForm("sessionId")
	index := c.PostForm("chunkIndex")
	total := c.PostForm("totalChunks")
	file, err := c.FormFile("chunk")
	if err != nil || sessionID == "" || index == "" || total == "" {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			response.Fail(c, http.StatusRequestEntityTooLarge, "Chunk too large")
			return
		}
		response.Fail(c, http.StatusBadRequest, "Invalid request parameters")
		return
	}
	n, err := strconv.Atoi(index)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "Invalid request parameters")
		return
	}
	opened, err := file.Open()
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "Error uploading chunk")
		return
	}
	defer func() { _ = opened.Close() }()
	data, err := io.ReadAll(opened)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "Error uploading chunk")
		return
	}
	if err := h.Sessions.AddChunk(sessionID, n, data); errors.Is(err, session.ErrNotFound) {
		response.Fail(c, http.StatusNotFound, "Session not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Chunk " + index + " uploaded successfully"})
}

func (h PDFStream) Finalize(c *gin.Context) {
	var req struct {
		SessionID string `json:"sessionId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" {
		response.Fail(c, http.StatusBadRequest, "Session ID is required")
		return
	}
	fileName, ok := h.Sessions.FileName(req.SessionID)
	if !ok {
		response.Fail(c, http.StatusNotFound, "Session not found")
		return
	}
	payload, err := h.Sessions.Assemble(req.SessionID)
	switch {
	case errors.Is(err, session.ErrNotFound):
		response.Fail(c, http.StatusNotFound, "Session not found")
		return
	case errors.Is(err, session.ErrIncomplete):
		response.Fail(c, http.StatusBadRequest, "Session is incomplete, not all chunks received")
		return
	case err != nil:
		response.Fail(c, http.StatusInternalServerError, "Failed to process session data")
		return
	}

	var pdfReq domain.Request
	if err := json.Unmarshal(payload, &pdfReq); err != nil {
		response.Fail(c, http.StatusInternalServerError, "Error generating PDF from stream")
		return
	}
	if pdfReq.FileName == "" {
		pdfReq.FileName = fileName
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.Timeout)
	defer cancel()

	var name string
	var data []byte
	if pdfReq.HTMLContent == "" && pdfReq.Datasets != nil {
		name, data, err = exportpdf.Datasets(ctx, h.Renderer, pdfReq, h.ChunkSize)
	} else {
		name, data, err = exportpdf.HTML(ctx, h.Renderer, pdfReq)
	}
	if err != nil {
		pdfFail(c, err, "Error generating PDF from stream")
		return
	}
	h.Sessions.Delete(req.SessionID)
	c.Header("Content-Disposition", "attachment; filename="+name+".pdf")
	c.Data(http.StatusOK, response.MIMEPDF, data)
}
