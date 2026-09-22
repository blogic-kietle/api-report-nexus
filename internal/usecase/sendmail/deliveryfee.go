package sendmail

import (
	"context"
	"encoding/base64"
	htmltemplate "html/template"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"time"

	"api-report-nexus/assets"
	"api-report-nexus/internal/domain/email"
	"api-report-nexus/internal/domain/pdf"
	"api-report-nexus/internal/pkg/datetime"
	"api-report-nexus/internal/pkg/money"
	"api-report-nexus/internal/usecase/exportpdf"
)

const deliveryFeeTitle = "Online Orders Delivery Fee Statement"

// DeliveryFeeData is the statement payload: address block, headline figures, one row per day with its tickets.
type DeliveryFeeData struct {
	Store   DeliveryFeeStore
	Summary DeliveryFeeSummary
	Data    []DeliveryFeeRow
}

// DeliveryFeeStore is the address block; the JSON names are those of `meta`.
type DeliveryFeeStore struct {
	ID      string `json:"storeID"`
	Name    string `json:"storeName"`
	MID     string `json:"storeMID"`
	State   string `json:"storeState"`
	City    string `json:"storeCity"`
	Address string `json:"storeAddress"`
	Zip     string `json:"storeZip"`
}

type DeliveryFeeSummary struct {
	TotalDeliveryTicketCount float64 `json:"totalDeliveryTicketCount"`
	TotalDeliveryFees        float64 `json:"totalDeliveryFees"`
	TotalPayment             float64 `json:"totalPayment"`
}

// DeliveryFeeRow is one day of the statement, or one ticket within a day.
type DeliveryFeeRow struct {
	Date        string  `json:"date"`
	TicketCount float64 `json:"ticketCount"`
	// string or number from the POS
	TicketNumber   any              `json:"ticketNumber"`
	SubTotal       float64          `json:"subTotal"`
	Discount       float64          `json:"discount"`
	ServiceAndFees float64          `json:"serviceAndFees"`
	DeliveryFee    float64          `json:"deliveryFee"`
	Tax            float64          `json:"tax"`
	CustomFee      float64          `json:"customFee"`
	TotalGratuity  float64          `json:"totalGratuity"`
	TotalAmount    float64          `json:"totalAmount"`
	TotalPayment   float64          `json:"totalPayment"`
	Details        []DeliveryFeeRow `json:"details"`
	// the generated first row
	IsSummary bool `json:"-"`
}

// text/template with mustacheEscape, as for the day part email, so the markup matches Node byte for byte.
var deliveryFeeTemplate = template.Must(template.New("delivery-fee-report.html").
	Funcs(template.FuncMap{
		"currency": money.USD,
		"number":   func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) },
	}).
	ParseFS(assets.FS, "templates/delivery-fee-report.html"))

type deliveryFeeView struct {
	Title, Logo, Now, ReportPeriod, DateRange, SupportPhoneNNumber string
	Tailwind                                                       string // inlined, never escaped
	Store                                                          DeliveryFeeStore
	Summary                                                        DeliveryFeeSummary
	Data, Details                                                  []DeliveryFeeRow
}

// Like bodyTemplate this escapes the store name, unlike Node.
var deliveryFeeBody = htmltemplate.Must(htmltemplate.New("body").Parse("\n      " + noteHead + `            <p>Attached to this email, you will find the Online Orders Delivery Fee Statement, covering the period from {{.Period}}.</p>
            <p>This statement provides a detailed breakdown of the <strong>delivery fees charged by DoorDash and collected by BLogic Systems</strong> for online orders during the specified period. These fees were initially applied by DoorDash and are being <strong>reconciled and collected by BLogic Systems</strong> as outlined in the attached statement.</p>
            <p>Please review the attached file for full details.</p>
            <p>Thank you,</p>
            <p><strong>BLogic Systems</strong></p>
          </body>
        </html>`))

func DeliveryFeePDF(ctx context.Context, r exportpdf.Renderer, m Meta, d DeliveryFeeData, now time.Time) ([]byte, error) {
	html, err := deliveryFeeHTML(m, d, now)
	if err != nil {
		return nil, err
	}
	// Node also asked for 1240x1754px, which puppeteer ignores once a format is named.
	return r.Render(ctx, html, pdf.Request{Format: "A4"}.Normalize())
}

// DeliveryFee leaves the bus announcement to the caller: a failure there must not undo a sent mail.
func DeliveryFee(ctx context.Context, s email.Sender, r exportpdf.Renderer, req Request, d DeliveryFeeData, now time.Time) error {
	data, err := DeliveryFeePDF(ctx, r, req.Meta, d, now)
	if err != nil {
		return err
	}
	from, to := req.Meta.FromDate, req.Meta.ToDate
	var body strings.Builder
	if err := deliveryFeeBody.Execute(&body, struct{ Store, Period string }{
		Store: storeOrCustomer(req.Meta.StoreName), Period: plainPeriod(from, to),
	}); err != nil {
		return err
	}
	return s.Send(ctx, email.Message{
		ToEmails:    req.Emails,
		Subject:     req.Meta.StoreName + " - " + deliveryFeeTitle + subjectPeriod(from, to),
		Body:        body.String(),
		FileName:    deliveryFeeFileName(req.Meta.StoreID, from, to),
		FileContent: attachPDF(data),
	})
}

func deliveryFeeFileName(storeID, from, to string) string {
	const layout = "Jan-02-2006-03-04-PM"
	f, _ := datetime.ParseISO(from)
	t, _ := datetime.ParseISO(to)
	return "Delivery-Fee-Statement-" + storeID + "-From-" + f.Format(layout) + " to " + t.Format(layout) + ".pdf"
}

// Dates keep the payload's offset and the statement date is in the store's zone, like the subject
func deliveryFeeHTML(m Meta, d DeliveryFeeData, now time.Time) (string, error) {
	local := func(s, layout string) string {
		t, ok := datetime.ParseISO(s)
		if !ok {
			return "Invalid DateTime"
		}
		return mustacheEscape(t.Format(layout))
	}
	if from, ok := datetime.ParseISO(m.FromDate); ok {
		now = now.In(from.Location())
	}

	days := slices.Clone(d.Data)
	slices.SortStableFunc(days, func(a, b DeliveryFeeRow) int {
		ta, okA := datetime.ParseISO(a.Date)
		tb, okB := datetime.ParseISO(b.Date)
		if !okA || !okB {
			return 0
		}
		return ta.Compare(tb)
	})
	total := DeliveryFeeRow{Date: "Summary", IsSummary: true}
	var details []DeliveryFeeRow
	for i, day := range days {
		total.TicketCount += day.TicketCount
		total.SubTotal += day.SubTotal
		total.Discount += day.Discount
		total.ServiceAndFees += day.ServiceAndFees
		total.DeliveryFee += day.DeliveryFee
		total.Tax += day.Tax
		total.CustomFee += day.CustomFee
		total.TotalGratuity += day.TotalGratuity
		total.TotalAmount += day.TotalAmount
		total.TotalPayment += day.TotalPayment
		for _, t := range day.Details {
			t.Date = local(t.Date, "01/02/2006 03:04 PM")
			t.TicketNumber = mustacheEscape(ticketNumber(t.TicketNumber))
			details = append(details, t)
		}
		// day rows print the raw value
		days[i].Date = mustacheEscape(day.Date)
	}

	v := deliveryFeeView{
		Title:               mustacheEscape(deliveryFeeTitle),
		Logo:                mustacheEscape("data:image/png;base64," + base64.StdEncoding.EncodeToString(assets.Logo)),
		Now:                 mustacheEscape(now.Format("Jan 2, 2006")),
		ReportPeriod:        local(m.FromDate, "01/02/2006") + " - " + local(m.ToDate, "01/02/2006"),
		DateRange:           local(m.FromDate, "01/02") + " - " + local(m.ToDate, "01/02"),
		SupportPhoneNNumber: "(800) 464-9777",
		Tailwind:            assets.Tailwind,
		Store: DeliveryFeeStore{
			ID: mustacheEscape(d.Store.ID), Name: mustacheEscape(d.Store.Name), MID: mustacheEscape(d.Store.MID),
			State: mustacheEscape(d.Store.State), City: mustacheEscape(d.Store.City),
			Address: mustacheEscape(d.Store.Address), Zip: mustacheEscape(d.Store.Zip),
		},
		Summary: d.Summary,
		Data:    append([]DeliveryFeeRow{total}, days...),
		Details: details,
	}
	var out strings.Builder
	err := deliveryFeeTemplate.Execute(&out, v)
	return out.String(), err
}

// ticketNumber is `detail.ticketNumber || '-'`.
func ticketNumber(v any) string {
	switch n := v.(type) {
	case string:
		if n != "" {
			return n
		}
	case float64:
		if n != 0 {
			return strconv.FormatFloat(n, 'f', -1, 64)
		}
	}
	return "-"
}
