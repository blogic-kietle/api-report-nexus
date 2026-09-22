package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"api-report-nexus/internal/delivery/http/response"
	"api-report-nexus/internal/domain/daypart"
	"api-report-nexus/internal/domain/email"
	"api-report-nexus/internal/domain/report"
	"api-report-nexus/internal/domain/salessummary"

	"api-report-nexus/internal/usecase/exportpdf"
	"api-report-nexus/internal/usecase/sendmail"

	"github.com/gin-gonic/gin"
)

type SendMail struct {
	Sender     email.Sender
	Now        func() time.Time
	Log        *slog.Logger
	ClientURL  string
	PDF        exportpdf.Renderer
	PDFTimeout time.Duration
	Notify     Notifier
}

// Notifier announces a sent delivery fee statement to the rest of the platform.
type Notifier interface {
	DeliveryReportUpdated(ctx context.Context, storeID string) error
}

func (h SendMail) DeliveryFee(c *gin.Context) {
	var req struct {
		Meta    *mailMeta                   `json:"meta" binding:"required"`
		Emails  Emails                      `json:"emails" binding:"required,dive,email"`
		Summary sendmail.DeliveryFeeSummary `json:"summary"`
		Data    []sendmail.DeliveryFeeRow   `json:"data" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.PDFTimeout)
	defer cancel()

	m := req.Meta
	err := sendmail.DeliveryFee(ctx, h.Sender, h.PDF, sendmail.Request{Meta: m.toUsecase(), Emails: req.Emails},
		sendmail.DeliveryFeeData{
			Store: sendmail.DeliveryFeeStore{
				ID: m.StoreID, Name: m.StoreName, MID: m.StoreMID, State: m.StoreState,
				City: m.StoreCity, Address: m.StoreAddress, Zip: m.StoreZip,
			},
			Summary: req.Summary,
			Data:    req.Data,
		}, h.Now())
	if err == nil && h.Notify != nil && m.StoreID != "" {
		if nerr := h.Notify.DeliveryReportUpdated(ctx, m.StoreID); nerr != nil {
			h.Log.Error("delivery report update not published", "err", nerr, "store", m.StoreID)
		}
	}
	h.finish(c, err)
}

func (h SendMail) SalesSummary(c *gin.Context) {
	var req struct {
		Meta   *mailMeta           `json:"meta" binding:"required"`
		Emails Emails              `json:"emails" binding:"required,dive,email"`
		Data   *salessummary.Input `json:"data" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	h.finish(c, sendmail.SalesSummary(c.Request.Context(), h.Sender,
		sendmail.Request{Meta: req.Meta.toUsecase(), Emails: req.Emails}, *req.Data, h.ClientURL, h.Now()))
}

// Emails drops blank entries before validation, as the Node sanitizer did.
type Emails []string

func (e *Emails) UnmarshalJSON(b []byte) error {
	var raw []string
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	for _, addr := range raw {
		if addr != "" {
			*e = append(*e, addr)
		}
	}
	return nil
}

type mailMeta struct {
	StoreID        string          `json:"storeID"`
	StoreName      string          `json:"storeName"`
	POSVersion     string          `json:"posVersion"`
	FromDate       string          `json:"fromDate" binding:"required"`
	ToDate         string          `json:"toDate" binding:"required"`
	Configurations json.RawMessage `json:"configurations"`
	StoreMID       string          `json:"storeMID"`
	StoreState     string          `json:"storeState"`
	StoreCity      string          `json:"storeCity"`
	StoreAddress   string          `json:"storeAddress"`
	StoreZip       string          `json:"storeZip"`
}

func (m mailMeta) toUsecase() sendmail.Meta {
	return sendmail.Meta{
		StoreID: m.StoreID, StoreName: m.StoreName, POSVersion: m.POSVersion,
		FromDate: m.FromDate, ToDate: m.ToDate,
	}
}

func (h SendMail) LowInventory(c *gin.Context) {
	var req struct {
		Meta   *mailMeta                   `json:"meta" binding:"required"`
		Emails Emails                      `json:"emails" binding:"required,dive,email"`
		Data   []sendmail.LowInventoryItem `json:"data" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	h.finish(c, sendmail.LowInventory(c.Request.Context(), h.Sender,
		sendmail.Request{Meta: req.Meta.toUsecase(), Emails: req.Emails}, req.Data, h.Now()))
}

func (h SendMail) ItemSalesByEmployee(c *gin.Context) {
	var req struct {
		Meta   *mailMeta            `json:"meta" binding:"required"`
		Emails Emails               `json:"emails" binding:"required,dive,email"`
		Data   []sendmail.ItemSales `json:"data" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	h.finish(c, sendmail.ItemSalesByEmployee(c.Request.Context(), h.Sender,
		sendmail.Request{Meta: req.Meta.toUsecase(), Emails: req.Emails}, req.Data, h.Now()))
}

func (h SendMail) SalesByModifier(c *gin.Context) {
	var req struct {
		Meta   *mailMeta              `json:"meta" binding:"required"`
		Emails Emails                 `json:"emails" binding:"required,dive,email"`
		Data   *sendmail.ModifierData `json:"data" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	h.finish(c, sendmail.SalesByModifier(c.Request.Context(), h.Sender,
		sendmail.Request{Meta: req.Meta.toUsecase(), Emails: req.Emails}, *req.Data, h.Now()))
}

func (h SendMail) SalesDayPart(c *gin.Context) {
	var req struct {
		Meta   *mailMeta       `json:"meta" binding:"required"`
		Emails Emails          `json:"emails" binding:"required,dive,email"`
		Data   []daypart.Shift `json:"data" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	var configs []report.Config
	if !configurations(c, req.Meta.Configurations, &configs) {
		return
	}
	h.finish(c, sendmail.SalesDayPart(c.Request.Context(), h.Sender,
		sendmail.Request{Meta: req.Meta.toUsecase(), Emails: req.Emails}, req.Data, configs, h.Now()))
}

func (h SendMail) SalesByCategory(c *gin.Context) {
	var req struct {
		Meta   *mailMeta              `json:"meta" binding:"required"`
		Emails Emails                 `json:"emails" binding:"required,dive,email"`
		Data   *sendmail.CategoryData `json:"data" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	var configs string
	if !configurations(c, req.Meta.Configurations, &configs) {
		return
	}
	h.finish(c, sendmail.SalesByCategory(c.Request.Context(), h.Sender,
		sendmail.Request{Meta: req.Meta.toUsecase(), Emails: req.Emails}, configs, *req.Data, h.Now()))
}

func (h SendMail) finish(c *gin.Context, err error) {
	if err != nil {
		h.Log.Error("send email failed", "err", err, "path", c.Request.URL.Path)
		response.Fail(c, http.StatusInternalServerError, "Error sending email")
		return
	}
	response.OK(c, true, "Email sent")
}

// false means the 400 was already written.
func configurations(c *gin.Context, raw json.RawMessage, v any) bool {
	if len(raw) == 0 {
		return true
	}
	if err := json.Unmarshal(raw, v); err != nil {
		response.BindError(c, err)
		return false
	}
	return true
}
