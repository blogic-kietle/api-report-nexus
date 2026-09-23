package httpapi

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"api-report-nexus/internal/infrastructure/gotenberg"

	"github.com/gin-gonic/gin"
)

const requestIDKey = "request_id"

func requestID(c *gin.Context) {
	id := c.GetHeader("X-Request-ID")
	if id == "" {
		id = rand.Text()
	}
	c.Set(requestIDKey, id)
	c.Header("X-Request-ID", id)
	c.Request = c.Request.WithContext(gotenberg.WithTrace(c.Request.Context(), id))
	c.Next()
}

func requestLog(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/healthz" {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()
		ms := time.Since(start).Milliseconds()
		log.Info(fmt.Sprintf("%s %s %d %dms", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), ms),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", ms,
			"req_bytes", c.Request.ContentLength,
			"bytes", c.Writer.Size(),
			"ip", c.ClientIP(),
			requestIDKey, c.GetString(requestIDKey),
		)
	}
}

func cors(c *gin.Context) {
	h := c.Writer.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	if c.Request.Method == http.MethodOptions {
		h.Set("Access-Control-Allow-Methods", "GET,HEAD,PUT,PATCH,POST,DELETE")
		h.Set("Access-Control-Allow-Headers", c.GetHeader("Access-Control-Request-Headers"))
		c.AbortWithStatus(http.StatusNoContent)
		return
	}
	c.Next()
}
