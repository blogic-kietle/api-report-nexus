package httpapi

import (
	"log/slog"
	"net/http"

	"api-report-nexus/assets"
	"api-report-nexus/internal/delivery/http/handler"
	"api-report-nexus/internal/delivery/http/response"
	"api-report-nexus/internal/domain/email"
	"api-report-nexus/internal/infrastructure/session"
	"api-report-nexus/internal/usecase/exportpdf"

	_ "net/http/pprof" //nolint:gosec // mounted only when DEBUG is set
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

type Deps struct {
	Version         string
	Debug           bool
	Log             *slog.Logger
	Now             func() time.Time
	PDF             exportpdf.Renderer
	PDFTimeout      time.Duration
	Sessions        *session.Store
	FinalizeTimeout time.Duration
	MaxChunk        int64
	ChunkSize       int
	Mail            email.Sender
	ClientURL       string
	// Notifier nil: statements are sent but not announced.
	Notifier handler.Notifier
}

// New builds the engine; the body size cap is http.MaxBytesHandler in main.go.
func New(d Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	// served via *http.Server so Gin never warns; trust nobody by default
	_ = r.SetTrustedProxies(nil)

	r.Use(requestID, requestLog(d.Log), gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, err any) {
		d.Log.Error("panic", "err", err, "stack", string(debug.Stack()), requestIDKey, c.GetString(requestIDKey))
		response.Fail(c, http.StatusInternalServerError, "Internal server error")
	}), cors)

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": serviceName, "version": d.Version})
	})
	r.GET("/api/openapi.yaml", func(c *gin.Context) { c.Data(http.StatusOK, "application/yaml", assets.OpenAPI) })
	r.GET("/api/docs", func(c *gin.Context) { c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(docsHTML)) })
	if d.Debug {
		r.GET("/debug/pprof/*any", gin.WrapH(http.DefaultServeMux))
	}

	excel := handler.ExportExcel{Now: d.Now}
	r.POST("/api/export-excel/sales-by-day-part", excel.SalesDayPart)

	if d.Mail != nil {
		mail := handler.SendMail{
			Sender: d.Mail, Now: d.Now, Log: d.Log, ClientURL: d.ClientURL,
			PDF: d.PDF, PDFTimeout: d.PDFTimeout, Notify: d.Notifier,
		}
		r.POST("/api/send-email/sales-summary", mail.SalesSummary)
		r.POST("/api/send-email/low-inventory-items", mail.LowInventory)
		r.POST("/api/send-email/item-sales-by-employee", mail.ItemSalesByEmployee)
		r.POST("/api/send-email/sales-by-modifier", mail.SalesByModifier)
		r.POST("/api/send-email/sales-by-day-part", mail.SalesDayPart)
		r.POST("/api/send-email/sales-by-category", mail.SalesByCategory)
		r.POST("/api/send-email/delivery-fee-report", mail.DeliveryFee)
	}

	r.POST("/api/export-pdf", handler.ExportPDF{Renderer: d.PDF, Timeout: d.PDFTimeout}.Export)
	stream := handler.PDFStream{
		Sessions: d.Sessions, Renderer: d.PDF,
		Timeout: d.FinalizeTimeout, MaxChunk: d.MaxChunk, ChunkSize: d.ChunkSize,
	}
	r.POST("/api/export-pdf/stream/init", stream.Init)
	r.POST("/api/export-pdf/stream/chunk", stream.Chunk)
	r.POST("/api/export-pdf/stream/finalize", stream.Finalize)
	return r
}

// Scalar is fetched from the CDN when the page is viewed.
const serviceName = "api-report-nexus"

const docsHTML = `<!doctype html>
<html>
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>report-nexus API</title></head>
<body>
<div id="app"></div>
<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference@1.71.0"></script>
<script>Scalar.createApiReference('#app', { url: '/api/openapi.yaml' })</script>
</body>
</html>`
