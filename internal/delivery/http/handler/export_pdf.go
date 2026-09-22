package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"api-report-nexus/internal/delivery/http/response"
	domain "api-report-nexus/internal/domain/pdf"
	"api-report-nexus/internal/infrastructure/gotenberg"
	"api-report-nexus/internal/usecase/exportpdf"

	"github.com/gin-gonic/gin"
)

type ExportPDF struct {
	Renderer exportpdf.Renderer
	Timeout  time.Duration
}

func (h ExportPDF) Export(c *gin.Context) {
	var req struct {
		domain.Request
		HTMLContent string `json:"htmlContent" binding:"required"`
		FileName    string `json:"fileName" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	req.Request.HTMLContent, req.Request.FileName = req.HTMLContent, req.FileName

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.Timeout)
	defer cancel()

	name, data, err := exportpdf.HTML(ctx, h.Renderer, req.Request)
	if err != nil {
		pdfFail(c, err, "Error generating PDF file")
		return
	}
	c.Header("Content-Disposition", "attachment; filename="+name+".pdf")
	c.Data(http.StatusOK, response.MIMEPDF, data)
}

// A busy Gotenberg gets a retryable 503; anything else is a 500.
func pdfFail(c *gin.Context, err error, msg string) {
	slog.Error("pdf failed", "err", err, "path", c.Request.URL.Path)
	if errors.Is(err, gotenberg.ErrBusy) {
		c.Header("Retry-After", "5")
		response.Fail(c, http.StatusServiceUnavailable, "Server busy, please retry")
		return
	}
	response.Fail(c, http.StatusInternalServerError, msg)
}
