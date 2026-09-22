package response

import (
	"errors"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// MIME types as the Node service sent them (charset included).
const (
	MIMEExcel = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet;charset=UTF-8"
	MIMEPDF   = "application/pdf"
)

type Envelope struct {
	Data       any    `json:"data"`
	Message    string `json:"message"`
	IsError    bool   `json:"isError"`
	StatusCode int    `json:"statusCode"`
}

func OK(c *gin.Context, data any, msg string) {
	c.JSON(http.StatusOK, Envelope{Data: data, Message: msg, StatusCode: http.StatusOK})
}

// Fail aborts the handler chain with an error envelope.
func Fail(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, Envelope{Message: msg, IsError: true, StatusCode: status})
}

// The validator reports JSON field names, so error paths read "htmlContent", not the Go field.
func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(func(f reflect.StructField) string {
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "-" {
				return ""
			}
			return name
		})
	}
}

// fieldMessages are express-validator's wordings, keyed by JSON path, so clients see the same text.
var fieldMessages = map[string]string{
	"meta":          "Meta is required",
	"meta.fromDate": "Meta From Date is required",
	"meta.toDate":   "Meta To Date is required",
	"data":          "Data is required",
	"emails":        "Emails is required",
	"htmlContent":   "HTML content is required",
	"fileName":      "File name is required",
}

// tagMessages cover a rule wherever it fails.
var tagMessages = map[string]string{
	"email": "Invalid email format",
}

func BindError(c *gin.Context, err error) {
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		Fail(c, http.StatusRequestEntityTooLarge, "Request body too large")
		return
	}
	details := []gin.H{}
	if fields, ok := errors.AsType[validator.ValidationErrors](err); ok {
		for _, f := range fields {
			// request structs are anonymous, so Namespace is the bare JSON path
			path := f.Namespace()
			msg, ok := fieldMessages[path]
			if !ok {
				if msg, ok = tagMessages[f.Tag()]; !ok {
					msg = path + " is " + f.Tag()
				}
			}
			details = append(details, gin.H{"msg": msg, "path": path})
		}
	}
	c.AbortWithStatusJSON(http.StatusBadRequest, Envelope{
		Data: details, Message: "Validation failed", IsError: true, StatusCode: http.StatusBadRequest,
	})
}
