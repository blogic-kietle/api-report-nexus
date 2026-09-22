package httpapi

import (
	"crypto/rand"
	"log/slog"
	"net/http"
	"time"

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
		log.Info("http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
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
