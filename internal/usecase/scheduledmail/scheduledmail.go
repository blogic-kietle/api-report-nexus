// Package scheduledmail turns the scheduler's SendReportMailRequest into one mail with every report attached.
package scheduledmail

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"api-report-nexus/internal/domain/daypart"
	"api-report-nexus/internal/domain/email"
	"api-report-nexus/internal/domain/report"
	"api-report-nexus/internal/domain/salessummary"
	mailv1 "api-report-nexus/internal/gen/blogic/report/mail/v1"
	"api-report-nexus/internal/usecase/exportpdf"
	"api-report-nexus/internal/usecase/sendmail"
)

// clock is the scheduler's date format: local wall-clock, no zone.
const clock = "2006-01-02T15:04:05"

var errUnsupported = errors.New("unsupported report kind")

// Decode parses an inbound body; an error means a poison message.
func Decode(body []byte) (*mailv1.SendReportMailRequest, error) {
	var req mailv1.SendReportMailRequest
	if err := proto.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

type Service struct {
	Mail      email.Sender
	PDF       exportpdf.Renderer
	ClientURL string
	Now       func() time.Time
}

// Accepted is the first reply: the message is ours now.
func (s Service) Accepted(id string) *mailv1.SendReportMailResult {
	return s.result(id, mailv1.DeliveryStatus_DELIVERY_STATUS_ACCEPTED)
}

// Run builds the files, sends one mail and returns the terminal reply.
func (s Service) Run(ctx context.Context, req *mailv1.SendReportMailRequest) *mailv1.SendReportMailResult {
	emails := slices.DeleteFunc(slices.Clone(req.Emails), func(e string) bool { return strings.TrimSpace(e) == "" })
	if len(emails) == 0 {
		return s.fail(req.MessageId, mailv1.DeliveryStatus_DELIVERY_STATUS_REJECTED, "INVALID_RECIPIENTS", "no recipient address")
	}
	var files []email.Attachment
	for _, att := range req.Reports {
		file, err := s.build(ctx, req, att)
		switch {
		case errors.Is(err, errUnsupported):
			return s.fail(req.MessageId, mailv1.DeliveryStatus_DELIVERY_STATUS_REJECTED, "UNSUPPORTED_REPORT", err.Error())
		case err != nil:
			return s.fail(req.MessageId, mailv1.DeliveryStatus_DELIVERY_STATUS_FAILED, "RENDER_FAILED", err.Error())
		case file != nil:
			files = append(files, *file)
		}
	}
	if len(files) == 0 {
		return s.fail(req.MessageId, mailv1.DeliveryStatus_DELIVERY_STATUS_REJECTED, "NO_DOCUMENTS", "no report matches the requested document formats")
	}
	if err := s.Mail.Send(ctx, email.Message{ToEmails: emails, Subject: req.Subject, Body: req.Body, Attachments: files}); err != nil {
		return s.fail(req.MessageId, mailv1.DeliveryStatus_DELIVERY_STATUS_FAILED, "MAIL_FAILED", err.Error())
	}
	r := s.result(req.MessageId, mailv1.DeliveryStatus_DELIVERY_STATUS_SENT)
	r.DeliveredTo = emails
	return r
}

// nil means the report's format was not requested.
func (s Service) build(ctx context.Context, req *mailv1.SendReportMailRequest, att *mailv1.ReportAttachment) (*email.Attachment, error) {
	m := req.GetMeta()
	meta := sendmail.Meta{
		StoreID: m.GetStoreId(), StoreName: m.GetStoreName(), POSVersion: m.GetPosVersion(),
		FromDate: cmp.Or(att.GetFromDate(), m.GetFromDate()), ToDate: cmp.Or(att.GetToDate(), m.GetToDate()),
	}
	kind, now := att.GetReportKind(), s.Now()
	format, ext := mailv1.DocumentFormat_DOCUMENT_FORMAT_XLSX, ".xlsx"
	if kind == mailv1.ReportKind_REPORT_KIND_DELIVERY_FEE {
		format, ext = mailv1.DocumentFormat_DOCUMENT_FORMAT_PDF, ".pdf"
	}
	if len(req.DocumentFormats) > 0 && !slices.Contains(req.DocumentFormats, format) {
		return nil, nil
	}
	title := func(t string) report.Meta {
		return report.Meta{Title: t, StoreID: meta.StoreID, StoreName: meta.StoreName, From: meta.FromDate, To: meta.ToDate}
	}

	var data []byte
	var err error
	switch kind {
	case mailv1.ReportKind_REPORT_KIND_SALES_SUMMARY:
		var in salessummary.Input
		if err = json.Unmarshal(att.Data, &in); err == nil {
			data, err = sendmail.SalesSummaryWorkbook(in, title("Sales Summary Report"),
				salessummary.Options{Version: meta.POSVersion, StoreID: meta.StoreID, ClientURL: s.ClientURL}, now)
		}
	case mailv1.ReportKind_REPORT_KIND_SALES_BY_DAY_PART:
		var shifts []daypart.Shift
		var configs []report.Config
		// configurations is a JSON array here; any other shape just keeps the default labels
		_ = json.Unmarshal([]byte(m.GetConfigurations()), &configs)
		if err = json.Unmarshal(att.Data, &shifts); err == nil {
			data, err = sendmail.SalesDayPartWorkbook(daypart.Map(shifts, meta.FromDate, meta.ToDate, configs), title("Sales by Day Part Report"), now)
		}
	case mailv1.ReportKind_REPORT_KIND_LOW_INVENTORY:
		var items []sendmail.LowInventoryItem
		if err = json.Unmarshal(att.Data, &items); err == nil {
			data, err = sendmail.LowInventoryWorkbook(items, title("Low Inventory Report"), now)
		}
	case mailv1.ReportKind_REPORT_KIND_SALES_BY_MODIFIER:
		var d sendmail.ModifierData
		if err = json.Unmarshal(att.Data, &d); err == nil {
			data, err = sendmail.SalesByModifierWorkbook(d, title("Sales by Modifier Report"), now)
		}
	case mailv1.ReportKind_REPORT_KIND_SALES_BY_CATEGORY:
		var d sendmail.CategoryData
		cfg := m.GetConfigurations()
		if err = json.Unmarshal(att.Data, &d); err == nil {
			data, err = sendmail.SalesByCategoryWorkbook(d, title("Sales By Category Report"),
				sendmail.CategoryOption(cfg, "IncludeItemDetails"), sendmail.CategoryOption(cfg, "IncludeItemDetailsWithModifier"), now)
		}
	case mailv1.ReportKind_REPORT_KIND_ITEM_SALES_BY_EMPLOYEE:
		var items []sendmail.ItemSales
		if err = json.Unmarshal(att.Data, &items); err == nil {
			data, err = sendmail.ItemSalesByEmployeeWorkbook(items, title("Item Sales by Employee Report"), now)
		}
	case mailv1.ReportKind_REPORT_KIND_DELIVERY_FEE:
		d := sendmail.DeliveryFeeData{Store: sendmail.DeliveryFeeStore{
			ID: m.GetStoreId(), Name: m.GetStoreName(), MID: m.GetStoreMid(), State: m.GetStoreState(),
			City: m.GetStoreCity(), Address: m.GetStoreAddress(), Zip: m.GetStoreZip(),
		}}
		if err = json.Unmarshal(att.Data, &d.Data); err == nil && len(att.Summary) > 0 {
			err = json.Unmarshal(att.Summary, &d.Summary)
		}
		if err == nil {
			data, err = sendmail.DeliveryFeePDF(ctx, s.PDF, meta, d, now)
		}
	default:
		return nil, fmt.Errorf("%w: %s", errUnsupported, kind)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", kind, err)
	}
	name := cmp.Or(att.GetReportSlug(), strings.ToLower(strings.TrimPrefix(kind.String(), "REPORT_KIND_")))
	return &email.Attachment{Name: name + ext, Content: data}, nil
}

func (s Service) result(id string, status mailv1.DeliveryStatus) *mailv1.SendReportMailResult {
	return &mailv1.SendReportMailResult{MessageId: id, Status: status, OccurredAt: s.Now().Format(clock)}
}

func (s Service) fail(id string, status mailv1.DeliveryStatus, code, msg string) *mailv1.SendReportMailResult {
	r := s.result(id, status)
	r.ErrorCode, r.ErrorMessage = code, msg
	return r
}
